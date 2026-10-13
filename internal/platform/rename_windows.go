//go:build windows

package platform

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func renameReplace(src, dst string) error {
	backoffs := []time.Duration{0, 50 * time.Millisecond, 150 * time.Millisecond, 400 * time.Millisecond}
	var err error
	for i, wait := range backoffs {
		if wait > 0 {
			time.Sleep(wait)
		}
		err = moveFile(src, dst)
		if err == nil {
			return nil
		}
		if i == len(backoffs)-1 {
			break
		}
	}
	return err
}

func moveFile(src, dst string) error {
	from, err := windows.UTF16PtrFromString(LongPath(src))
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(LongPath(dst))
	if err != nil {
		return err
	}
	err = windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	if err == nil {
		return nil
	}
	// Fall back to a remove-and-rename when the file is not locked.
	if rmErr := os.Remove(dst); rmErr == nil || os.IsNotExist(rmErr) {
		if rnErr := os.Rename(src, dst); rnErr == nil {
			return nil
		}
	}
	return err
}
