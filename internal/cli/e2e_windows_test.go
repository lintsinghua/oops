//go:build windows

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestE2EWindowsCaseRename(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	orig := filepath.Join(root, "Name.txt")
	if err := os.WriteFile(orig, []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, hookBody(root, "s", "case"), "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatal(out)
	}
	next := filepath.Join(root, "name.txt")
	if err := os.Rename(orig, next); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte("B"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	if code != 0 {
		t.Fatal(out)
	}
	code, out = runOops(t, home, root, "", "undo", "-y")
	if code != 0 {
		t.Fatal(out)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	for _, ent := range entries {
		if strings.EqualFold(ent.Name(), "Name.txt") && ent.Name() != ".git" {
			name = ent.Name()
		}
	}
	if name != "Name.txt" {
		t.Fatalf("case restored as %q", name)
	}
	b, err := os.ReadFile(filepath.Join(root, "Name.txt"))
	if err != nil || string(b) != "A" {
		t.Fatalf("bytes %q err %v", b, err)
	}
}

func TestE2EWindowsLockedFile(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, hookBody(root, "s", "lock"), "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatal(out)
	}
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	if code != 0 {
		t.Fatal(out)
	}
	f, err := exclusiveOpen(path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	code, out = runOops(t, home, root, "", "undo", "-y")
	closeErr := f.Close()
	if time.Since(start) > 5*time.Second {
		t.Fatalf("undo hung for %s", time.Since(start))
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if code == 0 {
		t.Fatalf("locked undo succeeded:\n%s", out)
	}
	if !strings.Contains(out, "a.txt") {
		t.Fatalf("error did not name the file:\n%s", out)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "v2" {
		t.Fatalf("file changed despite the lock: %q err %v\n%s", b, err, out)
	}
}

func exclusiveOpen(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}
