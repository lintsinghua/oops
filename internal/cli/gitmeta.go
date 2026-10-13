package cli

import (
	"os/exec"

	"github.com/lintsinghua/oops/internal/snapshot"
)

// annotateGit labels changes that git considers untracked or ignored.
// A missing git, or a directory that is not a work tree, leaves the flags unset.
func annotateGit(root string, changes []snapshot.Change) {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		return
	}
	for i := range changes {
		path := changes[i].Path
		if path == "" {
			continue
		}
		changes[i].Ignored = gitIgnored(root, path)
		changes[i].Untracked = !gitTracked(root, path)
	}
}

func gitIgnored(root, path string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", "--", path)
	cmd.Dir = root
	return cmd.Run() == nil
}

func gitTracked(root, path string) bool {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", "--", path)
	cmd.Dir = root
	return cmd.Run() == nil
}
