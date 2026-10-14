package engine

import (
	"context"
	"path/filepath"
	"testing"
)

// A file kept as the user's during a partial undo stays theirs through later
// undo and redo of that same chain, unless --force.
func TestKeptFileSurvivesUndoRedoUndo(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	base := filepath.Join(ws.Path, "src", "a.txt")
	mustWrite(t, base, "base\n")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "t", Prompt: "edit"}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		mustWrite(t, filepath.Join(ws.Path, "src", "m"+string(rune('0'+i))), "x\n")
	}
	mustWrite(t, base, "base\nagent\n")
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "t"}, false); err != nil {
		t.Fatal(err)
	}
	user := "base\nagent\nuser\n"
	mustWrite(t, base, user)

	undo, err := e.Undo(ctx, UndoRequest{Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if undo.Result != "partial" || mustRead(t, base) != user {
		t.Fatalf("undo %s %q", undo.Result, mustRead(t, base))
	}
	redo, err := e.Redo(ctx, ws, false)
	if err != nil {
		t.Fatal(err)
	}
	if redo.Result != "partial" || mustRead(t, base) != user {
		t.Fatalf("redo %s %q", redo.Result, mustRead(t, base))
	}
	again, err := e.Undo(ctx, UndoRequest{Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if again.Result != "partial" || mustRead(t, base) != user {
		t.Fatalf("second undo %s %q effects %#v", again.Result, mustRead(t, base), again.Effects)
	}
	for _, ef := range again.Effects {
		if ef.Path == "src/a.txt" && ef.Op != "kept" {
			t.Fatalf("src/a.txt op %s", ef.Op)
		}
	}
	redo2, err := e.Redo(ctx, ws, false)
	if err != nil {
		t.Fatal(err)
	}
	if redo2.Result != "partial" || mustRead(t, base) != user {
		t.Fatalf("second redo %s %q", redo2.Result, mustRead(t, base))
	}

	forced, err := e.Undo(ctx, UndoRequest{Workspace: ws, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if forced.Result == "partial" || mustRead(t, base) == user {
		t.Fatalf("force result %s body %q", forced.Result, mustRead(t, base))
	}
	if mustRead(t, base) != "base\n" {
		t.Fatalf("force left %q", mustRead(t, base))
	}
}
