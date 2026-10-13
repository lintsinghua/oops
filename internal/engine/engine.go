// Package engine ties snapshots, turns, and restore into the operations the CLI uses.
package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lintsinghua/oops/internal/config"
	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/store"
)

// Engine is one open oops data directory.
type Engine struct {
	Home     string
	UserHome string
	Config   config.Config
	DB       *meta.DB
	Store    *store.Store
	Now      time.Time
}

// Open prepares the data directory. home overrides the default location.
func Open(home, userHome, configPath string, now time.Time) (*Engine, error) {
	if home == "" {
		var err error
		home, err = config.DataHome()
		if err != nil {
			return nil, err
		}
	}
	if now.IsZero() {
		now = time.Now()
	}
	if configPath == "" {
		if p, err := config.ConfigPath(); err == nil {
			configPath = p
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	if err := platform.SecureMkdir(home); err != nil {
		return nil, err
	}
	if err := platform.SecureMkdir(filepath.Join(home, "logs")); err != nil {
		return nil, err
	}
	db, err := meta.Open(home)
	if err != nil {
		return nil, err
	}
	st, err := store.Open(home)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Engine{Home: home, UserHome: userHome, Config: cfg, DB: db, Store: st, Now: now}, nil
}

// Close releases resources.
func (e *Engine) Close() error {
	e.Store.Close()
	return e.DB.Close()
}

// Workspace is a resolved root.
type Workspace struct {
	ID   string
	Path string
}

// Resolve finds the workspace root for dir.
func (e *Engine) Resolve(dir string) (Workspace, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return Workspace{}, err
		}
	}
	root, err := platform.FindRoot(dir)
	if err != nil {
		return Workspace{}, err
	}
	if platform.ForbiddenRoot(root) {
		return Workspace{}, &RootError{Path: root}
	}
	id, abs, err := platform.WorkspaceID(root)
	if err != nil {
		return Workspace{}, err
	}
	if err := e.DB.UpsertWorkspace(id, abs, e.Now); err != nil {
		return Workspace{}, err
	}
	if err := platform.SecureMkdir(e.wsDir(id)); err != nil {
		return Workspace{}, err
	}
	return Workspace{ID: id, Path: abs}, nil
}

func (e *Engine) wsDir(id string) string {
	return filepath.Join(e.Home, "workspaces", id)
}

// RootError means the directory cannot be a workspace.
type RootError struct{ Path string }

func (r *RootError) Error() string { return "refusing workspace root " + r.Path }

// SnapOptions controls a capture.
type SnapOptions struct {
	Timeout time.Duration
}

// Capture scans the workspace and stores a snapshot row.
func (e *Engine) Capture(ctx context.Context, ws Workspace, opt SnapOptions) (meta.Snapshot, *snapshot.Result, error) {
	cache, err := snapshot.LoadCache(filepath.Join(e.wsDir(ws.ID), "statcache"))
	if err != nil {
		return meta.Snapshot{}, nil, err
	}
	maxSize, err := config.ParseSize(e.Config.Snapshot.MaxFileSize)
	if err != nil {
		return meta.Snapshot{}, nil, err
	}
	timeout := opt.Timeout
	if timeout == 0 {
		timeout, err = config.ParseDuration(e.Config.Snapshot.Timeout)
		if err != nil {
			return meta.Snapshot{}, nil, err
		}
	}
	res, err := snapshot.Scan(ctx, e.Store, ws.Path, snapshot.Options{
		MaxFileSize: maxSize,
		Timeout:     timeout,
		Protect:     e.Config.Snapshot.Protect,
		Exclude:     e.Config.Snapshot.Exclude,
		RespectGit:  e.Config.Snapshot.RespectGitignore,
		Prev:        cache,
		Now:         e.Now,
	})
	if err != nil {
		return meta.Snapshot{}, nil, err
	}
	cachePath := filepath.Join(e.wsDir(ws.ID), "statcache")
	if !res.Changed {
		if err := res.Cache.Touch(cachePath); err != nil {
			if err := res.Cache.Save(cachePath); err != nil {
				return meta.Snapshot{}, nil, err
			}
		}
	} else if err := res.Cache.Save(cachePath); err != nil {
		return meta.Snapshot{}, nil, err
	}
	id, err := newID()
	if err != nil {
		return meta.Snapshot{}, nil, err
	}
	var skipped []meta.Skipped
	for _, s := range res.Skipped {
		skipped = append(skipped, meta.Skipped{Path: s.Path, Reason: s.Skipped, Size: s.Size})
	}
	row := meta.Snapshot{
		ID: id, WS: ws.ID, Root: res.Root.String(), CreatedAt: e.Now,
		Partial: res.Partial, Skipped: skipped,
	}
	if err := e.DB.InsertSnapshot(row); err != nil {
		return meta.Snapshot{}, nil, err
	}
	return row, res, nil
}

// Files loads the flattened tree for a snapshot id.
func (e *Engine) Files(snapID string) (map[string]snapshot.Meta, meta.Snapshot, error) {
	row, err := e.DB.GetSnapshot(snapID)
	if err != nil {
		return nil, row, err
	}
	id, err := store.ParseID(row.Root)
	if err != nil {
		return nil, row, err
	}
	files, err := snapshot.Flatten(e.Store, id)
	return files, row, err
}

// ReadBlob returns file bytes for a meta entry.
func (e *Engine) ReadBlob(m *snapshot.Meta) []byte {
	if m == nil || m.Kind != snapshot.KindFile || m.ID.IsZero() || m.Skipped != "" {
		if m != nil && m.Kind == snapshot.KindSymlink {
			return []byte(m.Target)
		}
		return nil
	}
	b, err := e.Store.Get(m.ID)
	if err != nil {
		return nil
	}
	return b
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Logf appends a line to the rotating log.
func (e *Engine) Logf(format string, args ...any) {
	path := filepath.Join(e.Home, "logs", "oops.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	line, _ := json.Marshal(struct {
		Time string `json:"time"`
		Msg  string `json:"msg"`
	}{Time: e.Now.UTC().Format(time.RFC3339), Msg: sprintf(format, args...)})
	_, _ = f.Write(append(line, '\n'))
	rotateLog(path)
}

func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func rotateLog(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < 5<<20 {
		return
	}
	_ = os.Remove(path + ".2")
	_ = os.Rename(path+".1", path+".2")
	_ = os.Rename(path, path+".1")
}
