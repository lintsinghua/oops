//go:build windows

package hook

import "syscall"

func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
}
