package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitVersionColumnFits(t *testing.T) {
	home := t.TempDir()
	user := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	bin := t.TempDir()
	writeVersionAgent(t, bin, "claude", "2026.10.12-d47ca18")
	writeVersionAgent(t, bin, "codex", "1.2.3")
	code, out := runOops(t, home, root, "", "--user-home", user, "init", "--path", bin, "--agent", "claude", "--agent", "codex", "-y")
	if code != 0 {
		t.Fatalf("init: %d %s", code, out)
	}
	longCol := pathColumn(out, "2026.10.12-d47ca18")
	shortCol := pathColumn(out, "1.2.3")
	if longCol < 0 || shortCol < 0 || longCol != shortCol {
		t.Fatalf("path columns long=%d short=%d\n%s", longCol, shortCol, out)
	}
}

func TestInitUndoRemovesEmptyDirs(t *testing.T) {
	home := t.TempDir()
	user := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	bin := t.TempDir()
	writeFakeAgent(t, bin, "codex")
	writeFakeAgent(t, bin, "gemini")
	kept := filepath.Join(user, ".codex", "notes.txt")
	if err := os.MkdirAll(filepath.Dir(kept), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kept, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, "", "--user-home", user, "init", "--path", bin, "--agent", "codex", "--agent", "gemini", "-y")
	if code != 0 {
		t.Fatalf("init: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--undo", "--path", bin, "--agent", "codex", "--agent", "gemini", "-y")
	if code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(user, ".gemini")); !os.IsNotExist(err) {
		t.Fatalf(".gemini still present: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("user file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(user, ".codex")); err != nil {
		t.Fatalf(".codex with a user file was removed: %v", err)
	}
}

func TestInitUndoClearsEmptyClaudeHome(t *testing.T) {
	home := t.TempDir()
	user := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	bin := t.TempDir()
	writeFakeAgent(t, bin, "claude")
	code, out := runOops(t, home, root, "", "--user-home", user, "init", "--path", bin, "--agent", "claude", "-y")
	if code != 0 {
		t.Fatalf("init: %d %s", code, out)
	}
	settings := filepath.Join(user, ".claude", "settings.json")
	body, err := os.ReadFile(settings)
	if err != nil || !bytes.Contains(body, []byte(`"matcher": "oops"`)) || !bytes.Contains(body, []byte("UserPromptExpansion")) {
		t.Fatalf("settings: %s %v", body, err)
	}
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--undo", "--path", bin, "--agent", "claude", "-y")
	if code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	if _, err := os.Stat(settings); !os.IsNotExist(err) {
		left, _ := os.ReadFile(settings)
		t.Fatalf("settings remain: %v %s", err, left)
	}
	if _, err := os.Stat(filepath.Join(user, ".claude")); !os.IsNotExist(err) {
		t.Fatalf(".claude remains: %v", err)
	}
}

func TestInitUndoRestoresClaudeSettings(t *testing.T) {
	home := t.TempDir()
	user := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	bin := t.TempDir()
	writeFakeAgent(t, bin, "claude")
	settings := filepath.Join(user, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := []byte("{\n  \"theme\": \"dark\"\n}\n")
	if err := os.WriteFile(settings, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(user, ".claude", "notes.txt")
	if err := os.WriteFile(notes, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, "", "--user-home", user, "init", "--path", bin, "--agent", "claude", "-y")
	if code != 0 {
		t.Fatalf("init: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--undo", "--path", bin, "--agent", "claude", "-y")
	if code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	got, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(orig) {
		t.Fatalf("settings:\n%s", got)
	}
	if _, err := os.Stat(notes); err != nil {
		t.Fatal(err)
	}
}

func writeVersionAgent(t *testing.T, dir, name, version string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".cmd")
		body := "@echo " + version + "\r\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	path := filepath.Join(dir, name)
	body := "#!/bin/sh\necho " + version + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func pathColumn(out, version string) int {
	for _, line := range strings.Split(out, "\n") {
		i := strings.Index(line, version)
		if i < 0 {
			continue
		}
		rest := line[i+len(version):]
		j := 0
		for j < len(rest) && rest[j] == ' ' {
			j++
		}
		if j == len(rest) {
			continue
		}
		return i + len(version) + j
	}
	return -1
}
