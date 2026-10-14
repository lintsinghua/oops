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
		body = "@echo off\r\necho warning: experimental build 9.9.9 1>&2\r\necho 2.1.296 (Claude Code)\r\n"
	} else {
		body = "#!/bin/sh\necho 'warning: experimental build 9.9.9' >&2\necho '2.1.296 (Claude Code)'\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := versionOf(path); got != "2.1.296" {
		t.Fatalf("version %q", got)
	}
}

func TestFindVersionOutputs(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2.1.296 (Claude Code)\n", "2.1.296"},
		{"codex-cli 0.162.1\n", "0.162.1"},
		{"0.64.0\n", "0.64.0"},
		{"v0.64.0\n", "0.64.0"},
		{"version 0.64.0\n", "0.64.0"},
		{"banner: starting 1.0 helper\n", "1.0"},
		{"warning: experimental build\n", ""},
		{"Error: You must log in.\n", ""},
		{"Error: You must log in. See https://example.com/v1.2.3\n", "1.2.3"},
		{"banner: starting 1.0 helper\n0.64.0\n", "1.0"},
		{"warning: experimental build 9.9.9\n2.1.296 (Claude Code)\n", "9.9.9"},
	}
	for _, tc := range cases {
		if got := findVersion(tc.in); got != tc.want {
			t.Errorf("findVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGeminiVersionDoesNotRun(t *testing.T) {
	prefix := t.TempDir()
	binDir := filepath.Join(prefix, "bin")
	pkgDir := filepath.Join(prefix, "lib", "node_modules", "@google", "gemini-cli")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name":"@google/gemini-cli","version":"0.64.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(prefix, "launched")
	script := "#!/bin/sh\necho launched > '" + sentinel + "'\necho 'Error: You must log in.'\n"
	path := filepath.Join(binDir, "gemini")
	if runtime.GOOS == "windows" {
		path += ".cmd"
		script = "@echo off\r\necho launched > \"" + sentinel + "\"\r\necho Error: You must log in.\r\n"
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := versionOf(path); got != "0.64.0" {
		t.Fatalf("version %q", got)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("gemini was executed")
	}

	// A shim that only names the package via a quoted path, with no npm bin layout.
	shimDir := t.TempDir()
	index := filepath.Join(pkgDir, "index.js")
	if err := os.WriteFile(index, []byte("#!/usr/bin/env node\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(shimDir, "gemini")
	body := "#!/bin/sh\nexec node \"" + index + "\" \"$@\"\n"
	if runtime.GOOS == "windows" {
		shim += ".cmd"
		body = "@echo off\r\nnode \"" + index + "\" %*\r\n"
	}
	if err := os.WriteFile(shim, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := versionOf(shim); got != "0.64.0" {
		t.Fatalf("quoted path version %q", got)
	}
}

func TestNonSemverVersionIsUnknown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claude")
	body := "#!/bin/sh\necho 'Error: You must log in before continuing.'\n"
	if runtime.GOOS == "windows" {
		path += ".cmd"
		body = "@echo off\r\necho Error: You must log in before continuing.\r\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := versionOf(path); got != "" {
		t.Fatalf("error text parsed as %q", got)
	}
	inst := Detect(Claude(), t.TempDir(), t.TempDir(), false, []string{dir})
	if !inst.Found || inst.Version != "unknown" {
		t.Fatalf("detect %+v", inst)
	}
}

func TestRemoveOopsDropsEmptyMatcher(t *testing.T) {
	raw := []byte(`{"hooks":{"UserPromptExpansion":[{"matcher":"oops"}],"Stop":[{"hooks":[{"type":"command","command":"oops hook claude turn-end"}]}],"SessionStart":[{"hooks":[{"type":"command","command":"echo user-hook"}]}]}}`)
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	next, ok := RemoveOops(doc)
	if !ok {
		t.Fatal("expected a removal")
	}
	out, err := json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("oops")) || bytes.Contains(out, []byte("matcher")) || bytes.Contains(out, []byte("UserPromptExpansion")) {
		t.Fatalf("empty matcher kept: %s", out)
	}
	if !bytes.Contains(out, []byte("echo user-hook")) {
		t.Fatalf("user hook dropped: %s", out)
	}
	empty := map[string]any{"hooks": map[string]any{}}
	cleared, changed := RemoveOops(empty)
	if !changed {
		t.Fatal("empty hooks object stayed")
	}
	if left, _ := cleared.(map[string]any); len(left) != 0 {
		t.Fatalf("hooks object kept: %#v", cleared)
	}
}

func TestUninstallDeletesOopsCreatedClaudeFile(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "settings.json")
	if _, wrote, err := Claude().Install("oops", path, true); err != nil || !wrote {
		t.Fatalf("install wrote=%v %v", wrote, err)
	}
	body, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(body, []byte("UserPromptExpansion")) {
		t.Fatalf("settings: %s %v", body, err)
	}
	if _, changed, err := Claude().Uninstall(path); err != nil || !changed {
		t.Fatalf("uninstall changed=%v %v", changed, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("settings remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatalf(".claude remains: %v", err)
	}
}

func TestUninstallRestoresClaudeBytes(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "settings.json")
	orig := []byte("{\n  \"theme\": \"dark\",\n  \"hooks\": {\n    \"Stop\": [\n      {\n        \"matcher\": \"\",\n        \"hooks\": [{\"type\": \"command\", \"command\": \"echo stay\"}]\n      }\n    ]\n  }\n}\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(home, ".claude", "notes.txt")
	if err := os.WriteFile(kept, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Claude().Install("oops", path, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Claude().Uninstall(path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Fatalf("settings:\n%s", got)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); err != nil {
		t.Fatalf("dir with a user file was removed: %v", err)
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
