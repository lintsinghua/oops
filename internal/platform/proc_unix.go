//go:build !windows

package platform

import "syscall"

// ProcessAlive reports whether pid is a running process.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
