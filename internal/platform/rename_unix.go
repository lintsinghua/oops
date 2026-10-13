//go:build !windows

package platform

import "os"

func renameReplace(src, dst string) error {
	return os.Rename(src, dst)
}
