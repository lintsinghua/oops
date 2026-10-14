package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/snapshot"
)

func TestFileCountsIgnoreDirectories(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	mustWriteFile(t, filepath.Join(root, "keep.txt"), "k")
	agentTurn(t, home, root, "s", "add files", func() {
		if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "onlydir"), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWriteFile(t, filepath.Join(root, "nested", "a.txt"), "a")
		mustWriteFile(t, filepath.Join(root, "b.txt"), "b")
	})

	code, out := runOops(t, home, root, "", "list", "--json")
	if code != 0 {
		t.Fatalf("list: %d %s", code, out)
	}
	var body struct {
		Turns []struct {
			Stats struct {
				Files int `json:"files"`
			} `json:"stats"`
		} `json:"turns"`
	}
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Turns) == 0 || body.Turns[0].Stats.Files != 2 {
		t.Fatalf("stats %+v", body.Turns)
	}
	code, out = runOops(t, home, root, "", "list")
	if code != 0 || !strings.Contains(out, "2 files") || strings.Contains(out, "3 files") {
		t.Fatalf("list text:\n%s", out)
	}
	code, out = runOops(t, home, root, "", "show")
	if code != 0 || strings.Contains(out, "onlydir") || !strings.Contains(out, "b.txt") || !strings.Contains(out, "nested/a.txt") {
		t.Fatalf("show:\n%s", out)
	}
	code, out = runOops(t, home, root, "", "undo", "-n")
	if code != 0 || !strings.Contains(out, "2 files") || strings.Contains(out, "onlydir") {
		t.Fatalf("preview:\n%s", out)
	}
	code, out = runOops(t, home, root, hookBody(root, "s", "/oops"), "hook", "claude", "turn-start")
	if code != 0 || !strings.Contains(out, "2 files") || strings.Contains(out, "onlydir") || !strings.Contains(out, "✓ oops") {
		t.Fatalf("slash:\n%s", out)
	}
	code, out = runOops(t, home, root, hookBody(root, "s", "/oops diff"), "hook", "claude", "slash")
	if code != 0 || strings.Contains(out, "onlydir") || !strings.Contains(out, "diff") {
		t.Fatalf("slash diff:\n%s", out)
	}

	e, err := engine.Open(home, t.TempDir(), "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ws, err := e.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	turns, err := e.DB.ListTurns(ws.ID, 5, false)
	if err != nil || len(turns) == 0 {
		t.Fatal(err)
	}
	changes, err := e.ChangesBetween(turns[0])
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.CountFileChanges(changes) != turns[0].Stats.Files {
		t.Fatalf("diff %d stats %d", snapshot.CountFileChanges(changes), turns[0].Stats.Files)
	}
	preview, diffs := previewTurn(e, turns[0], changes)
	if !strings.Contains(preview, "2 files") || strings.Contains(preview, "onlydir") {
		t.Fatalf("timeline preview:\n%s", preview)
	}
	for _, d := range diffs {
		if strings.Contains(d, "onlydir") {
			t.Fatalf("dir diff %s", d)
		}
	}
	rows, err := timelineRows(&session{e: e, ws: ws})
	if err != nil || len(rows) == 0 || rows[0].Files != 2 {
		t.Fatalf("rows %+v %v", rows, err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "gapdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, "gap.txt"), "g")
	agentTurn(t, home, root, "s2", "next", func() {})
	code, out = runOops(t, home, root, "", "list")
	if code != 0 || !strings.Contains(out, "1 file changed outside agents") || strings.Contains(out, "2 files changed outside") {
		t.Fatalf("gap:\n%s", out)
	}
}

func TestRedoHintSuggestsUndo(t *testing.T) {
	home, root := oneTurnWorkspace(t)
	code, out := runOops(t, home, root, "", "undo", "-y")
	if code != 0 || !strings.Contains(out, "oops redo") {
		t.Fatalf("undo hint: %d %s", code, out)
	}
	code, out = runOops(t, home, root, "", "redo")
	if code != 0 || !strings.Contains(out, "oops undo") || strings.Contains(out, "oops redo") {
		t.Fatalf("redo hint: %d %s", code, out)
	}
}

func TestPreviewUndoneSaysRedo(t *testing.T) {
	changes := []snapshot.Change{
		{Path: "a.txt", Action: "deleted", UndoAction: "restore"},
		{Path: "b.txt", Action: "deleted", UndoAction: "restore"},
		{Path: "c.txt", Action: "deleted", UndoAction: "restore"},
		{Path: "d.txt", Action: "deleted", UndoAction: "restore"},
		{Path: "e.txt", Action: "modified", UndoAction: "revert"},
	}
	text, _ := previewTurn(nil, meta.Turn{Seq: 2, Status: "undone", Prompt: "tidy"}, changes)
	if !strings.Contains(text, "already undone") || !strings.Contains(text, "Redo #2 would re-apply:") {
		t.Fatal(text)
	}
	if strings.Contains(text, "Undo #2 would") || strings.Contains(text, "restore") {
		t.Fatalf("undo wording in redo preview:\n%s", text)
	}
	if !strings.Contains(text, "remove") || !strings.Contains(text, "4 files") || !strings.Contains(text, "modify") {
		t.Fatalf("re-apply lines:\n%s", text)
	}
	text, _ = previewTurn(nil, meta.Turn{Seq: 3, Status: "done"}, nil)
	if !strings.Contains(text, "Undo #3 would") {
		t.Fatal(text)
	}
}

func TestSlashSuccessIsShort(t *testing.T) {
	home, root := oneTurnWorkspace(t)
	code, out := runOops(t, home, root, hookBody(root, "s", "/oops undo"), "hook", "claude", "turn-start")
	if code != 0 || !strings.Contains(out, "decision") || !strings.Contains(out, "✓ oops") || !strings.Contains(out, "undid") {
		t.Fatalf("slash undo: %d %s", code, out)
	}
	if strings.Contains(out, "restored") || strings.Contains(out, "operation blocked") {
		t.Fatalf("slash looked like an error:\n%s", out)
	}
	if strings.Count(out, "\n") > 2 {
		t.Fatalf("slash undo is long:\n%s", out)
	}
}

func TestUndoNonTTYLatestAndPipedNo(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	mustWriteFile(t, filepath.Join(root, "a.txt"), "v1")
	agentTurn(t, home, root, "t1", "first", func() {
		mustWriteFile(t, filepath.Join(root, "a.txt"), "v2")
	})
	agentTurn(t, home, root, "t2", "second", func() {
		mustWriteFile(t, filepath.Join(root, "a.txt"), "v3")
	})

	code, out := runOops(t, home, root, "n\n", "undo", "1")
	if code != 2 || !strings.Contains(out, "needs confirmation") || strings.Contains(out, "Undo?") {
		t.Fatalf("older non-tty: %d %s", code, out)
	}
	if got := mustReadFile(t, filepath.Join(root, "a.txt")); got != "v3" {
		t.Fatalf("older undo wrote %q", got)
	}

	code, out = runOops(t, home, root, "n\n", "undo")
	if code != 0 || strings.Contains(out, "Undo?") || !strings.Contains(out, "turn #2") {
		t.Fatalf("latest non-tty: %d %s", code, out)
	}
	if got := mustReadFile(t, filepath.Join(root, "a.txt")); got != "v2" {
		t.Fatalf("latest undo got %q", got)
	}

	testingTTY = func(any) bool { return true }
	t.Cleanup(func() { testingTTY = nil })
	code, out = runOops(t, home, root, "n\n", "undo", "1")
	if code != 0 || !strings.Contains(out, "Nothing changed.") || !strings.Contains(out, "Undo?") {
		t.Fatalf("piped n: %d %s", code, out)
	}
	if got := mustReadFile(t, filepath.Join(root, "a.txt")); got != "v2" {
		t.Fatalf("piped n applied undo: %q\n%s", got, out)
	}
}

func TestGeminiInitLeavesSettings(t *testing.T) {
	home := t.TempDir()
	user := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
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
	settings := filepath.Join(user, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("{\n  \"disableAutoUpdate\": true,\n\t\"theme\": \"dark\"\n}")
	if err := os.WriteFile(settings, original, 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(prefix, "launched")
	script := "#!/bin/sh\necho launched > '" + sentinel + "'\nprintf '%s' rewritten > '" + settings + "'\necho 'Error: You must log in.'\n"
	name := filepath.Join(binDir, "gemini")
	if runtime.GOOS == "windows" {
		name += ".cmd"
		script = "@echo off\r\necho launched> \"" + sentinel + "\"\r\necho rewritten> \"" + settings + "\"\r\necho Error: You must log in.\r\n"
	}
	if err := os.WriteFile(name, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, "", "--user-home", user, "init", "--path", binDir, "--agent", "gemini", "-y")
	if code != 0 {
		t.Fatalf("init: %d %s", code, out)
	}
	if strings.Contains(out, "log in") || strings.Contains(out, "Error") || strings.Contains(out, "unknown") || !strings.Contains(out, "0.64.0") {
		t.Fatalf("version column:\n%s", out)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("gemini was launched")
	}
	if bytes.Equal(mustBytes(t, settings), original) {
		t.Fatal("init did not install hooks")
	}
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--undo", "--path", binDir, "--agent", "gemini", "-y")
	if code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	if got := mustBytes(t, settings); !bytes.Equal(got, original) {
		t.Fatalf("settings:\n%s", got)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("gemini was launched during undo")
	}
}
