package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/lintsinghua/oops/internal/meta"
)

func TestRedoStack(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	path := filepath.Join(ws.Path, "a.txt")
	mustWrite(t, path, "v1")
	session := func(id, from, to string) {
		t.Helper()
		if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: id, Prompt: id}); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, path, to)
		if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: id}, false); err != nil {
			t.Fatal(err)
		}
		if mustRead(t, path) != to {
			t.Fatalf("after %s got %s", id, mustRead(t, path))
		}
		_ = from
	}
	session("t1", "v1", "v2")
	session("t2", "v2", "v3")

	undo := func(want string) {
		t.Helper()
		if _, err := e.Undo(ctx, UndoRequest{Workspace: ws}); err != nil {
			t.Fatal(err)
		}
		if got := mustRead(t, path); got != want {
			t.Fatalf("undo got %q want %q", got, want)
		}
	}
	redo := func(want string) {
		t.Helper()
		if _, err := e.Redo(ctx, ws, false); err != nil {
			t.Fatal(err)
		}
		if got := mustRead(t, path); got != want {
			t.Fatalf("redo got %q want %q", got, want)
		}
	}
	undo("v2")
	undo("v1")
	redo("v2")
	redo("v3")
	undo("v2")
	redo("v3")
	if _, err := e.Redo(ctx, ws, false); !meta.IsNotFound(err) {
		t.Fatalf("redo past the stack: %v", err)
	}

	// A new explicit undo discards the redo stack.
	undo("v2")
	redo("v3")
	if _, err := e.Undo(ctx, UndoRequest{Workspace: ws, Seq: 2}); err != nil {
		t.Fatal(err)
	}
	if mustRead(t, path) != "v2" {
		t.Fatalf("explicit undo got %q", mustRead(t, path))
	}
	redo("v3")
	if _, err := e.Redo(ctx, ws, false); !meta.IsNotFound(err) {
		t.Fatalf("discarded redo came back: %v", err)
	}
}

func TestWaitPendingBounded(t *testing.T) {
	if pendingWait != 30*time.Second {
		t.Fatalf("pending wait bound = %s", pendingWait)
	}
	e, ws := openTest(t)
	start := time.Now()
	if err := e.WaitPending(context.Background(), ws); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	t.Logf("empty WaitPending %s", elapsed)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("empty WaitPending took %s", elapsed)
	}
}

func TestCrashedChildFinishesTurn(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	path := filepath.Join(ws.Path, "a.txt")
	mustWrite(t, path, "v1")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, "v2")
	if err := e.QueueFinish(ws, "s", false); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(e.pendingDir(ws.ID), "lock")
	if err := os.WriteFile(lock, []byte(strconv.Itoa(1<<30)), 0o600); err != nil {
		t.Fatal(err)
	}
	// A later process, not the dead child, finishes the queued post snapshot.
	e2, err := Open(e.Home, t.TempDir(), "", e.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e2.Close() })
	ws2, err := e2.Resolve(ws.Path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := e2.WaitPending(ctx, ws2); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("recovery took %s", time.Since(start))
	}
	if e2.hasPending(ws2) {
		t.Fatal("job left behind")
	}
	turns, err := e2.DB.ListTurns(ws2.ID, 0, false)
	if err != nil || len(turns) != 1 {
		t.Fatalf("turns %#v err %v", turns, err)
	}
	if turns[0].Status == "running" || turns[0].PreSnap == "" || turns[0].PostSnap == "" {
		t.Fatalf("turn not closed cleanly: %+v", turns[0])
	}
	if mustRead(t, path) != "v2" {
		t.Fatalf("workspace changed during recovery: %q", mustRead(t, path))
	}
	res, err := e2.Undo(ctx, UndoRequest{Workspace: ws2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != "ok" || mustRead(t, path) != "v1" {
		t.Fatalf("undo after crash recovery result=%s body=%q", res.Result, mustRead(t, path))
	}
}

func TestNoticeOncePerReason(t *testing.T) {
	e, ws := openTest(t)
	e.AddNotice(ws, "large", "skipped big.bin")
	e.AddNotice(ws, "large", "skipped again")
	e.AddNotice(ws, "partial", "snapshot was partial")
	got := e.TakeNotices(ws)
	if len(got) != 2 {
		t.Fatalf("notices %#v", got)
	}
	e.AddNotice(ws, "large", "third time")
	if again := e.TakeNotices(ws); len(again) != 0 {
		t.Fatalf("repeated notice %#v", again)
	}
}

func TestPartialSnapshotMarksTurn(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		mustWrite(t, filepath.Join(ws.Path, "f", strconv.Itoa(i)+".txt"), "x")
	}
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "slow", Timeout: time.Nanosecond}); err != nil {
		t.Fatal(err)
	}
	turns, err := e.DB.RunningTurns(ws.ID)
	if err != nil || len(turns) != 1 {
		t.Fatal(err)
	}
	// The pre snapshot used a deadline that is already past, so it is partial,
	// and the hook caller still got a turn row.
	snap, err := e.DB.GetSnapshot(turns[0].PreSnap)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Partial {
		t.Fatal("expected a partial pre snapshot")
	}
}

func TestManualSnapSameTree(t *testing.T) {
	e, ws := openTest(t)
	mustWrite(t, filepath.Join(ws.Path, "a.txt"), "v")
	t1, err := e.ManualSnap(context.Background(), ws, "checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	if t1.PreSnap == "" || t1.PreSnap != t1.PostSnap {
		t.Fatalf("pre %q post %q", t1.PreSnap, t1.PostSnap)
	}
}

func TestGCKeepsSafetySnapshot(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	path := filepath.Join(ws.Path, "a.txt")
	mustWrite(t, path, "v1")
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "edit"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, "v2")
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Undo(ctx, UndoRequest{Workspace: ws}); err != nil {
		t.Fatal(err)
	}
	op, err := e.DB.LatestAppliedStackOp(ws.ID)
	if err != nil || op.SafetySnap == "" {
		t.Fatalf("safety op %+v err %v", op, err)
	}
	if _, err := e.GC(ws, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.DB.GetSnapshot(op.SafetySnap); err != nil {
		t.Fatalf("safety snapshot swept: %v", err)
	}
	if _, err := e.Redo(ctx, ws, false); err != nil {
		t.Fatal(err)
	}
	if mustRead(t, path) != "v2" {
		t.Fatalf("redo after gc got %q", mustRead(t, path))
	}
}

func TestEmptyTurnAddsLittle(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	mustWrite(t, filepath.Join(ws.Path, "a.txt"), "same")
	if _, _, err := e.Capture(ctx, ws, SnapOptions{}); err != nil {
		t.Fatal(err)
	}
	before, err := e.Store.Size()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.ManualSnap(ctx, ws, "empty"); err != nil {
		t.Fatal(err)
	}
	after, err := e.Store.Size()
	if err != nil {
		t.Fatal(err)
	}
	if after-before > 4096 {
		t.Fatalf("unchanged turn grew store by %d bytes", after-before)
	}
}

func TestUndoThousandFiles(t *testing.T) {
	e, ws := openTest(t)
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		mustWrite(t, filepath.Join(ws.Path, "d", strconv.Itoa(i)+".txt"), "v1")
	}
	if _, err := e.StartTurn(ctx, TurnInput{Workspace: ws, Agent: "claude", SessionID: "s", Prompt: "bulk"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		mustWrite(t, filepath.Join(ws.Path, "d", strconv.Itoa(i)+".txt"), "v2")
	}
	if _, err := e.CloseTurn(ctx, TurnInput{Workspace: ws, SessionID: "s"}, false); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	res, err := e.Undo(ctx, UndoRequest{Workspace: ws})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	limit := time.Second
	if runtime.GOOS == "windows" {
		limit = 2 * time.Second
	}
	t.Logf("undo 1000 files %s", elapsed)
	if elapsed > limit {
		t.Fatalf("undo 1000 files took %s", elapsed)
	}
	if res.Result != "ok" {
		t.Fatalf("result %s", res.Result)
	}
	if mustRead(t, filepath.Join(ws.Path, "d", "0.txt")) != "v1" {
		t.Fatal("first file not restored")
	}
}
