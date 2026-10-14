package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/platform"
)

// pendingJob is a post-turn snapshot the hook did not wait for.
type pendingJob struct {
	Seq  int  `json:"seq"`
	Late bool `json:"late"`
}

func (e *Engine) pendingDir(id string) string {
	return filepath.Join(e.wsDir(id), "pending")
}

// QueueFinish records that the running turn still needs its post snapshot.
// Before it returns it marks the turn ended in the database, so a command
// that starts after the hook returns does not see a still-running turn.
// The caller can return immediately and let finish-pending take the snapshot.
func (e *Engine) QueueFinish(ws Workspace, session string, late bool) error {
	t, err := e.runningTurn(ws, session)
	if err != nil {
		return err
	}
	if err := e.DB.MarkTurnPending(t.ID, e.Now); err != nil {
		return err
	}
	dir := e.pendingDir(ws.ID)
	if err := platform.SecureMkdir(dir); err != nil {
		return err
	}
	body, err := json.Marshal(pendingJob{Seq: t.Seq, Late: late})
	if err != nil {
		return err
	}
	return platform.WriteFileAtomic(filepath.Join(dir, strconv.Itoa(t.Seq)+".json"), body, 0o600)
}

func (e *Engine) runningTurn(ws Workspace, session string) (meta.Turn, error) {
	if session != "" {
		return e.DB.RunningBySession(ws.ID, session)
	}
	running, rerr := e.DB.RunningTurns(ws.ID)
	if rerr != nil {
		return meta.Turn{}, rerr
	}
	if len(running) == 1 {
		return running[0], nil
	}
	return meta.Turn{}, os.ErrNotExist
}

// SealRunning closes every running turn from the tree as it is now.
// The next turn's pre hook calls this so a background post snapshot cannot
// later attribute that turn's edits to the previous one.
func (e *Engine) SealRunning(ctx context.Context, ws Workspace) error {
	if err := e.lockPending(ws); err != nil {
		if err := e.WaitPending(ctx, ws); err != nil {
			return err
		}
		if err := e.lockPending(ws); err != nil {
			return err
		}
	}
	defer e.unlockPending(ws)
	running, err := e.DB.RunningTurns(ws.ID)
	if err != nil {
		return err
	}
	pending, err := e.DB.PendingTurns(ws.ID)
	if err != nil {
		return err
	}
	if len(running) == 0 && len(pending) == 0 {
		return nil
	}
	for _, t := range append(running, pending...) {
		if _, err := e.finish(ctx, ws, t, true); err != nil {
			return err
		}
		_ = os.Remove(filepath.Join(e.pendingDir(ws.ID), strconv.Itoa(t.Seq)+".json"))
	}
	return nil
}

// pendingWait is the longest a command waits for a background post snapshot
// before it finishes that snapshot itself.
const pendingWait = 30 * time.Second

// snapshotWait is how long undo waits for a post snapshot Stop already queued.
const snapshotWait = 5 * time.Second

// WaitPending blocks until a background post-snapshot for ws has finished.
// If the background process is gone, this process finishes the job itself.
// The wait is bounded by pendingWait. When nothing is queued it returns
// without sleeping.
func (e *Engine) WaitPending(ctx context.Context, ws Workspace) error {
	return e.waitPending(ctx, ws, pendingWait)
}

// WaitSnapshot waits for a post snapshot that Stop has already queued.
// A turn that is still running, with no queued snapshot, returns immediately.
// The wait is bounded by snapshotWait. At the bound this process stores the
// snapshot itself when the background child is gone.
func (e *Engine) WaitSnapshot(ctx context.Context, ws Workspace) error {
	return e.waitPending(ctx, ws, snapshotWait)
}

func (e *Engine) waitPending(ctx context.Context, ws Workspace, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for {
		outstanding, err := e.snapshotOutstanding(ws)
		if err != nil {
			return err
		}
		if !outstanding {
			return nil
		}
		if !e.pendingOwnerAlive(ws) || time.Now().After(deadline) {
			if err := e.FinishPending(ctx, ws); err != nil {
				return err
			}
			outstanding, err = e.snapshotOutstanding(ws)
			if err != nil {
				return err
			}
			if !outstanding {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return e.FinishPending(ctx, ws)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// snapshotOutstanding reports a queued post snapshot or a turn Stop has
// ended whose post snapshot is not stored yet.
func (e *Engine) snapshotOutstanding(ws Workspace) (bool, error) {
	if e.hasPending(ws) {
		return true, nil
	}
	pending, err := e.DB.PendingTurns(ws.ID)
	if err != nil {
		return false, err
	}
	return len(pending) > 0, nil
}

func (e *Engine) hasPending(ws Workspace) bool {
	entries, err := os.ReadDir(e.pendingDir(ws.ID))
	if err != nil {
		return false
	}
	for _, ent := range entries {
		if strings.HasSuffix(ent.Name(), ".json") {
			return true
		}
	}
	return false
}

// FinishPending takes every queued post-snapshot for ws.
// A second caller while one is running returns without doing the work.
func (e *Engine) FinishPending(ctx context.Context, ws Workspace) error {
	if err := e.lockPending(ws); err != nil {
		return nil
	}
	defer e.unlockPending(ws)
	dir := e.pendingDir(ws.ID)
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, ent := range entries {
		if !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var job pendingJob
		if json.Unmarshal(b, &job) != nil {
			_ = os.Remove(path)
			continue
		}
		t, err := e.DB.GetTurn(ws.ID, job.Seq)
		if err != nil || (t.Status != "running" && t.Status != meta.StatusPending) {
			_ = os.Remove(path)
			continue
		}
		if _, err := e.finish(ctx, ws, t, job.Late); err != nil {
			return err
		}
		_ = os.Remove(path)
	}
	pending, err := e.DB.PendingTurns(ws.ID)
	if err != nil {
		return err
	}
	for _, t := range pending {
		if t.PostSnap != "" {
			continue
		}
		if _, err := e.finish(ctx, ws, t, false); err != nil {
			return err
		}
	}
	return nil
}

// completePendingTurn stores the post snapshot for a turn Stop already ended.
func (e *Engine) completePendingTurn(ctx context.Context, ws Workspace, t meta.Turn) (meta.Turn, error) {
	if t.Status != meta.StatusPending {
		return t, nil
	}
	done, err := e.finish(ctx, ws, t, false)
	if err != nil {
		return t, err
	}
	_ = os.Remove(filepath.Join(e.pendingDir(ws.ID), strconv.Itoa(t.Seq)+".json"))
	return done, nil
}

// FinishAllPending closes queued post-snapshots in every workspace.
func (e *Engine) FinishAllPending(ctx context.Context) error {
	list, err := e.DB.Workspaces()
	if err != nil {
		return err
	}
	for _, w := range list {
		if err := e.FinishPending(ctx, Workspace{ID: w.ID, Path: w.Path}); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) lockPending(ws Workspace) error {
	if err := platform.SecureMkdir(e.pendingDir(ws.ID)); err != nil {
		return err
	}
	path := filepath.Join(e.pendingDir(ws.ID), "lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if e.pendingOwnerAlive(ws) {
			return err
		}
		_ = os.Remove(path)
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
	}
	_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
	return f.Close()
}

func (e *Engine) unlockPending(ws Workspace) {
	_ = os.Remove(filepath.Join(e.pendingDir(ws.ID), "lock"))
}

func (e *Engine) pendingOwnerAlive(ws Workspace) bool {
	b, err := os.ReadFile(filepath.Join(e.pendingDir(ws.ID), "lock"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 || pid == os.Getpid() {
		return false
	}
	return platform.ProcessAlive(pid)
}
