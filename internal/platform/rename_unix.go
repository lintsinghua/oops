//go:build !windows

package platform

import "os"

func renameReplace(src, dst string, _ bool) error {
	return os.Rename(src, dst)
}

// IsBusy reports a sharing or lock conflict. Unix renames do not surface one.
func IsBusy(error) bool { return false }
