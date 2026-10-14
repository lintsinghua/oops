package engine

import (
	"os"
	"path/filepath"
	"time"

	"github.com/lintsinghua/oops/internal/config"
	"github.com/lintsinghua/oops/internal/platform"
)

// Maintain drops turns past the count and age limits, then drops the oldest
// turns until snapshot bytes fit the workspace and global caps.
// Each call deletes at most 64 turns. Objects written in the last hour are
// left in place so an in-flight snapshot cannot lose its blobs.
func (e *Engine) Maintain(ws Workspace) (GCResult, error) {
	if err := e.lockMaintain(ws); err != nil {
		return GCResult{}, nil
	}
	defer e.unlockMaintain(ws)
	var res GCResult
	if err := e.expireTurns(ws, &res, false); err != nil {
		return res, err
	}
	n, over, err := e.enforceCaps(ws, 64)
	if err != nil {
		return res, err
	}
	res.TurnsRemoved += n
	if res.TurnsRemoved > 0 {
		freed, objs, serr := e.sweep(ws, false, false)
		res.BytesFreed += freed
		res.ObjectsRemoved += objs
		if serr != nil {
			return res, serr
		}
	}
	if over {
		e.AddNotice(ws, "quota", "snapshot storage is over the cap. Oldest turns were removed. oops doctor")
	}
	return res, nil
}

// MaintainAll runs Maintain for every workspace.
func (e *Engine) MaintainAll() error {
	list, err := e.DB.Workspaces()
	if err != nil {
		return err
	}
	for _, w := range list {
		if _, err := e.Maintain(Workspace{ID: w.ID, Path: w.Path}); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) enforceCaps(ws Workspace, budget int) (int, bool, error) {
	wsMax, err := config.ParseSize(e.Config.Retention.WorkspaceMax)
	if err != nil {
		return 0, false, err
	}
	globalMax, err := config.ParseSize(e.Config.Retention.GlobalMax)
	if err != nil {
		return 0, false, err
	}
	removed := 0
	for removed < budget {
		total, reclaimable, err := e.objectBytes()
		if err != nil {
			return removed, false, err
		}
		overWS := wsMax > 0 && total > wsMax
		overGlobal := globalMax > 0 && total > globalMax
		if !overWS && !overGlobal {
			return removed, false, nil
		}
		reclaimOver := (wsMax > 0 && reclaimable > wsMax) || (globalMax > 0 && reclaimable > globalMax)
		if !reclaimOver {
			// Recent objects are inside the one-hour grace window.
			return removed, true, nil
		}
		n, err := e.DB.CountTurns(ws.ID)
		if err != nil {
			return removed, false, err
		}
		if n <= 1 {
			return removed, true, nil
		}
		oldest, err := e.DB.OldestTurns(ws.ID, 1)
		if err != nil {
			return removed, false, err
		}
		if len(oldest) == 0 {
			return removed, true, nil
		}
		if err := e.DB.DeleteTurn(oldest[0].ID); err != nil {
			return removed, false, err
		}
		removed++
		if _, _, err := e.sweep(ws, false, false); err != nil {
			return removed, false, err
		}
	}
	total, _, err := e.objectBytes()
	if err != nil {
		return removed, true, err
	}
	over := (wsMax > 0 && total > wsMax) || (globalMax > 0 && total > globalMax)
	return removed, over, nil
}

func (e *Engine) objectBytes() (total, reclaimable int64, err error) {
	grace := e.Now.Add(-time.Hour)
	objRoot := filepath.Join(e.Home, "objects")
	err = filepath.WalkDir(objRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if info.ModTime().Before(grace) {
			reclaimable += info.Size()
		}
		return nil
	})
	return total, reclaimable, err
}

func (e *Engine) lockMaintain(ws Workspace) error {
	if err := platform.SecureMkdir(e.wsDir(ws.ID)); err != nil {
		return err
	}
	path := filepath.Join(e.wsDir(ws.ID), "maintain.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		info, statErr := os.Stat(path)
		if statErr == nil && e.Now.Sub(info.ModTime()) > 10*time.Minute {
			_ = os.Remove(path)
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		}
	}
	if err != nil {
		return err
	}
	return f.Close()
}

func (e *Engine) unlockMaintain(ws Workspace) {
	_ = os.Remove(filepath.Join(e.wsDir(ws.ID), "maintain.lock"))
}
