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
// The caller can return immediately and let finish-pending take the snapshot.
func (e *Engine) QueueFinish(ws Workspace, session string, late bool) error {
	t, err := e.runningTurn(ws, session)
	if err != nil {
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
	t, err := e.DB.RunningBySession(ws.ID, session)
	if err == nil {
		return t, nil
	}
	if !meta.IsNotFound(err) {
		return meta.Turn{}, err
	}
	running, rerr := e.DB.RunningTurns(ws.ID)
	if rerr != nil {
		return meta.Turn{}, rerr
	}
	if len(running) == 1 {
		return running[0], nil
	}
	return meta.Turn{}, err
}

// WaitPending blocks until a background post-snapshot for ws has finished.
// If the background process is gone, this process finishes the job itself.
func (e *Engine) WaitPending(ctx context.Context, ws Workspace) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		if !e.hasPending(ws) {
			return nil
		}
		if !e.pendingOwnerAlive(ws) || time.Now().After(deadline) {
			if err := e.FinishPending(ctx, ws); err != nil {
				return err
			}
			if !e.hasPending(ws) {
				return nil
			}
		}
		if time.Now().After(deadline) && e.hasPending(ws) {
			return e.FinishPending(ctx, ws)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
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
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
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
		if err != nil || t.Status != "running" {
			_ = os.Remove(path)
			continue
		}
		if _, err := e.finish(ctx, ws, t, job.Late); err != nil {
			return err
		}
		_ = os.Remove(path)
	}
	return nil
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
