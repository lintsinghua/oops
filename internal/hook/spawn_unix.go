//go:build !windows

package hook

import "syscall"

func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
