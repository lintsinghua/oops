package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/restore"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/store"
)

// UndoRequest selects what to reverse.
type UndoRequest struct {
	Workspace Workspace
	Seq       int
	Files     []string
	Force     bool
	DryRun    bool
	Running   bool
}

// Effect is the user-facing outcome for one path.
type Effect struct {
	Path   string `json:"path"`
	Op     string `json:"op"`
	Reason string `json:"reason,omitempty"`
	// Dir marks a directory. Displayed file counts skip these; the apply still runs them.
	Dir bool `json:"-"`
	// Before is the file on disk now. Want is what applying this effect would write.
	Before *snapshot.Meta `json:"-"`
	Want   *snapshot.Meta `json:"-"`
}

// UndoResult is the outcome of a mutating command.
type UndoResult struct {
	Turn    meta.Turn `json:"-"`
	Turns   []meta.Turn
	Result  string   `json:"result"`
	Effects []Effect `json:"effects"`
	DryRun  bool     `json:"dry_run"`
	Seqs    []int    `json:"seqs,omitempty"`
}

// Undo reverses one finished turn. With no turn number, a redo is undone
// first (the stack), and a still-running turn is refused unless Running is set.
func (e *Engine) Undo(ctx context.Context, req UndoRequest) (UndoResult, error) {
	if err := e.WaitSnapshot(ctx, req.Workspace); err != nil {
		return UndoResult{}, err
	}
	if req.Seq == 0 && len(req.Files) == 0 && !req.Running {
		running, err := e.DB.RunningTurns(req.Workspace.ID)
		if err != nil {
			return UndoResult{}, err
		}
		if len(running) > 0 {
			return UndoResult{}, &RunningError{Turn: running[len(running)-1]}
		}
	}
	if req.Seq == 0 && len(req.Files) == 0 {
		if op, err := e.DB.LatestStackOp(req.Workspace.ID); err == nil && !op.Applied {
			// A turn that finished after this redo is newer. Drop the redo
			// record so plain undo targets that turn.
			if e.redoSuperseded(req.Workspace, op) {
				if err := e.DB.DiscardUnapplied(req.Workspace.ID); err != nil {
					return UndoResult{}, err
				}
			} else {
				return e.reapply(ctx, req.Workspace, op, req.DryRun, req.Force)
			}
		}
	}
	t, err := e.turnForUndo(ctx, req)
	if err != nil {
		return UndoResult{}, err
	}
	pre, _, err := e.Files(t.PreSnap)
	if err != nil {
		return UndoResult{}, err
	}
	post, _, err := e.Files(t.PostSnap)
	if err != nil {
		return UndoResult{}, err
	}
	paths := changePaths(snapshot.Diff(pre, post))
	if len(req.Files) > 0 {
		matched, missing := filterPaths(paths, req.Files)
		if len(matched) == 0 {
			return UndoResult{Turn: t}, &PathError{Missing: missing, Seq: t.Seq}
		}
		paths = matched
	}
	curSnap, curRes, err := e.Capture(ctx, req.Workspace, SnapOptions{})
	if err != nil {
		return UndoResult{}, err
	}
	items, err := e.planUndoItems(req.Workspace, t, curRes.Files, paths, req.Force)
	if err != nil {
		return UndoResult{}, err
	}
	res := UndoResult{Turn: t, Result: resultOf(items), DryRun: req.DryRun, Seqs: []int{t.Seq}}
	res.Effects = effects(items)
	if req.DryRun {
		return res, nil
	}
	if err := e.DB.DiscardUnapplied(req.Workspace.ID); err != nil {
		return res, err
	}
	if err := e.apply(ctx, req.Workspace, "undo", []int{t.Seq}, curSnap.ID, items, true, len(req.Files) == 0 && !hasKept(items)); err != nil {
		return res, err
	}
	if len(req.Files) == 0 && !hasKept(items) {
		if err := e.DB.SetStatus(t.ID, "undone"); err != nil {
			return res, err
		}
		t.Status = "undone"
		t.Stats.Kept = nil
		_ = e.DB.UpdateTurn(t)
		res.Turn = t
	} else if hasKept(items) {
		t.Status = "kept"
		t.Stats.Kept = keptPaths(items)
		if err := e.DB.UpdateTurn(t); err != nil {
			return res, err
		}
		res.Turn = t
	}
	return res, nil
}

func (e *Engine) turnForUndo(ctx context.Context, req UndoRequest) (meta.Turn, error) {
	var t meta.Turn
	var err error
	if req.Seq == 0 {
		if req.Running {
			running, rerr := e.DB.RunningTurns(req.Workspace.ID)
			if rerr != nil {
				return t, rerr
			}
			if len(running) > 0 {
				t = running[len(running)-1]
			} else {
				t, err = e.DB.LatestUndoable(req.Workspace.ID)
			}
		} else {
			t, err = e.DB.LatestUndoable(req.Workspace.ID)
		}
	} else {
		t, err = e.DB.GetTurn(req.Workspace.ID, req.Seq)
	}
	if err != nil {
		return t, err
	}
	if t.Status == meta.StatusPending {
		t, err = e.completePendingTurn(ctx, req.Workspace, t)
		if err != nil {
			return t, err
		}
	}
	if t.Status == "running" {
		if !req.Running {
			return t, &RunningError{Turn: t}
		}
		t, err = e.finish(ctx, req.Workspace, t, true)
		if err != nil {
			return t, err
		}
	}
	if t.PreSnap == "" || t.PostSnap == "" {
		return t, os.ErrInvalid
	}
	return t, nil
}

// Rewind moves the workspace to just before seq.
func (e *Engine) Rewind(ctx context.Context, ws Workspace, seq int, force, dry bool) (UndoResult, error) {
	if err := e.WaitPending(ctx, ws); err != nil {
		return UndoResult{}, err
	}
	turns, err := e.turnsFrom(ws, seq)
	if err != nil {
		return UndoResult{}, err
	}
	if len(turns) == 0 {
		return UndoResult{}, os.ErrNotExist
	}
	pre, _, err := e.Files(turns[0].PreSnap)
	if err != nil {
		return UndoResult{}, err
	}
	expected := map[string]snapshot.Meta{}
	pathSet := map[string]struct{}{}
	for _, t := range turns {
		if t.Status == "running" || t.PostSnap == "" {
			continue
		}
		post, _, err := e.Files(t.PostSnap)
		if err != nil {
			return UndoResult{}, err
		}
		tpre, _, err := e.Files(t.PreSnap)
		if err != nil {
			return UndoResult{}, err
		}
		for _, c := range snapshot.Diff(tpre, post) {
			pathSet[c.Path] = struct{}{}
			if m, ok := post[c.Path]; ok {
				expected[c.Path] = m
			} else {
				delete(expected, c.Path)
			}
		}
	}
	var paths []string
	for p := range pathSet {
		paths = append(paths, p)
	}
	curSnap, curRes, err := e.Capture(ctx, ws, SnapOptions{})
	if err != nil {
		return UndoResult{}, err
	}
	own, _ := e.loadOwnWrites(ws)
	windows := snapWindows(turns)
	items := restore.Plan(paths, withBaseline(expected, own, earliestStart(windows)), pre, curRes.Files, force)
	if !force {
		preserveAfterStop(items, windows, expected, curRes.Files, own)
	}
	var seqs []int
	for _, t := range turns {
		seqs = append(seqs, t.Seq)
	}
	res := UndoResult{Turns: turns, Turn: turns[len(turns)-1], Result: resultOf(items), DryRun: dry, Seqs: seqs, Effects: effects(items)}
	if dry {
		return res, nil
	}
	if err := e.DB.DiscardUnapplied(ws.ID); err != nil {
		return res, err
	}
	if err := e.apply(ctx, ws, "rewind", seqs, curSnap.ID, items, true, true); err != nil {
		return res, err
	}
	for _, t := range turns {
		_ = e.DB.SetStatus(t.ID, "undone")
	}
	return res, nil
}

func (e *Engine) turnsFrom(ws Workspace, seq int) ([]meta.Turn, error) {
	all, err := e.DB.ListTurns(ws.ID, 0, false)
	if err != nil {
		return nil, err
	}
	var out []meta.Turn
	for i := len(all) - 1; i >= 0; i-- {
		t := all[i]
		if t.Seq >= seq && t.Kind != "safety" && t.Status != "running" {
			out = append(out, t)
		}
	}
	return out, nil
}

// RestoreFiles brings paths back from before seq.
func (e *Engine) RestoreFiles(ctx context.Context, ws Workspace, seq int, paths []string, force, dry bool) (UndoResult, error) {
	if err := e.WaitPending(ctx, ws); err != nil {
		return UndoResult{}, err
	}
	t, err := e.DB.GetTurn(ws.ID, seq)
	if err != nil {
		return UndoResult{}, err
	}
	pre, _, err := e.Files(t.PreSnap)
	if err != nil {
		return UndoResult{}, err
	}
	curSnap, curRes, err := e.Capture(ctx, ws, SnapOptions{})
	if err != nil {
		return UndoResult{}, err
	}
	var items []restore.Item
	for _, p := range paths {
		want, ok := pre[p]
		now, has := curRes.Files[p]
		var wantPtr, nowPtr *snapshot.Meta
		if ok {
			cp := want
			wantPtr = &cp
		}
		if has {
			cp := now
			nowPtr = &cp
		}
		it := restore.Item{Path: p, Before: nowPtr, Want: wantPtr, After: wantPtr}
		switch {
		case !ok:
			it.Op = "skip"
			it.Reason = "missing_in_snapshot"
		case snapshot.SameState(nowPtr, wantPtr):
			it.Op = "skip"
			it.Reason = "already"
		case has && !force:
			it.Op = "kept"
			it.Reason = "exists"
		default:
			it.Op = "restore"
			if force && has {
				it.Reason = "forced"
			}
		}
		items = append(items, it)
	}
	res := UndoResult{Turn: t, Result: resultOf(items), DryRun: dry, Effects: effects(items), Seqs: []int{seq}}
	if dry {
		return res, nil
	}
	if hasKept(items) {
		return res, nil
	}
	if err := e.DB.DiscardUnapplied(ws.ID); err != nil {
		return res, err
	}
	if err := e.apply(ctx, ws, "restore", []int{seq}, curSnap.ID, items, true, false); err != nil {
		return res, err
	}
	return res, nil
}

// Redo inverts the newest applied undo, rewind, or restore. The redo record
// itself is not applied, so a second redo pops the previous operation and
// `undo` with no turn number puts the redone operation back. dry reports the
// plan and writes nothing.
func (e *Engine) Redo(ctx context.Context, ws Workspace, dry bool) (UndoResult, error) {
	if err := e.WaitPending(ctx, ws); err != nil {
		return UndoResult{}, err
	}
	op, err := e.DB.LatestAppliedStackOp(ws.ID)
	if err != nil {
		return UndoResult{}, err
	}
	var detail opDetail
	if err := json.Unmarshal([]byte(op.Detail), &detail); err != nil {
		return UndoResult{}, err
	}
	curSnap, curRes, err := e.Capture(ctx, ws, SnapOptions{})
	if err != nil {
		return UndoResult{}, err
	}
	items := planRedoItems(detail.Items, curRes.Files)
	res := UndoResult{Result: resultOf(items), Effects: effects(items), Seqs: op.Targets, DryRun: dry}
	e.attachTurn(ws, &res)
	if dry || !hasWritable(items) {
		return res, nil
	}
	if err := e.apply(ctx, ws, "redo", op.Targets, curSnap.ID, items, false, false); err != nil {
		return res, err
	}
	if err := e.DB.MoveToRedoStack(op); err != nil {
		return res, err
	}
	for _, seq := range op.Targets {
		if t, err := e.DB.GetTurn(ws.ID, seq); err == nil && op.Type != "restore" {
			t.Status = "done"
			t.Stats.Kept = nil
			_ = e.DB.UpdateTurn(t)
		}
	}
	return res, nil
}

// planUndoItems is the plan Undo applies. paths limits the plan; nil plans every change in the turn.
func (e *Engine) planUndoItems(ws Workspace, t meta.Turn, cur map[string]snapshot.Meta, paths []string, force bool) ([]restore.Item, error) {
	pre, _, err := e.Files(t.PreSnap)
	if err != nil {
		return nil, err
	}
	post, _, err := e.Files(t.PostSnap)
	if err != nil {
		return nil, err
	}
	if paths == nil {
		paths = changePaths(snapshot.Diff(pre, post))
	}
	own, _ := e.loadOwnWrites(ws)
	windows := []snapWindow{{Start: t.EndedAt, Done: t.Stats.SnapDone}}
	items := restore.Plan(paths, withBaseline(post, own, t.EndedAt), pre, cur, force)
	if !force {
		preserveAfterStop(items, windows, post, cur, own)
	}
	return items, nil
}

// planRedoItems is the plan Redo applies to a recorded undo.
// A path kept as the user's file stays kept for the rest of this chain.
func planRedoItems(recorded []restore.Item, cur map[string]snapshot.Meta) []restore.Item {
	var items []restore.Item
	for _, ef := range recorded {
		now := lookup(cur, ef.Path)
		if ef.Op == "kept" || ef.Op == "unprotected" {
			items = append(items, restore.Item{Path: ef.Path, Op: "kept", Reason: "changed_after_turn", Before: now, Want: ef.Before})
			continue
		}
		if !snapshot.SameState(now, ef.After) && !snapshot.SameState(now, ef.Want) {
			items = append(items, restore.Item{Path: ef.Path, Op: "kept", Reason: "changed_after_turn", Before: now, Want: ef.Before})
			continue
		}
		opName := "revert"
		if ef.Before == nil {
			opName = "remove"
		} else if now == nil {
			opName = "restore"
		}
		items = append(items, restore.Item{Path: ef.Path, Op: opName, Before: now, Want: ef.Before, After: ef.Before})
	}
	return items
}

// ActionPlan is what the timeline would do to this turn. A finished turn uses
// the undo plan. An undone or partial turn uses the redo plan. Both are the
// planners Undo and Redo apply.
func (e *Engine) ActionPlan(ctx context.Context, ws Workspace, t meta.Turn, cur map[string]snapshot.Meta) (UndoResult, bool, error) {
	if cur == nil {
		_, res, err := e.Capture(ctx, ws, SnapOptions{})
		if err != nil {
			return UndoResult{}, false, err
		}
		cur = res.Files
	}
	if t.Status == "undone" || t.Status == "kept" {
		op, ok, err := e.latestAppliedOp(ws.ID, t.Seq)
		if err != nil {
			return UndoResult{}, true, err
		}
		if !ok {
			return UndoResult{}, true, os.ErrNotExist
		}
		var detail opDetail
		if err := json.Unmarshal([]byte(op.Detail), &detail); err != nil {
			return UndoResult{}, true, err
		}
		items := planRedoItems(detail.Items, cur)
		return UndoResult{Turn: t, Effects: effects(items), Result: resultOf(items), Seqs: op.Targets}, true, nil
	}
	items, err := e.planUndoItems(ws, t, cur, nil, false)
	if err != nil {
		return UndoResult{}, false, err
	}
	return UndoResult{Turn: t, Effects: effects(items), Result: resultOf(items), Seqs: []int{t.Seq}}, false, nil
}

func (e *Engine) latestAppliedOp(ws string, seq int) (meta.Op, bool, error) {
	ops, err := e.DB.ListOps(ws, 0)
	if err != nil {
		return meta.Op{}, false, err
	}
	for _, op := range ops {
		if !op.Applied || op.Status == "discarded" {
			continue
		}
		if op.Type != "undo" && op.Type != "rewind" && op.Type != "restore" {
			continue
		}
		for _, target := range op.Targets {
			if target == seq {
				return op, true, nil
			}
		}
	}
	return meta.Op{}, false, nil
}

func lookup(m map[string]snapshot.Meta, path string) *snapshot.Meta {
	if v, ok := m[path]; ok {
		cp := v
		return &cp
	}
	return nil
}

type opDetail struct {
	Items []restore.Item `json:"items"`
}

// reapply puts an undone stack operation back into effect (undo of a redo).
func (e *Engine) reapply(ctx context.Context, ws Workspace, op meta.Op, dry, force bool) (UndoResult, error) {
	var detail opDetail
	if err := json.Unmarshal([]byte(op.Detail), &detail); err != nil {
		return UndoResult{}, err
	}
	curSnap, curRes, err := e.Capture(ctx, ws, SnapOptions{})
	if err != nil {
		return UndoResult{}, err
	}
	var items []restore.Item
	for _, ef := range detail.Items {
		if ef.Op == "skip" {
			continue
		}
		now := lookup(curRes.Files, ef.Path)
		// A file kept on the way down stays the user's unless --force.
		// Matching the recorded "before" bytes is that user version, not a
		// license to write the undo target over it.
		if (ef.Op == "kept" || ef.Op == "unprotected") && !force {
			if snapshot.SameState(now, ef.Want) {
				items = append(items, restore.Item{Path: ef.Path, Op: "skip", Reason: "already", Before: now, Want: ef.Want, After: ef.Want})
			} else {
				items = append(items, restore.Item{Path: ef.Path, Op: "kept", Reason: "changed_after_turn", Before: now, Want: ef.Want, After: now})
			}
			continue
		}
		switch {
		case snapshot.SameState(now, ef.Want) || snapshot.SameState(now, ef.After):
			items = append(items, restore.Item{Path: ef.Path, Op: "skip", Reason: "already", Before: now, Want: ef.Want, After: ef.Want})
		case !snapshot.SameState(now, ef.Before):
			items = append(items, restore.Item{Path: ef.Path, Op: "kept", Reason: "changed_after_turn", Before: now, Want: ef.Want})
		default:
			opName := "revert"
			if ef.Want == nil {
				opName = "remove"
			} else if now == nil {
				opName = "restore"
			}
			items = append(items, restore.Item{Path: ef.Path, Op: opName, Before: now, Want: ef.Want, After: ef.Want})
		}
	}
	res := UndoResult{Result: resultOf(items), Effects: effects(items), Seqs: op.Targets, DryRun: dry}
	e.attachTurn(ws, &res)
	if dry || !hasWritable(items) {
		return res, nil
	}
	if err := e.restoreItems(ws, restore.Journal{
		SafetySnap: curSnap.ID, Kind: op.Type, Seqs: op.Targets, Applied: true, MarkUndone: op.Type != "restore",
	}, items); err != nil {
		return res, err
	}
	if err := e.DB.SetOpApplied(op.ID, true); err != nil {
		return res, err
	}
	if op.Type != "restore" {
		for _, seq := range op.Targets {
			t, err := e.DB.GetTurn(ws.ID, seq)
			if err != nil {
				continue
			}
			if hasKept(items) {
				t.Status = "kept"
				t.Stats.Kept = keptPaths(items)
			} else {
				t.Status = "undone"
				t.Stats.Kept = nil
			}
			_ = e.DB.UpdateTurn(t)
		}
	}
	if err := e.commitJournal(ws); err != nil {
		return res, err
	}
	return res, nil
}

func (e *Engine) apply(ctx context.Context, ws Workspace, kind string, seqs []int, safety string, items []restore.Item, applied, markUndone bool) error {
	if err := e.restoreItems(ws, restore.Journal{
		SafetySnap: safety, Kind: kind, Seqs: seqs, Applied: applied, MarkUndone: markUndone,
	}, items); err != nil {
		return err
	}
	_ = ctx
	if err := e.recordJournalOp(restore.Journal{
		WS: ws.ID, SafetySnap: safety, Kind: kind, Seqs: seqs, Applied: applied, MarkUndone: markUndone,
		Items: items, CreatedAt: e.Now,
	}); err != nil {
		return err
	}
	return e.commitJournal(ws)
}

func (e *Engine) restoreItems(ws Workspace, j restore.Journal, items []restore.Item) error {
	jpath := filepath.Join(e.wsDir(ws.ID), "journal", "current.json")
	if err := os.MkdirAll(filepath.Dir(jpath), 0o700); err != nil {
		return err
	}
	j.Status = "pending"
	j.WS = ws.ID
	j.Items = items
	j.CreatedAt = e.Now
	if err := restore.WriteJournal(jpath, j); err != nil {
		return err
	}
	if err := restore.Apply(ws.Path, e.Store, items); err != nil {
		return err
	}
	return e.noteOwnWrites(ws, items)
}

func (e *Engine) commitJournal(ws Workspace) error {
	jpath := filepath.Join(e.wsDir(ws.ID), "journal", "current.json")
	j, err := restore.ReadJournal(jpath)
	if err != nil {
		return err
	}
	j.Status = "committed"
	return restore.WriteJournal(jpath, j)
}

// ResumeJournal finishes or rolls back an interrupted restore.
func (e *Engine) ResumeJournal(ctx context.Context, ws Workspace) (string, error) {
	jpath := filepath.Join(e.wsDir(ws.ID), "journal", "current.json")
	j, err := restore.ReadJournal(jpath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if j.Status == "committed" || j.Status == "" {
		return "", nil
	}
	if err := restore.Apply(ws.Path, e.Store, j.Items); err != nil {
		// Roll back to the safety snapshot for the paths we know.
		if j.SafetySnap != "" {
			files, _, ferr := e.Files(j.SafetySnap)
			if ferr == nil {
				var back []restore.Item
				for _, it := range j.Items {
					if m, ok := files[it.Path]; ok {
						cp := m
						back = append(back, restore.Item{Path: it.Path, Op: "revert", Want: &cp})
					}
				}
				_ = restore.Apply(ws.Path, e.Store, back)
				_ = e.noteOwnWrites(ws, back)
			}
		}
		_ = os.Remove(jpath)
		return "rolled-back", err
	}
	_ = e.noteOwnWrites(ws, j.Items)
	if err := e.recordJournalOp(j); err != nil {
		return "finished", err
	}
	j.Status = "committed"
	if err := restore.WriteJournal(jpath, j); err != nil {
		return "finished", err
	}
	_ = ctx
	return "finished", nil
}

// recordJournalOp stores the undo operation once. A resume after a crash calls
// it again; an operation for the same safety snapshot is left as it is.
func (e *Engine) recordJournalOp(j restore.Journal) error {
	if j.Kind == "" || j.SafetySnap == "" {
		return nil
	}
	ops, err := e.DB.ListOps(j.WS, 0)
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.SafetySnap == j.SafetySnap && op.Type == j.Kind && op.Status != "discarded" {
			return nil
		}
	}
	detail, err := json.Marshal(opDetail{Items: j.Items})
	if err != nil {
		return err
	}
	if _, err := e.DB.InsertOp(meta.Op{
		WS: j.WS, Type: j.Kind, Targets: j.Seqs, SafetySnap: j.SafetySnap,
		CreatedAt: j.CreatedAt, Status: "committed", Detail: string(detail), Applied: j.Applied,
	}); err != nil {
		return err
	}
	if !j.MarkUndone {
		return nil
	}
	for _, seq := range j.Seqs {
		t, err := e.DB.GetTurn(j.WS, seq)
		if err != nil {
			continue
		}
		if err := e.DB.SetStatus(t.ID, "undone"); err != nil {
			return err
		}
	}
	return nil
}

// snapWindow is the time the post snapshot was being written.
// Own writes are matched against the earliest start, so an undo or redo
// after Stop replaces the post snapshot as the baseline.
type snapWindow struct {
	Start time.Time
	Done  time.Time
}

func snapWindows(turns []meta.Turn) []snapWindow {
	var out []snapWindow
	for _, t := range turns {
		if t.EndedAt.IsZero() || t.Stats.SnapDone.IsZero() {
			continue
		}
		out = append(out, snapWindow{Start: t.EndedAt, Done: t.Stats.SnapDone})
	}
	return out
}

// preserveAfterStop keeps a user edit. An undo or redo baseline wins:
// matching those bytes is not a user edit. Otherwise a file is a user edit
// only when its content differs from the post snapshot. An mtime, size, or
// ctime inside the snapshot window is not enough. On NTFS those fields can
// still be settling, or can disagree in precision with the snapshot, while
// the bytes match. --force skips this pass.
func preserveAfterStop(items []restore.Item, windows []snapWindow, expected, cur map[string]snapshot.Meta, own map[string]ownWrite) {
	for i := range items {
		it := &items[i]
		switch it.Op {
		case "skip", "kept", "unprotected":
			continue
		}
		if planIsDir(*it) {
			if it.Op == "remove" && dirHasUserEdit(it.Path, windows, expected, cur, own) {
				it.Op = "kept"
				it.Reason = "changed_after_turn"
			}
			continue
		}
		if userEdited(it.Path, it.Before, windows, expected, own) {
			it.Op = "kept"
			it.Reason = "changed_after_turn"
		}
	}
}

// userEdited reports whether path's current bytes differ from the baseline.
// An undo or redo that ran after Stop is the baseline. Otherwise the baseline
// is the post snapshot. Metadata that disagrees with the snapshot is not a
// user edit when the content hash matches.
func userEdited(path string, now *snapshot.Meta, windows []snapWindow, expected map[string]snapshot.Meta, own map[string]ownWrite) bool {
	if rec, ok := freshWrite(own, path, earliestStart(windows)); ok {
		return !baselineMatch(now, rec)
	}
	return !snapshot.SameState(now, lookupMeta(expected, path))
}

func baselineMatch(now *snapshot.Meta, rec ownWrite) bool {
	if now == nil {
		return rec.Gone
	}
	if rec.Gone {
		return false
	}
	return rec.Hash != "" && now.Kind == snapshot.KindFile && now.ID.String() == rec.Hash
}

func lookupMeta(m map[string]snapshot.Meta, path string) *snapshot.Meta {
	if v, ok := m[path]; ok {
		cp := v
		return &cp
	}
	return nil
}

func dirHasUserEdit(rel string, windows []snapWindow, expected, cur map[string]snapshot.Meta, own map[string]ownWrite) bool {
	prefix := strings.TrimSuffix(rel, "/") + "/"
	seen := map[string]struct{}{}
	for path, now := range cur {
		if path != strings.TrimSuffix(rel, "/") && !strings.HasPrefix(path, prefix) {
			continue
		}
		if path == strings.TrimSuffix(rel, "/") {
			continue
		}
		seen[path] = struct{}{}
		cp := now
		if userEdited(path, &cp, windows, expected, own) {
			return true
		}
	}
	base := strings.TrimSuffix(rel, "/")
	for path := range expected {
		if _, ok := seen[path]; ok || path == base {
			continue
		}
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		if userEdited(path, nil, windows, expected, own) {
			return true
		}
	}
	return false
}

// withBaseline overlays bytes oops wrote on the post snapshot. Later undos
// compare against that write, not the original post snapshot.
func earliestStart(windows []snapWindow) time.Time {
	var start time.Time
	for _, w := range windows {
		if w.Start.IsZero() {
			continue
		}
		if start.IsZero() || w.Start.Before(start) {
			start = w.Start
		}
	}
	return start
}

// freshWrite is an undo or redo that happened after this turn's Stop.
// An earlier write belongs to a previous turn; the post snapshot is the baseline.
func freshWrite(own map[string]ownWrite, path string, stop time.Time) (ownWrite, bool) {
	rec, ok := own[path]
	if !ok {
		return ownWrite{}, false
	}
	if !stop.IsZero() && rec.At != 0 && rec.At < stop.UnixNano() {
		return ownWrite{}, false
	}
	return rec, true
}

func withBaseline(expected map[string]snapshot.Meta, own map[string]ownWrite, stop time.Time) map[string]snapshot.Meta {
	if len(own) == 0 {
		return expected
	}
	out := make(map[string]snapshot.Meta, len(expected)+len(own))
	for k, v := range expected {
		out[k] = v
	}
	for path, rec := range own {
		if _, ok := freshWrite(own, path, stop); !ok {
			continue
		}
		if rec.Gone {
			delete(out, path)
			continue
		}
		if rec.Hash == "" {
			continue
		}
		id, err := store.ParseID(rec.Hash)
		if err != nil {
			continue
		}
		m := out[path]
		m.Path = path
		m.Kind = snapshot.KindFile
		m.ID = id
		m.Mtime = rec.Mtime
		if m.Mode == 0 {
			m.Mode = 0o644
		}
		out[path] = m
	}
	return out
}

func keptPaths(items []restore.Item) []string {
	var out []string
	for _, it := range items {
		if it.Op != "kept" && it.Op != "unprotected" {
			continue
		}
		if planIsDir(it) {
			continue
		}
		out = append(out, it.Path)
	}
	return out
}

type ownWrite struct {
	Mtime int64  `json:"mtime,omitempty"`
	At    int64  `json:"at,omitempty"`
	Hash  string `json:"hash,omitempty"`
	Gone  bool   `json:"gone,omitempty"`
}

func (e *Engine) ownWritePath(ws Workspace) string {
	return filepath.Join(e.wsDir(ws.ID), "own-writes.json")
}

func (e *Engine) loadOwnWrites(ws Workspace) (map[string]ownWrite, error) {
	raw, err := os.ReadFile(e.ownWritePath(ws))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]ownWrite{}, nil
		}
		return nil, err
	}
	out := map[string]ownWrite{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]ownWrite{}, nil
	}
	return out, nil
}

func (e *Engine) noteOwnWrites(ws Workspace, items []restore.Item) error {
	own, err := e.loadOwnWrites(ws)
	if err != nil {
		return err
	}
	if own == nil {
		own = map[string]ownWrite{}
	}
	for _, it := range items {
		switch it.Op {
		case "restore", "revert", "remove":
		default:
			continue
		}
		full, err := platform.SafeRel(ws.Path, strings.TrimSuffix(it.Path, "/"))
		if err != nil {
			continue
		}
		st, err := os.Lstat(platform.LongPath(full))
		if err != nil {
			own[it.Path] = ownWrite{Gone: true, At: time.Now().UnixNano()}
			continue
		}
		rec := ownWrite{Mtime: st.ModTime().UnixNano(), At: time.Now().UnixNano()}
		if it.Want != nil && it.Want.Kind == snapshot.KindFile && !it.Want.ID.IsZero() {
			rec.Hash = it.Want.ID.String()
		}
		own[it.Path] = rec
	}
	if err := os.MkdirAll(filepath.Dir(e.ownWritePath(ws)), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(own)
	if err != nil {
		return err
	}
	return os.WriteFile(e.ownWritePath(ws), raw, 0o600)
}

func planIsDir(it restore.Item) bool {
	return metaIsDir(it.Want) || metaIsDir(it.Before) || strings.HasSuffix(it.Path, "/")
}

func changePaths(changes []snapshot.Change) []string {
	var out []string
	for _, c := range changes {
		out = append(out, c.Path)
	}
	return out
}

func effects(items []restore.Item) []Effect {
	var out []Effect
	for _, it := range items {
		if it.Op == "skip" {
			continue
		}
		out = append(out, Effect{Path: it.Path, Op: it.Op, Reason: it.Reason, Dir: effectIsDir(it), Before: it.Before, Want: it.Want})
	}
	return out
}

func effectIsDir(it restore.Item) bool {
	return metaIsDir(it.Want) || metaIsDir(it.Before) || metaIsDir(it.After)
}

func metaIsDir(m *snapshot.Meta) bool {
	return m != nil && m.Kind == snapshot.KindDir
}

// CountFileEffects counts effects that change a file. Directories are not included.
func CountFileEffects(effects []Effect) int {
	n := 0
	for _, ef := range effects {
		if ef.Dir || ef.Op == "skip" || ef.Op == "kept" || ef.Op == "unprotected" {
			continue
		}
		n++
	}
	return n
}

func resultOf(items []restore.Item) string {
	kept := false
	changed := false
	for _, it := range items {
		switch it.Op {
		case "kept", "unprotected":
			kept = true
		case "restore", "remove", "revert":
			changed = true
		}
	}
	switch {
	case kept && changed:
		return "partial"
	case kept && !changed:
		return "partial"
	default:
		return "ok"
	}
}

func hasKept(items []restore.Item) bool {
	for _, it := range items {
		if it.Op == "kept" || it.Op == "unprotected" {
			return true
		}
	}
	return false
}

func hasWritable(items []restore.Item) bool {
	for _, it := range items {
		switch it.Op {
		case "restore", "remove", "revert":
			return true
		}
	}
	return false
}

// redoSuperseded is true when a finished turn is newer than every turn this
// redo would put back. Plain undo must target that turn.
func (e *Engine) redoSuperseded(ws Workspace, op meta.Op) bool {
	latest, err := e.DB.LatestUndoable(ws.ID)
	if err != nil {
		return false
	}
	max := 0
	for _, seq := range op.Targets {
		if seq > max {
			max = seq
		}
	}
	return latest.Seq > max
}

func (e *Engine) attachTurn(ws Workspace, res *UndoResult) {
	if res == nil || len(res.Seqs) == 0 {
		return
	}
	seq := res.Seqs[len(res.Seqs)-1]
	if t, err := e.DB.GetTurn(ws.ID, seq); err == nil {
		res.Turn = t
		return
	}
	res.Turn = meta.Turn{Seq: seq}
}

// RunningError means the turn is still open.
type RunningError struct{ Turn meta.Turn }

func (r *RunningError) Error() string { return "turn is running" }

// PathError means requested paths were not in the turn.
type PathError struct {
	Missing []string
	Seq     int
}

func (p *PathError) Error() string { return "path is not part of the turn" }

// ManualSnap records a checkpoint where pre and post are the current tree.
func (e *Engine) ManualSnap(ctx context.Context, ws Workspace, message string) (meta.Turn, error) {
	if err := e.WaitPending(ctx, ws); err != nil {
		return meta.Turn{}, err
	}
	snap, _, err := e.Capture(ctx, ws, SnapOptions{})
	if err != nil {
		return meta.Turn{}, err
	}
	t := meta.Turn{
		WS: ws.ID, Agent: "you", Kind: "manual", Prompt: SummarizePrompt(message, true, 200),
		StartedAt: e.Now, EndedAt: e.Now, Status: "done", PreSnap: snap.ID, PostSnap: snap.ID,
	}
	inserted, err := e.DB.InsertTurn(t)
	if err != nil {
		return inserted, err
	}
	if err := e.DB.DiscardUnapplied(ws.ID); err != nil {
		return inserted, err
	}
	return inserted, nil
}

// ChangesBetween loads the diff for a turn.
func (e *Engine) ChangesBetween(t meta.Turn) ([]snapshot.Change, error) {
	pre, _, err := e.Files(t.PreSnap)
	if err != nil {
		return nil, err
	}
	post, _, err := e.Files(t.PostSnap)
	if err != nil {
		return nil, err
	}
	ch := snapshot.Diff(pre, post)
	return snapshot.AnnotateLines(ch, e.ReadBlob), nil
}

// GapFiles diffs post of older against pre of newer.
func (e *Engine) GapFiles(older, newer meta.Turn) ([]snapshot.Change, error) {
	if older.PostSnap == "" || newer.PreSnap == "" || older.PostSnap == newer.PreSnap {
		return nil, nil
	}
	post, _, err := e.Files(older.PostSnap)
	if err != nil {
		return nil, err
	}
	pre, _, err := e.Files(newer.PreSnap)
	if err != nil {
		return nil, err
	}
	if postRootEqual(post, pre) {
		return nil, nil
	}
	return snapshot.Diff(post, pre), nil
}

func postRootEqual(a, b map[string]snapshot.Meta) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !snapshot.SameState(&av, &bv) {
			return false
		}
	}
	return true
}
