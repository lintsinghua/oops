package engine

import (
	"os"
	"path/filepath"
	"time"

	"github.com/lintsinghua/oops/internal/config"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/store"
)

// GCResult describes a collection pass.
type GCResult struct {
	TurnsRemoved   int   `json:"turns_removed"`
	ObjectsRemoved int   `json:"objects_removed"`
	BytesFreed     int64 `json:"bytes_freed"`
	DryRun         bool  `json:"dry_run"`
}

// GC drops expired turns and unreferenced objects.
func (e *Engine) GC(ws Workspace, dry, all bool) (GCResult, error) {
	res := GCResult{DryRun: dry}
	if all {
		if !dry {
			if err := e.DB.DeleteWorkspaceData(ws.ID); err != nil {
				return res, err
			}
			_ = os.RemoveAll(e.wsDir(ws.ID))
		}
	} else if err := e.expireTurns(ws, &res, dry); err != nil {
		return res, err
	}
	freed, objs, err := e.sweep(ws, dry, all)
	res.BytesFreed += freed
	res.ObjectsRemoved += objs
	return res, err
}

func (e *Engine) expireTurns(ws Workspace, res *GCResult, dry bool) error {
	keepDays := e.Config.Retention.KeepDays
	keepN := e.Config.Retention.KeepTurns
	turns, err := e.DB.ListTurns(ws.ID, 0, false)
	if err != nil {
		return err
	}
	cutoff := e.Now.AddDate(0, 0, -keepDays)
	var drop []int64
	for i, t := range turns {
		// turns are newest first. Index from the end is age rank.
		rank := len(turns) - i
		old := !t.StartedAt.IsZero() && t.StartedAt.Before(cutoff)
		if rank > keepN || old {
			drop = append(drop, t.ID)
		}
	}
	res.TurnsRemoved = len(drop)
	if dry {
		return nil
	}
	for _, id := range drop {
		if err := e.DB.DeleteTurn(id); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) sweep(ws Workspace, dry, all bool) (int64, int, error) {
	keepSnaps, err := e.DB.ReferencedSnapshots(ws.ID)
	if err != nil {
		return 0, 0, err
	}
	if all {
		keepSnaps = map[string]struct{}{}
	}
	roots := map[store.ID]struct{}{}
	for id := range keepSnaps {
		row, err := e.DB.GetSnapshot(id)
		if err != nil {
			continue
		}
		rid, err := store.ParseID(row.Root)
		if err != nil {
			continue
		}
		roots[rid] = struct{}{}
	}
	// Other workspaces still reference objects. Always keep their roots too.
	others, err := e.DB.Workspaces()
	if err != nil {
		return 0, 0, err
	}
	for _, w := range others {
		if w.ID == ws.ID && all {
			continue
		}
		refs, err := e.DB.ReferencedSnapshots(w.ID)
		if err != nil {
			return 0, 0, err
		}
		for id := range refs {
			row, err := e.DB.GetSnapshot(id)
			if err != nil {
				continue
			}
			rid, err := store.ParseID(row.Root)
			if err == nil {
				roots[rid] = struct{}{}
			}
		}
	}
	live := map[string]struct{}{}
	for id := range roots {
		if err := mark(e.Store, id, live); err != nil {
			return 0, 0, err
		}
	}
	grace := e.Now.Add(-time.Hour)
	var freed int64
	var n int
	objRoot := filepath.Join(e.Home, "objects")
	err = filepath.WalkDir(objRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, rerr := filepath.Rel(objRoot, path)
		if rerr != nil {
			return rerr
		}
		hexID := filepath.ToSlash(rel)
		hexID = hexID[:2] + hexID[3:]
		if _, ok := live[hexID]; ok {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(grace) {
			return nil
		}
		freed += info.Size()
		n++
		if !dry {
			return os.Remove(path)
		}
		return nil
	})
	if !dry {
		rows, err2 := e.DB.UnreferencedSnapshotRows(ws.ID, keepSnaps)
		if err2 == nil {
			for _, id := range rows {
				_ = e.DB.DeleteSnapshot(id)
			}
		}
	}
	return freed, n, err
}

func mark(st *store.Store, id store.ID, live map[string]struct{}) error {
	if id.IsZero() {
		return nil
	}
	key := id.String()
	if _, ok := live[key]; ok {
		return nil
	}
	live[key] = struct{}{}
	raw, err := st.Get(id)
	if err != nil {
		return nil
	}
	entries, err := snapshot.DecodeTree(raw)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.ID.IsZero() {
			if e.Kind == snapshot.KindDir {
				if err := mark(st, e.ID, live); err != nil {
					return err
				}
			} else {
				live[e.ID.String()] = struct{}{}
			}
		}
	}
	return nil
}

// OverQuota reports whether retention limits are exceeded.
func (e *Engine) OverQuota(ws Workspace) (bool, error) {
	maxWS, err := config.ParseSize(e.Config.Retention.WorkspaceMax)
	if err != nil {
		return false, err
	}
	n, err := e.Store.Size()
	if err != nil {
		return false, err
	}
	return maxWS > 0 && n > maxWS, nil
}
