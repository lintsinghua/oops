package slash

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
)

func TestSlashCommands(t *testing.T) {
	if parseSeq("#12") != 12 || parseSeq("no") != 0 || parseSeq("") != 0 {
		t.Fatal("parseSeq")
	}
	text := formatUndo(engine.UndoResult{Turn: meta.Turn{Seq: 4}, Effects: []engine.Effect{{Op: "restore"}, {Op: "remove"}, {Op: "revert"}}})
	if text == "" || codeOf(engine.UndoResult{Result: "partial"}) != 3 || codeOf(engine.UndoResult{}) != 0 {
		t.Fatal(text)
	}
	home := t.TempDir()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, err := engine.Open(home, t.TempDir(), filepath.Join(home, "c.toml"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	ws, err := e.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	out, code := Run(e, ws, nil, time.Now())
	if code != 0 || out == "" {
		t.Fatalf("list %d %s", code, out)
	}
	out, code = Run(e, ws, []string{"undo"}, time.Now())
	if code == 0 {
		t.Fatalf("undo empty %d %s", code, out)
	}
	out, code = Run(e, ws, []string{"undo", "3"}, time.Now())
	if code == 0 {
		t.Fatalf("undo 3 %d %s", code, out)
	}
	out, code = Run(e, ws, []string{"redo"}, time.Now())
	if code == 0 || out == "" {
		t.Fatalf("redo %d %s", code, out)
	}
	out, code = Run(e, ws, []string{"diff"}, time.Now())
	if code == 0 {
		t.Fatalf("diff %d %s", code, out)
	}
}

func TestSlashUndoWaitsForPendingStop(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, err := engine.Open(home, t.TempDir(), filepath.Join(home, "c.toml"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	ws, err := e.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := e.StartTurn(context.Background(), engine.TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := Run(e, ws, []string{"undo"}, time.Now())
	if code != 4 || !strings.Contains(out, "still running") {
		t.Fatalf("live turn: %d %s", code, out)
	}
	if got, _ := os.ReadFile(path); string(got) != "v2" {
		t.Fatalf("live undo wrote %s", got)
	}
	if err := e.QueueFinish(ws, "s", false); err != nil {
		t.Fatal(err)
	}
	out, code = Run(e, ws, []string{"undo"}, time.Now())
	if strings.Contains(out, "still running") || code != 0 || !strings.Contains(out, "✓ oops") {
		t.Fatalf("pending stop: %d %s", code, out)
	}
	if got, _ := os.ReadFile(path); string(got) != "v1" {
		t.Fatalf("undo got %s", got)
	}
}
