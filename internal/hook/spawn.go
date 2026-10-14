package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// spawnFinish starts a detached process that runs retention. The post
// snapshot is already stored by the caller. It returns false when no child
// is started (tests, or when the child cannot be started).
func spawnFinish(home string) bool {
	if home == "" || os.Getenv("OOPS_NO_MAINTAIN") == "1" {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	base := filepath.Base(exe)
	if strings.Contains(base, ".test") {
		return false
	}
	cmd := exec.Command(exe, "finish-pending", "--home", home)
	cmd.SysProcAttr = detachAttr()
	return cmd.Start() == nil
}
