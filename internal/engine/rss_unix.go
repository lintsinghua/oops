//go:build !linux && !windows

package engine

import (
	"runtime"
	"syscall"
)

// residentBytes is the process high-water mark. Darwin reports ru_maxrss in bytes.
func residentBytes() uint64 {
	return maxRSS()
}

func maxRSS() uint64 {
	var r syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &r); err != nil {
		return 0
	}
	n := uint64(r.Maxrss)
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		return n
	}
	return n * 1024
}
