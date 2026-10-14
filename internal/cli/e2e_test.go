package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runOops(t *testing.T, home, dir, in string, args ...string) (int, string) {
	t.Helper()
	full := append([]string{"--home", home, "--dir", dir, "--color", "never"}, args...)
	var stdout, stderr bytes.Buffer
	code := ExecuteIO(full, strings.NewReader(in), &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

func hookBody(dir, session, prompt string) string {
	b, _ := json.Marshal(map[string]string{"cwd": dir, "session_id": session, "prompt": prompt})
	return string(b)
}

func TestE2EUndoRestoresDeletedEnv(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "a.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("API_KEY=dev\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, hookBody(root, "e2e", "break"), "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatalf("start: %d %s", code, out)
	}
	if err := os.RemoveAll(filepath.Join(root, "src")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".env")); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, hookBody(root, "e2e", ""), "hook", "claude", "turn-end")
	if code != 0 {
		t.Fatalf("end: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "undo", "-y")
	if code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "a.txt")); err != nil {
		t.Fatalf("src/a.txt missing after undo: %v\n%s", err, out)
	}
	env, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil || string(env) != "API_KEY=dev\n" {
		t.Fatalf("env = %q err=%v", env, err)
	}
}

func TestE2EEditMoveModeSymlinkUnicode(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "héllo.txt"), []byte("uni"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, hookBody(root, "s", "edit"), "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatalf("start: %s", out)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "héllo.txt"), filepath.Join(root, "moved.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("moved.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "empty")); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	if code != 0 {
		t.Fatalf("end: %s", out)
	}
	code, out = runOops(t, home, root, "", "undo", "-n", "--json")
	if code != 0 || !strings.Contains(out, `"dry_run": true`) || !strings.Contains(out, `"schema_version": 1`) {
		t.Fatalf("dry json: %d %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, "moved.txt")); err != nil {
		t.Fatalf("dry run changed files: %v", err)
	}
	code, out = runOops(t, home, root, "", "undo", "-y")
	if code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	b, err := os.ReadFile(filepath.Join(root, "a.txt"))
	if err != nil || string(b) != "v1" {
		t.Fatalf("a.txt = %q %v", b, err)
	}
	info, err := os.Stat(filepath.Join(root, "a.txt"))
	if err != nil || info.Mode().Perm()&0o111 != 0 {
		t.Fatalf("mode = %v %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(root, "héllo.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "moved.txt")); !os.IsNotExist(err) {
		t.Fatalf("moved.txt still present: %v", err)
	}
	target, err := os.Readlink(filepath.Join(root, "link"))
	if err != nil || target != "a.txt" {
		t.Fatalf("link -> %q %v", target, err)
	}
	if _, err := os.Stat(filepath.Join(root, "empty")); err != nil {
		t.Fatalf("empty dir: %v", err)
	}
}

func TestE2ERedoAndSnapDryRun(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", "x"), "hook", "claude", "turn-start")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	code, out := runOops(t, home, root, "", "undo", "-y")
	if code != 0 {
		t.Fatalf("undo: %s", out)
	}
	code, out = runOops(t, home, root, "", "redo", "-n")
	if code != 0 || !strings.Contains(out, "Dry run") {
		t.Fatalf("redo dry: %d %s", code, out)
	}
	b, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(b) != "v1" {
		t.Fatalf("redo dry wrote %q", b)
	}
	code, out = runOops(t, home, root, "", "redo")
	if code != 0 {
		t.Fatalf("redo: %s", out)
	}
	b, _ = os.ReadFile(filepath.Join(root, "a.txt"))
	if string(b) != "v2" {
		t.Fatalf("redo got %q", b)
	}
	code, out = runOops(t, home, root, "", "list", "--json")
	if code != 0 {
		t.Fatal(out)
	}
	before := out
	code, out = runOops(t, home, root, "", "snap", "-n")
	if code != 0 || !strings.Contains(out, "Nothing changed") {
		t.Fatalf("snap dry: %d %s", code, out)
	}
	_, out = runOops(t, home, root, "", "list", "--json")
	if out != before {
		t.Fatalf("snap dry added a turn\n%q\n%q", before, out)
	}
}

func TestE2EShowJSONAndConflict(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", "x"), "hook", "claude", "turn-start")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	code, out := runOops(t, home, root, "", "show", "--json")
	if code != 0 || !strings.Contains(out, `"files"`) || !strings.Contains(out, "a.txt") || !strings.Contains(out, `"schema_version": 1`) {
		t.Fatalf("show json: %d %s", code, out)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, "", "undo")
	if code != 3 {
		t.Fatalf("conflict exit = %d %s", code, out)
	}
	b, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(b) != "user" {
		t.Fatalf("conflict overwrote %q", b)
	}
}

func TestE2ESubagentStopDoesNotSplit(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", "parent"), "hook", "claude", "turn-start")
	code, out := runOops(t, home, root, hookBody(root, "s", "child"), "hook", "claude", "subagent-stop")
	if code != 0 || strings.TrimSpace(out) != "" {
		t.Fatalf("subagent stdout: %d %q", code, out)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	code, out = runOops(t, home, root, "", "list", "--json")
	if code != 0 {
		t.Fatal(out)
	}
	if strings.Count(out, `"kind"`) != 1 {
		t.Fatalf("subagent split the turn: %s", out)
	}
}

func TestE2ELargeFileUnprotected(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte("[snapshot]\nmax_file_size = \"20B\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "big.bin"), bytes.Repeat([]byte("x"), 64), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, hookBody(root, "s", "x"), "--config", cfg, "hook", "claude", "turn-start")
	if code != 0 {
		t.Fatalf("start: %s", out)
	}
	if err := os.Remove(filepath.Join(root, "big.bin")); err != nil {
		t.Fatal(err)
	}
	code, out = runOops(t, home, root, hookBody(root, "s", ""), "--config", cfg, "hook", "claude", "turn-end")
	if code != 0 {
		t.Fatalf("end: %s", out)
	}
	code, out = runOops(t, home, root, "", "--config", cfg, "show")
	if code != 0 || !strings.Contains(out, "not protected") {
		t.Fatalf("show: %d %s", code, out)
	}
	_, out = runOops(t, home, root, "", "--config", cfg, "undo", "-y")
	if _, err := os.Stat(filepath.Join(root, "big.bin")); err == nil {
		t.Fatalf("large file was restored\n%s", out)
	}
}

func TestVersionCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v9.9.9"})
	}))
	defer srv.Close()
	code, out := runOops(t, t.TempDir(), t.TempDir(), "", "version", "--check", "--url", srv.URL)
	if code != 0 || !strings.Contains(out, "update available") || !strings.Contains(out, "v9.9.9") {
		t.Fatalf("check: %d %s", code, out)
	}
	code, out = runOops(t, t.TempDir(), t.TempDir(), "", "version", "--check", "--url", srv.URL+"/missing")
	if code != 0 || !strings.Contains(out, "no release published") {
		t.Fatalf("404: %d %s", code, out)
	}
}

func TestE2EWindowsLongPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows long path")
	}
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	long := strings.Repeat("d/", 40) + "f.txt"
	full := filepath.Join(root, filepath.FromSlash(long))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("long"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", "x"), "hook", "claude", "turn-start")
	if err := os.Remove(full); err != nil {
		t.Fatal(err)
	}
	_, _ = runOops(t, home, root, hookBody(root, "s", ""), "hook", "claude", "turn-end")
	code, out := runOops(t, home, root, "", "undo", "-y")
	if code != 0 {
		t.Fatal(out)
	}
	b, err := os.ReadFile(full)
	if err != nil || string(b) != "long" {
		t.Fatalf("long path restore: %v %q", err, b)
	}
}
