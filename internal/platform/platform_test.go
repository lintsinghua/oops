package platform

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestPathHelpers(t *testing.T) {
	if Key("") == "." {
		t.Fatal("empty key")
	}
	if Key("a/b") == "" || Same("a/b", "a/b") != true {
		t.Fatal("key")
	}
	if runtime.GOOS == "windows" {
		sys := os.Getenv("SystemRoot")
		vol := filepath.VolumeName(sys)
		if sys == "" || vol == "" || !ForbiddenRoot(sys) || !ForbiddenRoot(vol+`\`) {
			t.Fatalf("forbidden windows sys=%q vol=%q", sys, vol)
		}
	} else if !ForbiddenRoot("/") || !ForbiddenRoot("/etc") {
		t.Fatal("forbidden")
	}
	dir := t.TempDir()
	if ForbiddenRoot(dir) {
		t.Fatal("temp root")
	}
	if LongPath(dir) == "" {
		t.Fatal("long")
	}
	full, err := SafeRel(dir, "a/b.txt")
	if err != nil || !strings.HasSuffix(full, "b.txt") {
		t.Fatal(err, full)
	}
	for _, bad := range []string{"", "/", `../x`, `a/../../x`, "CON", "a/\x00"} {
		if _, err := SafeRel(dir, bad); err == nil && bad != "" {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, err := SafeRel(dir, ""); err == nil {
		t.Fatal("empty")
	}
	if err := SecureMkdir(filepath.Join(dir, "priv")); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out.txt")
	if err := WriteFileAtomic(dst, []byte("hi"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := CopyReaderAtomic(filepath.Join(dir, "copy.txt"), bytes.NewReader([]byte("c")), 0o600); err != nil {
		t.Fatal(err)
	}
	if !IsNotExist(os.ErrNotExist) || IsNotExist(errors.New("x")) {
		t.Fatal("exist")
	}
	if runtime.GOOS != "windows" && ExecBit(0o755) != 0o111 {
		t.Fatal("exec")
	}
	if DisplayPath("/tmp/a", "") != "/tmp/a" && runtime.GOOS != "windows" {
		t.Fatal("display")
	}
	home := t.TempDir()
	if DisplayPath(filepath.Join(home, "a"), home) != "~/a" {
		t.Fatalf("home display %s", DisplayPath(filepath.Join(home, "a"), home))
	}
	if DisplayPath(home, home) != "~" {
		t.Fatal("tilde")
	}
	if !HasUnicodeSpace("a b") || HasUnicodeSpace("ab") {
		t.Fatal("space")
	}
	if ProcessAlive(0) || !ProcessAlive(os.Getpid()) {
		t.Fatal("pid")
	}
	id, abs, err := WorkspaceID(dir)
	if err != nil || len(id) != 16 || abs == "" {
		t.Fatal(id, abs, err)
	}
	outside := filepath.Join(dir, "out")
	if err := os.Symlink(t.TempDir(), outside); err != nil {
		var errno syscall.Errno
		if runtime.GOOS == "windows" && errors.As(err, &errno) && errno == 1314 {
			t.Logf("symlink privilege: %v", err)
			return
		}
		t.Fatal(err)
	}
	if _, err := SafeRel(dir, "out/file"); err == nil {
		t.Fatal("escaping symlink")
	}
}
