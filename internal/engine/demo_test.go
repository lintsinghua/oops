package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lintsinghua/oops/internal/meta"
)

func TestStopThenImmediateNewTurn(t *testing.T) {
	t.Run("stop exits before snapshot", func(t *testing.T) {
		e, ws := openTest(t)
		ctx := context.Background()
		path := filepath.Join(ws.Path, "a.txt")
		mustWrite(t, path, "v1")
		if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s1", Prompt: "edit"}); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, path, "v2")
		if err := e.QueueFinish(ws, "s1", false); err != nil {
			t.Fatal(err)
		}
		if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s2", Prompt: "delete"}); err != nil {
			t.Fatal(err)
		}
		t1, err := e.DB.GetTurn(ws.ID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if t1.Status != "done" || t1.PostSnap == "" {
			t.Fatalf("turn 1 was still open: %+v", t1.Status)
		}
		sealed := t1.PostSnap
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s2"}, false); err != nil {
			t.Fatal(err)
		}
		t1, err = e.DB.GetTurn(ws.ID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if t1.PostSnap != sealed {
			t.Fatal("turn 1 post changed after the next turn")
		}
		if act := changeAction(t, e, t1, "a.txt"); act != "modified" {
			t.Fatalf("turn 1 action %s", act)
		}
		t2, err := e.DB.GetTurn(ws.ID, 2)
		if err != nil {
			t.Fatal(err)
		}
		if act := changeAction(t, e, t2, "a.txt"); act != "deleted" {
			t.Fatalf("turn 2 action %s", act)
		}
		if _, err := e.Undo(ctx, UndoRequest{Workspace: ws}); err != nil {
			t.Fatal(err)
		}
		if got := mustRead(t, path); got != "v2" {
			t.Fatalf("undo restored %q", got)
		}
		body, err := json.Marshal(pendingJob{Seq: 1, Late: false})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(e.pendingDir(ws.ID), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(e.pendingDir(ws.ID), "1.json"), body, 0o600); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, path, "v3")
		if err := e.FinishPending(ctx, ws); err != nil {
			t.Fatal(err)
		}
		t1, err = e.DB.GetTurn(ws.ID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if t1.PostSnap != sealed || t1.Status != "done" {
			t.Fatalf("late finish rewrote turn 1 status %s", t1.Status)
		}
	})

	t.Run("stop never queued", func(t *testing.T) {
		e, ws := openTest(t)
		ctx := context.Background()
		path := filepath.Join(ws.Path, "a.txt")
		mustWrite(t, path, "v1")
		if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s1", Prompt: "edit"}); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, path, "v2")
		if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s2", Prompt: "delete"}); err != nil {
			t.Fatal(err)
		}
		t1, err := e.DB.GetTurn(ws.ID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if t1.Status == "running" {
			t.Fatal("previous turn was still running when the next session started")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s2"}, false); err != nil {
			t.Fatal(err)
		}
		t1, _ = e.DB.GetTurn(ws.ID, 1)
		t2, err := e.DB.GetTurn(ws.ID, 2)
		if err != nil {
			t.Fatal(err)
		}
		if act := changeAction(t, e, t1, "a.txt"); act != "modified" {
			t.Fatalf("turn 1 action %s", act)
		}
		if act := changeAction(t, e, t2, "a.txt"); act != "deleted" {
			t.Fatalf("turn 2 action %s", act)
		}
	})
}

func changeAction(t *testing.T, e *Engine, turn meta.Turn, path string) string {
	t.Helper()
	changes, err := e.ChangesBetween(turn)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changes {
		if c.Path == path || c.Path == filepath.ToSlash(path) {
			return c.Action
		}
	}
	t.Fatalf("missing %s in %+v", path, changes)
	return ""
}
