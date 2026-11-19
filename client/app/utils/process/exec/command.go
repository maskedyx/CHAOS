package exec

import "os/exec"

// Command builds a command from an absolute path. The path is used as-is
// because the executable may already be deleted from disk.
func Command(executable string) (*exec.Cmd, error) {
	return &exec.Cmd{Path: executable, Args: []string{executable}}, nil
}
