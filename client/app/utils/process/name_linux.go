//go:build linux

package process

import (
	"syscall"
	"unsafe"
)

const processNameMaxLength = 15

func setProcessName(name string) {
	replaceArgvZero(name)

	if len(name) > processNameMaxLength {
		name = name[:processNameMaxLength]
	}

	buf := make([]byte, 16)
	copy(buf, name)

	_, _, _ = syscall.Syscall(syscall.SYS_PRCTL, syscall.PR_SET_NAME, uintptr(unsafe.Pointer(&buf[0])), 0)
}
