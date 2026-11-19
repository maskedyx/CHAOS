//go:build !linux && !darwin

package process

// EmptyName is the process name shown while Run Hidden is enabled.
const EmptyName = " "

// Hide is a no-op on platforms where the process cannot be renamed at runtime.
func Hide() (bool, error) {
	return false, nil
}

// SetName is a no-op on platforms where the process cannot be renamed at runtime.
func SetName(string) {}
