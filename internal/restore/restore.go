// Package restore plans and applies undos without clobbering newer edits.
package restore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/store"
)

// Item is one planned path change.
type Item struct {
	Path   string         `json:"path"`
	Op     string         `json:"op"` // restore, remove, revert, kept, skip, unprotected
	Reason string         `json:"reason,omitempty"`
	Before *snapshot.Meta `json:"before,omitempty"`
	After  *snapshot.Meta `json:"after,omitempty"`
	Want   *snapshot.Meta `json:"want,omitempty"`
}

// Plan decides how to move cur toward desired for each path.
// expected is the state the agent left behind. desired is the undo target.
func Plan(paths []string, expected, desired, cur map[string]snapshot.Meta, force bool) []Item {
	var items []Item
	seen := map[string]struct{}{}
	for _, p := range paths {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		exp := lookupPlanned(expected, p, paths)
		des := lookupPlanned(desired, p, paths)
		now := lookupPlanned(cur, p, paths)
		item := Item{Path: p, Before: now, Want: des, After: des}
		switch {
		case snapshot.SameState(now, des):
			item.Op = "skip"
			item.Reason = "already"
		case needsBytes(des) && des.Skipped == "too_large" || needsBytes(exp) && exp != nil && exp.Skipped == "too_large" && des != nil && des.ID.IsZero() && des.Kind == snapshot.KindFile:
			item.Op = "unprotected"
			item.Reason = "too_large"
		case snapshot.SameState(now, exp) || force:
			item.Op = opFor(now, des)
			if force && !snapshot.SameState(now, exp) {
				item.Reason = "forced"
			}
		default:
			item.Op = "kept"
			item.Reason = "changed_after_turn"
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items
}

func opFor(now, des *snapshot.Meta) string {
	if des == nil {
		return "remove"
	}
	if des.Kind == snapshot.KindDir {
		if now == nil {
			return "restore"
		}
		return "skip"
	}
	if now == nil {
		return "restore"
	}
	return "revert"
}

func needsBytes(m *snapshot.Meta) bool {
	return m != nil && m.Kind == snapshot.KindFile
}

// lookupPlanned finds path in m. An exact key wins. A case-insensitive match
// is used only when no other path in this plan owns that spelling, so a
// case-only rename stays a remove of the new name plus a restore of the old one.
func lookupPlanned(m map[string]snapshot.Meta, path string, planned []string) *snapshot.Meta {
	if v, ok := m[path]; ok {
		cp := v
		return &cp
	}
	key := platform.Key(path)
	for k, v := range m {
		if platform.Key(k) != key {
			continue
		}
		if plannedOwns(planned, k) {
			continue
		}
		cp := v
		return &cp
	}
	return nil
}

func plannedOwns(planned []string, key string) bool {
	for _, p := range planned {
		if p == key {
			return true
		}
	}
	return false
}

// Apply writes the plan into root. st supplies blob bytes.
func Apply(root string, st *store.Store, items []Item) error {
	var dirs []Item
	var writes []Item
	var removes []Item
	for _, it := range items {
		switch it.Op {
		case "restore", "revert":
			if it.Want != nil && it.Want.Kind == snapshot.KindDir {
				dirs = append(dirs, it)
			} else {
				writes = append(writes, it)
			}
		case "remove":
			if it.Before != nil && it.Before.Kind == snapshot.KindDir || (it.Want == nil && strings.HasSuffix(it.Path, "/")) {
				dirs = append(dirs, it)
			} else {
				removes = append(removes, it)
			}
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i].Path) < len(dirs[j].Path) })
	for _, it := range dirs {
		if it.Op == "remove" {
			continue
		}
		full, err := platform.SafeRel(root, it.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(platform.LongPath(full), 0o755); err != nil {
			return err
		}
	}
	renamed, err := caseRenames(root, writes, removes)
	if err != nil {
		return err
	}
	for _, it := range writes {
		if err := writeOne(root, st, it); err != nil {
			if platform.IsBusy(err) {
				return &BusyError{Path: it.Path, Err: err}
			}
			return err
		}
	}
	sort.Slice(removes, func(i, j int) bool { return len(removes[i].Path) > len(removes[j].Path) })
	for _, it := range removes {
		if _, ok := renamed[it.Path]; ok {
			continue
		}
		full, err := platform.SafeRel(root, it.Path)
		if err != nil {
			return err
		}
		if err := os.Remove(platform.LongPath(full)); err != nil && !os.IsNotExist(err) {
			return &BusyError{Path: it.Path, Err: err}
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i].Path) > len(dirs[j].Path) })
	for _, it := range dirs {
		if it.Op != "remove" {
			continue
		}
		full, err := platform.SafeRel(root, it.Path)
		if err != nil {
			return err
		}
		_ = os.Remove(platform.LongPath(full))
	}
	return nil
}

// caseRenames moves a case-only rename onto the spelling undo should restore.
// The returned set is the remove paths already renamed; Apply must not delete
// them afterwards, because on a case-insensitive volume that delete removes
// the restored file.
func caseRenames(root string, writes, removes []Item) (map[string]struct{}, error) {
	done := map[string]struct{}{}
	for _, rm := range removes {
		for _, wr := range writes {
			if rm.Path == wr.Path || !platform.Same(rm.Path, wr.Path) {
				continue
			}
			src, err := platform.SafeRel(root, rm.Path)
			if err != nil {
				return nil, err
			}
			dst, err := platform.SafeRel(root, wr.Path)
			if err != nil {
				return nil, err
			}
			tmp := src + ".oops-case-tmp"
			if err := os.Rename(platform.LongPath(src), platform.LongPath(tmp)); err != nil {
				return nil, err
			}
			if err := os.Rename(platform.LongPath(tmp), platform.LongPath(dst)); err != nil {
				return nil, err
			}
			done[rm.Path] = struct{}{}
		}
	}
	return done, nil
}

func writeOne(root string, st *store.Store, it Item) error {
	if it.Want == nil {
		return nil
	}
	full, err := platform.SafeRel(root, it.Path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(platform.LongPath(filepath.Dir(full)), 0o755); err != nil {
		return err
	}
	switch it.Want.Kind {
	case snapshot.KindDir:
		return os.MkdirAll(platform.LongPath(full), 0o755)
	case snapshot.KindSymlink:
		_ = os.Remove(platform.LongPath(full))
		if err := os.Symlink(it.Want.Target, platform.LongPath(full)); err != nil {
			return err
		}
		return nil
	default:
		if it.Want.Skipped != "" || it.Want.ID.IsZero() {
			return errors.New("missing blob for " + it.Path)
		}
		raw, err := st.Get(it.Want.ID)
		if err != nil {
			return err
		}
		mode := os.FileMode(it.Want.Mode)
		if mode == 0 {
			mode = 0o644
		}
		return platform.WriteFileAtomic(full, raw, mode)
	}
}

// BusyError means a file could not be replaced.
type BusyError struct {
	Path string
	Err  error
}

func (e *BusyError) Error() string { return e.Err.Error() }
func (e *BusyError) Unwrap() error { return e.Err }

// Journal is an in-progress restore.
type Journal struct {
	Status     string    `json:"status"`
	WS         string    `json:"ws"`
	SafetySnap string    `json:"safety_snap"`
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
	Message    string    `json:"message,omitempty"`
}

// WriteJournal stores a journal atomically.
func WriteJournal(path string, j Journal) error {
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return platform.WriteFileAtomic(path, b, 0o600)
}

// ReadJournal loads a journal. Missing is a zero value and os.ErrNotExist.
func ReadJournal(path string) (Journal, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Journal{}, err
	}
	var j Journal
	err = json.Unmarshal(b, &j)
	return j, err
}
