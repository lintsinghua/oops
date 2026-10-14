package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUndoKeepsFilesNewerThanStop(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	agent := filepath.Join(ws.Path, "agent.txt")
	user := filepath.Join(ws.Path, "user.txt")
	nested := filepath.Join(ws.Path, "sub", "a.txt")
	note := filepath.Join(ws.Path, "sub", "note.txt")
	mustWrite(t, agent, "v1")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, agent, "v2")
	mustWrite(t, user, "created")
	mustWrite(t, nested, "agent")
	mustWrite(t, note, "user")
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s"}, false); err != nil {
		t.Fatal(err)
	}
	// Bytes the post snapshot already stored are undone even when their mtime
	// falls inside the snapshot window. A later edit with different bytes stays.
	mustWrite(t, user, "edited")
	mustWrite(t, note, "edited-note")
	res, err := e.Undo(ctx, UndoRequest{Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != "partial" {
		t.Fatalf("result %s", res.Result)
	}
	if got := mustRead(t, agent); got != "v1" {
		t.Fatalf("agent.txt %q", got)
	}
	if got := mustRead(t, user); got != "edited" {
		t.Fatalf("user.txt %q", got)
	}
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Fatalf("old nested file still present: %v", err)
	}
	if got := mustRead(t, note); got != "edited-note" {
		t.Fatalf("note.txt %q", got)
	}
	if _, err := os.Stat(filepath.Join(ws.Path, "sub")); err != nil {
		t.Fatalf("directory removed: %v", err)
	}
	kept := map[string]bool{}
	for _, ef := range res.Effects {
		if ef.Op == "kept" && ef.Reason == "changed_after_turn" {
			kept[ef.Path] = true
		}
	}
	if !kept["user.txt"] || !kept["sub/note.txt"] {
		t.Fatalf("kept %#v", kept)
	}
}

func TestUndoForceOverwritesNewerMtime(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	user := filepath.Join(ws.Path, "user.txt")
	mustWrite(t, filepath.Join(ws.Path, "agent.txt"), "v1")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(ws.Path, "agent.txt"), "v2")
	mustWrite(t, user, "created")
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s"}, false); err != nil {
		t.Fatal(err)
	}
	turn, err := e.DB.GetTurn(ws.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	turn.EndedAt = time.Now().Add(-2 * time.Hour)
	if err := e.DB.UpdateTurn(turn); err != nil {
		t.Fatal(err)
	}
	res, err := e.Undo(ctx, UndoRequest{Workspace: ws, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != "ok" {
		t.Fatalf("result %s", res.Result)
	}
	if got := mustRead(t, filepath.Join(ws.Path, "agent.txt")); got != "v1" {
		t.Fatalf("agent.txt %q", got)
	}
	if _, err := os.Stat(user); !os.IsNotExist(err) {
		t.Fatalf("force left user.txt: %v", err)
	}
}

func TestUndoRedoUndoSameTurn(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	path := filepath.Join(ws.Path, "README.md")
	mustWrite(t, path, "base")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit readme"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, "turn")
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s"}, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		res, err := e.Undo(ctx, UndoRequest{Workspace: ws, Seq: 1})
		if err != nil {
			t.Fatal(err)
		}
		if res.Result != "ok" {
			t.Fatalf("pass %d undo %s %#v", i, res.Result, res.Effects)
		}
		if got := mustRead(t, path); got != "base" {
			t.Fatalf("pass %d undo left %q", i, got)
		}
		res, err = e.Redo(ctx, ws, false)
		if err != nil {
			t.Fatal(err)
		}
		if res.Result != "ok" {
			t.Fatalf("pass %d redo %s %#v", i, res.Result, res.Effects)
		}
		if got := mustRead(t, path); got != "turn" {
			t.Fatalf("pass %d redo left %q", i, got)
		}
	}
}

func TestMatchingBytesInsideSnapshotWindowAreRestored(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	path := filepath.Join(ws.Path, "README.md")
	mustWrite(t, path, "base")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit readme"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, "captured")
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s"}, false); err != nil {
		t.Fatal(err)
	}
	turn, err := e.DB.GetTurn(ws.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if turn.EndedAt.IsZero() || turn.Stats.SnapDone.IsZero() {
		t.Fatalf("window start %s done %s", turn.EndedAt, turn.Stats.SnapDone)
	}
	// The bytes match the post snapshot. Moving mtime into the scan window
	// used to look like a user edit. Content that still matches is restored.
	mid := turn.EndedAt
	if turn.Stats.SnapDone.After(turn.EndedAt) {
		mid = turn.EndedAt.Add(turn.Stats.SnapDone.Sub(turn.EndedAt) / 2)
	}
	if err := os.Chtimes(path, mid, mid); err != nil {
		t.Fatal(err)
	}
	res, err := e.Undo(ctx, UndoRequest{Workspace: ws, Seq: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != "ok" {
		t.Fatalf("result %s %#v", res.Result, res.Effects)
	}
	if got := mustRead(t, path); got != "base" {
		t.Fatalf("README.md %q", got)
	}
	if len(turnKept(res)) != 0 {
		t.Fatalf("kept %#v", res.Effects)
	}
	stored, err := e.DB.GetTurn(ws.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "undone" || len(stored.Stats.Kept) != 0 {
		t.Fatalf("status %s kept %#v", stored.Status, stored.Stats.Kept)
	}
}

func turnKept(res UndoResult) []string {
	var out []string
	for _, ef := range res.Effects {
		if ef.Op == "kept" {
			out = append(out, ef.Path)
		}
	}
	return out
}
