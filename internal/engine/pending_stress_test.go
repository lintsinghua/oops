package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/lintsinghua/oops/internal/meta"
)

// TestUndoImmediatelyAfterStop is the race where Stop has returned and the
// post snapshot is still queued. Undo must apply the turn on every iteration.
func TestUndoImmediatelyAfterStop(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	path := filepath.Join(ws.Path, "a.txt")
	mustWrite(t, path, "v1")
	const n = 200
	for i := 0; i < n; i++ {
		seq := i + 1
		if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit"}); err != nil {
			t.Fatalf("iter %d start: %v", seq, err)
		}
		mustWrite(t, path, "v2")
		if err := e.QueueFinish(ws, "s", false); err != nil {
			t.Fatalf("iter %d queue: %v", seq, err)
		}
		turn, err := e.DB.GetTurn(ws.ID, seq)
		if err != nil {
			t.Fatal(err)
		}
		if turn.Status != meta.StatusPending || turn.EndedAt.IsZero() || turn.PostSnap != "" {
			t.Fatalf("iter %d after stop: status %s post %q", seq, turn.Status, turn.PostSnap)
		}
		open, err := e.DB.RunningTurns(ws.ID)
		if err != nil || len(open) != 0 {
			t.Fatalf("iter %d running: %+v err %v", seq, open, err)
		}
		job := filepath.Join(e.pendingDir(ws.ID), strconv.Itoa(seq)+".json")
		if _, err := os.Stat(job); err != nil {
			t.Fatalf("iter %d job: %v", seq, err)
		}
		if _, err := e.Undo(ctx, UndoRequest{Workspace: ws}); err != nil {
			t.Fatalf("iter %d undo: %v", seq, err)
		}
		if got := mustRead(t, path); got != "v1" {
			t.Fatalf("iter %d restored %q", seq, got)
		}
	}
}

func TestUndoRefusesTurnThatNeverStopped(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	path := filepath.Join(ws.Path, "a.txt")
	mustWrite(t, path, "v1")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, "v2")
	start := time.Now()
	_, err := e.Undo(ctx, UndoRequest{Workspace: ws})
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("live turn refused after %s", time.Since(start))
	}
	var running *RunningError
	if !errors.As(err, &running) {
		t.Fatalf("err %v", err)
	}
	if mustRead(t, path) != "v2" {
		t.Fatal("refused undo wrote the file")
	}
}
