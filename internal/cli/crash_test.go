package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lintsinghua/oops/internal/restore"
)

// TestKillDuringUndoRecovers starts a real oops process, kills it after the
// safety snapshot and journal are on disk and the apply has started, and checks
// that the next command finishes the restore. Every user file is a complete
// version the whole time: a crash must not leave a truncated file.
func TestKillDuringUndoRecovers(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "oops")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/oops")
	build.Dir = moduleRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	v1 := strings.Repeat("v1-", 128*1024)
	v2 := strings.Repeat("v2-", 128*1024)
	const n = 32
	for i := 0; i < n; i++ {
		path := filepath.Join(root, "d", fmt.Sprintf("%02d.bin", i))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(v1), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code, out := runOops(t, home, root, hookBody(root, "s", "bulk"), "hook", "claude", "turn-start"); code != 0 {
		t.Fatalf("turn-start %d %s", code, out)
	}
	for i := 0; i < n; i++ {
		path := filepath.Join(root, "d", fmt.Sprintf("%02d.bin", i))
		if err := os.WriteFile(path, []byte(v2), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code, out := runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end"); code != 0 {
		t.Fatalf("turn-end %d %s", code, out)
	}

	cmd := exec.Command(bin, "--home", home, "--dir", root, "--color", "never", "undo", "-y")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	killed := false
	deadline := time.Now().Add(60 * time.Second)
	for !killed && time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("undo exited before it could be killed: %v\n%s%s", err, stdout.String(), stderr.String())
		default:
		}
		j, ok := pendingJournal(home)
		if ok && j.SafetySnap != "" && applyBegan(root, v1) {
			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			killed = true
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !killed {
		_ = cmd.Process.Kill()
		<-done
		t.Fatal("journal did not appear while undo was still running")
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("killed process did not exit")
	}

	whole := 0
	for i := 0; i < n; i++ {
		got := mustReadFile(t, filepath.Join(root, "d", fmt.Sprintf("%02d.bin", i)))
		if got == v1 || got == v2 {
			whole++
			continue
		}
		t.Fatalf("file %02d.bin is neither version (len %d)", i, len(got))
	}
	if whole != n {
		t.Fatalf("complete files %d", whole)
	}

	resume := exec.Command(bin, "--home", home, "--dir", root, "--color", "never", "list")
	resume.Dir = root
	out, err := resume.CombinedOutput()
	if err != nil {
		t.Fatalf("resume: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "interrupted") || !strings.Contains(string(out), "Finishing") {
		t.Fatalf("resume output: %s", out)
	}
	for i := 0; i < n; i++ {
		got := mustReadFile(t, filepath.Join(root, "d", fmt.Sprintf("%02d.bin", i)))
		if got != v1 {
			t.Fatalf("file %02d.bin was not restored (len %d)", i, len(got))
		}
	}
}

func pendingJournal(home string) (restore.Journal, bool) {
	entries, err := os.ReadDir(filepath.Join(home, "workspaces"))
	if err != nil {
		return restore.Journal{}, false
	}
	for _, e := range entries {
		j, err := restore.ReadJournal(filepath.Join(home, "workspaces", e.Name(), "journal", "current.json"))
		if err != nil {
			continue
		}
		if j.Status == "pending" && j.SafetySnap != "" {
			return j, true
		}
	}
	return restore.Journal{}, false
}

func applyBegan(root, restored string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".oops-tmp-") {
			found = true
			return filepath.SkipAll
		}
		if d.Name() == "current.json" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err == nil && string(body) == restored {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func moduleRoot(t *testing.T) string {
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
