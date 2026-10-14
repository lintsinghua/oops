package agents

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVersionSkipsBanner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-agent")
	var body string
	if runtime.GOOS == "windows" {
		path += ".cmd"
		body = "@echo off\r\necho warning: experimental build 1>&2\r\necho banner: starting 1.0 helper\r\necho 0.64.0\r\n"
	} else {
		body = "#!/bin/sh\necho 'warning: experimental build' >&2\necho 'banner: starting 1.0 helper'\necho '0.64.0'\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := versionOf(path); got != "0.64.0" {
		t.Fatalf("version %q", got)
	}
	if got := findVersion("warning: experimental build\n"); got != "" {
		t.Fatalf("banner parsed as %q", got)
	}
}

func TestRemoveOopsDropsNullEvents(t *testing.T) {
	doc := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "echo user-hook"}}},
			},
			"Stop": []any{
				map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "oops hook codex turn-end"}}},
			},
		},
	}
	next, ok := RemoveOops(doc)
	if !ok {
		t.Fatal("expected a removal")
	}
	raw, err := json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("null")) {
		t.Fatalf("null event: %s", raw)
	}
	if !bytes.Contains(raw, []byte("echo user-hook")) || bytes.Contains(raw, []byte("oops")) {
		t.Fatalf("hooks: %s", raw)
	}
}

func TestProjectDirCursorRoots(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	hooks := filepath.Join(t.TempDir(), ".cursor")
	raw, err := json.Marshal(map[string]any{
		"conversation_id": "c",
		"generation_id":   "g",
		"prompt":          "hi",
		"workspace_roots": []string{first, second},
	})
	if err != nil {
		t.Fatal(err)
	}
	ev := Parse("cursor", "beforeSubmitPrompt", raw)
	if ev.CWD != "" {
		t.Fatalf("payload cwd %q", ev.CWD)
	}
	t.Setenv("CURSOR_PROJECT_DIR", second)
	if got := ProjectDir(ev); got != second {
		t.Fatalf("multi-root %q", got)
	}
	t.Setenv("CURSOR_PROJECT_DIR", "")
	ev.CWD = hooks
	if got := ProjectDir(ev); got != first {
		t.Fatalf("hooks cwd won: %q", got)
	}
	ev = Parse("cursor", "turn-start", []byte(`{"conversation_id":"c","prompt":"x"}`))
	ev.CWD = hooks
	t.Setenv("CURSOR_PROJECT_DIR", second)
	if got := ProjectDir(ev); got != second {
		t.Fatalf("env %q", got)
	}
}
