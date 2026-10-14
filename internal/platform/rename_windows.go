//go:build windows

package platform

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// IsBusy reports a sharing or lock conflict from a replace.
func IsBusy(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}

func renameReplace(src, dst string, sync bool) error {
	backoffs := []time.Duration{0, 50 * time.Millisecond, 150 * time.Millisecond, 400 * time.Millisecond}
	var err error
	for i, wait := range backoffs {
		if wait > 0 {
			time.Sleep(wait)
		}
		err = moveFile(src, dst, sync)
		if err == nil {
			return nil
		}
		if i == len(backoffs)-1 {
			break
		}
	}
	return err
}

func moveFile(src, dst string, sync bool) error {
	from, err := windows.UTF16PtrFromString(LongPath(src))
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(LongPath(dst))
	if err != nil {
		return err
	}
	// WRITE_THROUGH flushes each rename. A 1,000-file undo uses the unsynced
	// path and flushes the directory once at the end of the batch.
	flags := uint32(windows.MOVEFILE_REPLACE_EXISTING)
	if sync {
		flags |= windows.MOVEFILE_WRITE_THROUGH
	}
	err = windows.MoveFileEx(from, to, flags)
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
