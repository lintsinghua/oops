package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/tui"
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

func TestPreviewListsEachPath(t *testing.T) {
	two := []snapshot.Change{
		{Path: "src/theme.tsx", Action: "added", UndoAction: "remove"},
		{Path: "src/other/theme.tsx", Action: "added", UndoAction: "remove"},
	}
	text, _ := previewTurn(nil, meta.Turn{Seq: 4, Status: "done", Prompt: "tidy imports"}, two)
	for _, path := range []string{"src/theme.tsx", "src/other/theme.tsx"} {
		if !strings.Contains(text, "\n      "+path+"\n") {
			t.Fatalf("path %s not on its own line:\n%s", path, text)
		}
	}
	if strings.Contains(text, "theme.tsx,") || strings.Contains(text, ", src") {
		t.Fatalf("paths merged:\n%s", text)
	}
	if !strings.Contains(text, "2 files") || strings.Contains(text, "+") {
		t.Fatalf("count:\n%s", text)
	}
	if !strings.Contains(text, "\n  ran: tidy imports\n") {
		t.Fatalf("ran:\n%s", text)
	}
	var many []snapshot.Change
	for i := 0; i < 10; i++ {
		many = append(many, snapshot.Change{Path: "f" + string(rune('0'+i)) + ".txt", Action: "added", UndoAction: "remove"})
	}
	capped, _ := previewTurn(nil, meta.Turn{Seq: 5, Status: "done"}, many)
	if !strings.Contains(capped, "10 files") || !strings.Contains(capped, "+2 more") {
		t.Fatalf("cap:\n%s", capped)
	}
	if strings.Contains(capped, "f8.txt") || strings.Contains(capped, "f9.txt") || !strings.Contains(capped, "f0.txt") || !strings.Contains(capped, "f7.txt") {
		t.Fatalf("capped paths:\n%s", capped)
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

func TestPreviewPartialUndoSaysRedo(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	mustWriteFile(t, filepath.Join(root, "base.txt"), "base")
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	agentTurn(t, home, root, "t1", "add nine", func() {
		for i := 1; i <= 9; i++ {
			mustWriteFile(t, filepath.Join(root, "pkg", "m"+strconv.Itoa(i)+".go"), "pkg")
		}
	})
	mustWriteFile(t, filepath.Join(root, "pkg", "m1.go"), "user edit")

	s := openSession(t, home, root)
	defer s.e.Close()
	ctx := context.Background()
	undoRow := timelineRow(t, s, 1)
	dryUndo, err := s.e.Undo(ctx, engine.UndoRequest{Workspace: s.ws, Seq: 1, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if undoRow.Plan != planSummary(dryUndo.Effects, false) {
		t.Fatalf("undo preview %q\ndry %q\n%s", undoRow.Plan, planSummary(dryUndo.Effects, false), undoRow.Preview)
	}
	undone, err := s.e.Undo(ctx, engine.UndoRequest{Workspace: s.ws, Seq: 1})
	if err != nil {
		t.Fatal(err)
	}
	if undoRow.Plan != planSummary(undone.Effects, false) {
		t.Fatalf("undo preview %q\napplied %q", undoRow.Plan, planSummary(undone.Effects, false))
	}
	if w, k := countPlanned(undone.Effects); w != 8 || k != 1 {
		t.Fatalf("undo applied write %d kept %d\n%s", w, k, undoRow.Preview)
	}

	redoRow := timelineRow(t, s, 1)
	if !strings.Contains(redoRow.Preview, "undone (partial)") || !strings.Contains(redoRow.Preview, "Redo #1 would re-apply:") {
		t.Fatalf("preview:\n%s", redoRow.Preview)
	}
	if strings.Contains(redoRow.Preview, "Undo #1 would") || strings.Contains(redoRow.Preview, "9 files") {
		t.Fatalf("preview counts the kept file:\n%s", redoRow.Preview)
	}
	dryRedo, err := s.e.Redo(ctx, s.ws, true)
	if err != nil {
		t.Fatal(err)
	}
	if redoRow.Plan != planSummary(dryRedo.Effects, true) {
		t.Fatalf("redo preview %q\ndry %q\n%s", redoRow.Plan, planSummary(dryRedo.Effects, true), redoRow.Preview)
	}
	redone, err := s.e.Redo(ctx, s.ws, false)
	if err != nil {
		t.Fatal(err)
	}
	if redoRow.Plan != planSummary(redone.Effects, true) {
		t.Fatalf("redo preview %q\napplied %q", redoRow.Plan, planSummary(redone.Effects, true))
	}
	if w, k := countPlanned(redone.Effects); w != 8 || k != 1 {
		t.Fatalf("redo applied write %d kept %d\nplan %q\n%s", w, k, redoRow.Plan, redoRow.Preview)
	}
	if redoRow.Plan != "add 8 files, keep 1 file (yours)" {
		t.Fatalf("plan %q", redoRow.Plan)
	}
	if strings.Contains(redoRow.Preview, "modify") || !strings.Contains(redoRow.Preview, "1 file (yours)") {
		t.Fatalf("preview:\n%s", redoRow.Preview)
	}
	card := tui.New([]tui.Row{redoRow}, "proj", "", false, true)
	card.Width = 120
	card.Height = 30
	var model tea.Model = card
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := model.(tui.Model).View()
	if !strings.Contains(view, "keep 1 file (yours)") || strings.Contains(view, "modify") {
		t.Fatalf("confirm card:\n%s", view)
	}
	if lines := strings.Split(view, "\n"); len(lines) == 0 || !strings.Contains(lines[0], "oops") || len(lines) > 30 {
		t.Fatalf("title left the 30-row card view (%d lines)\n%s", len(lines), view)
	}
}

func TestApplyTimelineRedoNamesKeptFile(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	mustWriteFile(t, filepath.Join(root, "base.txt"), "base")
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	agentTurn(t, home, root, "t1", "add nine", func() {
		for i := 1; i <= 9; i++ {
			mustWriteFile(t, filepath.Join(root, "pkg", "m"+strconv.Itoa(i)+".go"), "pkg")
		}
	})
	mustWriteFile(t, filepath.Join(root, "pkg", "m1.go"), "user edit")
	s := openSession(t, home, root)
	defer s.e.Close()
	if _, err := s.e.Undo(context.Background(), engine.UndoRequest{Workspace: s.ws, Seq: 1}); err != nil {
		t.Fatal(err)
	}
	_, notice, err := applyTimeline(s, tui.Action{Kind: "redo", Seq: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notice, "except") || !strings.Contains(notice, "pkg/m1.go") {
		t.Fatalf("notice %q", notice)
	}
}

func timelineRow(t *testing.T, s *session, seq int) tui.Row {
	t.Helper()
	rows, err := timelineRows(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Seq == seq {
			return row
		}
	}
	t.Fatalf("turn %d missing", seq)
	return tui.Row{}
}

func openSession(t *testing.T, home, root string) *session {
	t.Helper()
	s, err := begin(&options{
		home: home, dir: root, color: "never", quiet: true,
		stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func countPlanned(effects []engine.Effect) (write, kept int) {
	for _, ef := range effects {
		if ef.Dir || ef.Op == "skip" {
			continue
		}
		switch ef.Op {
		case "kept", "unprotected":
			kept++
		case "restore", "remove", "revert":
			write++
		}
	}
	return write, kept
}

func TestRedoNoticeNamesKeptFile(t *testing.T) {
	note := redoNotice(engine.UndoResult{
		Effects: []engine.Effect{
			{Path: "pkg/m1.go", Op: "kept"},
			{Path: "a.go", Op: "restore"},
		},
	})
	want := "Redid the last change, except 1 file you changed afterwards: pkg/m1.go"
	if note != want {
		t.Fatalf("notice %q", note)
	}
	clean := redoNotice(engine.UndoResult{Effects: []engine.Effect{{Path: "a.go", Op: "restore"}}})
	if clean != "Redid the last change" || strings.Contains(clean, "Undid") {
		t.Fatalf("clean %q", clean)
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
