package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestEditRightAfterStopIsKept builds the real oops binary. The turn-end
// hook must not return until the post snapshot is stored, so an edit made
// the instant that process exits is the user's, and a file the user creates
// then is not deleted.
func TestEditRightAfterStopIsKept(t *testing.T) {
	rootDir := findModuleRoot(t)
	bin := filepath.Join(t.TempDir(), "oops")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/oops")
	build.Dir = rootDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runBin(t, bin, home, root, hookBody(root, "s", "edit"), "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatalf("start: %d %s", code, out)
	}
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = runBin(t, bin, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	if code != 0 {
		t.Fatalf("end: %d %s", code, out)
	}
	if err := os.WriteFile(path, []byte("v3"), 0o644); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(root, "user-new.txt")
	if err := os.WriteFile(created, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = runBin(t, bin, home, root, "", "undo", "-y")
	if code != 3 {
		t.Fatalf("undo: %d %s", code, out)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "v3" {
		t.Fatalf("a.txt %q err %v\n%s", got, err, out)
	}
	if got, err := os.ReadFile(created); err != nil || string(got) != "mine" {
		t.Fatalf("user-new.txt %q err %v\n%s", got, err, out)
	}
	if !strings.Contains(out, "kept") || !strings.Contains(out, "a.txt") {
		t.Fatalf("kept report:\n%s", out)
	}
}

func runBin(t *testing.T, bin, home, dir, in string, args ...string) (int, string) {
	t.Helper()
	full := append([]string{"--home", home, "--dir", dir, "--color", "never"}, args...)
	cmd := exec.Command(bin, full...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(in)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), string(out)
	}
	t.Fatal(err)
	return 1, string(out)
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
