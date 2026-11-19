package screenshot

import (
	"os"
	"os/exec"
)

// captureDarwin uses the built-in screencapture utility so macOS clients can be
// built with CGO disabled, which is required for cross compilation.
func captureDarwin() ([]byte, error) {
	file, err := os.CreateTemp("", "chaos-screenshot-*.png")
	if err != nil {
		return nil, err
	}
	filepath := file.Name()
	file.Close()
	defer os.Remove(filepath)

	if err := exec.Command("screencapture", "-x", "-t", "png", filepath).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath)
}
