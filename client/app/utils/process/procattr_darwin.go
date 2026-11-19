//go:build darwin

package process

import "syscall"

func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true, Setpgid: true}
}
