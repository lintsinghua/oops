package engine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUndoRestoresBytes(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	mustWrite(t, filepath.Join(ws.Path, "src", "a.txt"), "one\n")
	mustWrite(t, filepath.Join(ws.Path, ".env"), "SECRET=1\n")
	if err := os.WriteFile(filepath.Join(ws.Path, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit files"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(ws.Path, "src", "a.txt"), "two\n")
	mustWrite(t, filepath.Join(ws.Path, "scripts", "build.mjs"), "export {}\n")
	if err := os.Remove(filepath.Join(ws.Path, ".env")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s"}, false); err != nil {
		t.Fatal(err)
	}
	e.Now = e.Now.Add(time.Minute)
	res, err := e.Undo(ctx, UndoRequest{Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != "ok" {
		t.Fatalf("result %s effects %#v", res.Result, res.Effects)
	}
	if got := mustRead(t, filepath.Join(ws.Path, "src", "a.txt")); got != "one\n" {
		t.Fatalf("a.txt %q", got)
	}
	if got := mustRead(t, filepath.Join(ws.Path, ".env")); got != "SECRET=1\n" {
		t.Fatalf(".env %q", got)
	}
	if _, err := os.Stat(filepath.Join(ws.Path, "scripts", "build.mjs")); !os.IsNotExist(err) {
		t.Fatalf("build.mjs still present: %v", err)
	}
}

func TestRedoRestoresUndo(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	mustWrite(t, filepath.Join(ws.Path, "a.txt"), "v1")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "codex", SessionID: "s", Prompt: "change"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(ws.Path, "a.txt"), "v2")
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s"}, false); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(ws.Path, "a.txt"))
	if _, err := e.Undo(ctx, UndoRequest{Workspace: ws}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Redo(ctx, ws, false); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(ws.Path, "a.txt"))
	if !bytes.Equal(before, after) {
		t.Fatalf("redo got %q want %q", after, before)
	}
}

func TestConflictKeepsUserEdit(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	mustWrite(t, filepath.Join(ws.Path, "a.txt"), "v1")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "change"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(ws.Path, "a.txt"), "v2")
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s"}, false); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(ws.Path, "a.txt"), "user")
	res, err := e.Undo(ctx, UndoRequest{Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != "partial" {
		t.Fatalf("result %s", res.Result)
	}
	if got := mustRead(t, filepath.Join(ws.Path, "a.txt")); got != "user" {
		t.Fatalf("kept %q", got)
	}
}

func TestHotScanReusesObjects(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	for i := 0; i < 400; i++ {
		mustWrite(t, filepath.Join(ws.Path, "f", "n"+itoa(i)+".txt"), "same\n")
	}
	if _, _, err := e.Capture(ctx, ws, SnapOptions{}); err != nil {
		t.Fatal(err)
	}
	before, err := e.Store.Size()
	if err != nil {
		t.Fatal(err)
	}
	// Files written in the last second are re-read. Wait that window out so
	// this measures a cache hit, which is what a hot scan is.
	time.Sleep(1100 * time.Millisecond)
	start := time.Now()
	if _, _, err := e.Capture(ctx, ws, SnapOptions{}); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	after, err := e.Store.Size()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("hot scan grew objects %d -> %d", before, after)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("hot scan took %s", elapsed)
	}
}

func openTest(t *testing.T) (*Engine, Workspace) {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()
	e, err := Open(home, t.TempDir(), "", time.Date(2026, 10, 14, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	ws, err := e.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	return e, ws
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFinishPendingKeepsTurnBytes(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	mustWrite(t, filepath.Join(ws.Path, "a.txt"), "v1")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(ws.Path, "a.txt"), "v2")
	if err := e.QueueFinish(ws, "s", false); err != nil {
		t.Fatal(err)
	}
	if err := e.FinishPending(ctx, ws); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, filepath.Join(ws.Path, "a.txt"))
	if got != "v2" {
		t.Fatalf("workspace = %q", got)
	}
	res, err := e.Undo(ctx, UndoRequest{Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != "ok" {
		t.Fatalf("undo result %s", res.Result)
	}
	if mustRead(t, filepath.Join(ws.Path, "a.txt")) != "v1" {
		t.Fatalf("undo got %q", mustRead(t, filepath.Join(ws.Path, "a.txt")))
	}
}

func TestMaintainKeepsRecentTurns(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	e.Config.Retention.KeepTurns = 2
	for i := 0; i < 4; i++ {
		mustWrite(t, filepath.Join(ws.Path, "a.txt"), string(rune('a'+i)))
		if _, err := e.ManualSnap(ctx, ws, "m"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.Maintain(ws); err != nil {
		t.Fatal(err)
	}
	n, err := e.DB.CountTurns(ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n > 2 {
		t.Fatalf("turns = %d, want <= 2", n)
	}
	if n == 0 {
		t.Fatal("maintain removed every turn")
	}
}

func TestMaintainEnforcesByteCap(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	e.Config.Retention.WorkspaceMax = "1B"
	for i := 0; i < 3; i++ {
		mustWrite(t, filepath.Join(ws.Path, "f"+string(rune('a'+i))+".txt"), "payload-"+string(rune('a'+i)))
		if _, err := e.ManualSnap(ctx, ws, "m"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.Maintain(ws); err != nil {
		t.Fatal(err)
	}
	n, err := e.DB.CountTurns(ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n > 1 {
		t.Fatalf("turns = %d, want the newest only once over the cap", n)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
