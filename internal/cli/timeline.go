package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/lintsinghua/oops/internal/agents"
	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/tui"
	"github.com/lintsinghua/oops/internal/ui"
)

func cmdTimeline(cmd *cobra.Command, opt *options) error {
	if cmd.Flags().NArg() > 0 {
		return fail(2, "oops doesn't take arguments. Try oops --help.")
	}
	s, err := begin(opt)
	if err != nil {
		return err
	}
	defer s.e.Close()
	if !wantTUI(opt) {
		return printStatus(opt, s)
	}
	rows, err := timelineRows(s)
	if err != nil {
		return err
	}
	header := platform.DisplayPath(s.ws.Path, s.e.UserHome)
	if names := connectedShort(s); len(names) > 0 {
		header += "  " + strings.Join(names, " · ")
	}
	model := tui.New(rows, header, "", opt.colorOn(), ui.ASCIIMode())
	model.Apply = func(a tui.Action) ([]tui.Row, string, error) {
		return applyTimeline(s, a)
	}
	// Alt screen so quit leaves the terminal as it was. Undo and redo refresh in place.
	prog := bubbletea.NewProgram(model, bubbletea.WithInput(opt.stdin), bubbletea.WithOutput(opt.stdout), bubbletea.WithMouseCellMotion(), bubbletea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		return err
	}
	done, ok := final.(tui.Model)
	if !ok || done.Action().Kind == "" {
		return nil
	}
	action := done.Action()
	switch action.Kind {
	case "undo":
		return cmdUndo(opt, action.Seq, nil, false, false, true, false)
	case "rewind":
		return cmdRewind(opt, action.Seq, false, false, true)
	case "redo":
		return newRedoCmd(opt).RunE(cmd, nil)
	default:
		return nil
	}
}

func wantTUI(opt *options) bool {
	if opt.json || opt.accessible || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTTY(opt.stdin) && isTTY(opt.stdout)
}

func printStatus(opt *options, s *session) error {
	turns, err := s.e.DB.ListTurns(s.ws.ID, 5, false)
	if err != nil {
		return err
	}
	if opt.json {
		return writeListJSON(opt.stdout, s.ws, turns)
	}
	if len(turns) == 0 {
		fmt.Fprintln(opt.stdout, "No snapshots here yet.")
		fmt.Fprintln(opt.stdout)
		fmt.Fprintln(opt.stdout, "  oops protects this folder once an agent runs in it.")
		fmt.Fprintln(opt.stdout, "  Not set up?  oops init        Any other tool?  oops run -- <command>")
		return nil
	}
	names := connectedShort(s)
	protected := "not connected"
	if len(names) > 0 {
		protected = "protected (" + strings.Join(names, ", ") + ")"
	}
	fmt.Fprintf(opt.stdout, "%s · %s\n", platform.DisplayPath(s.ws.Path, s.e.UserHome), protected)
	now := time.Now()
	for _, t := range turns {
		if t.Kind == "safety" {
			continue
		}
		fmt.Fprintf(opt.stdout, "#%d  %s  %s   %s   %s\n",
			t.Seq, ui.Pad(t.Agent, 7), ui.Pad(ui.RelTime(now, t.StartedAt), 12), ui.Quote(t.Prompt, 40), ui.Plural(t.Stats.Files, "file", "files"))
	}
	return nil
}

func connectedShort(s *session) []string {
	user := s.e.UserHome
	var names []string
	for _, a := range agents.Agents() {
		doc, _, err := agents.ReadJSON(a.Config(user, s.ws.Path, false))
		if err == nil && agents.Connected(doc) {
			names = append(names, shortAgent(a.Name))
		}
		doc, _, err = agents.ReadJSON(a.Config(user, s.ws.Path, true))
		if err == nil && agents.Connected(doc) {
			short := shortAgent(a.Name)
			seen := false
			for _, n := range names {
				if n == short {
					seen = true
				}
			}
			if !seen {
				names = append(names, short)
			}
		}
	}
	return names
}

func timelineRows(s *session) ([]tui.Row, error) {
	turns, err := s.e.DB.ListTurns(s.ws.ID, 50, false)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var cur map[string]snapshot.Meta
	if _, res, err := s.e.Capture(context.Background(), s.ws, engine.SnapOptions{}); err == nil {
		cur = res.Files
	}
	var rows []tui.Row
	for i, t := range turns {
		if t.Kind == "safety" {
			continue
		}
		row := tui.Row{
			Seq: t.Seq, Agent: t.Agent, When: ui.RelTime(now, t.StartedAt),
			Prompt: t.Prompt, Files: t.Stats.Files, Status: t.Status,
			Insertions: t.Stats.Insertions, Deletions: t.Stats.Deletions,
		}
		changes, err := s.e.ChangesBetween(t)
		if err == nil {
			if cur != nil {
				if plan, redo, perr := s.e.ActionPlan(context.Background(), s.ws, t, cur); perr == nil {
					row.Preview, row.Diffs = previewFromPlan(s.e, t, changes, plan.Effects, redo)
					row.Plan = planSummary(plan.Effects, redo)
				}
			}
			if row.Preview == "" {
				row.Preview, row.Diffs = previewTurn(s.e, t, changes)
			}
			if len(row.Diffs) > 0 {
				row.Diff = row.Diffs[0]
			}
		}
		rows = append(rows, row)
		if i+1 < len(turns) {
			gap, err := s.e.GapFiles(turns[i+1], turns[i])
			if n := snapshot.CountFileChanges(gap); err == nil && n > 0 {
				rows = append(rows, tui.Row{
					Gap: true, Agent: "you", When: ui.RelTime(now, turns[i].StartedAt),
					Prompt: ui.Plural(n, "file changed outside agents", "files changed outside agents"),
				})
			}
		}
	}
	return rows, nil
}

func previewTurn(e *engine.Engine, t meta.Turn, changes []snapshot.Change) (string, []string) {
	var effects []engine.Effect
	var diffs []string
	partial := t.Status == "kept" || len(t.Stats.Kept) > 0
	redoView := partial || t.Status == "undone"
	keptSet := map[string]bool{}
	for _, p := range t.Stats.Kept {
		keptSet[p] = true
	}
	for _, c := range snapshot.FileChanges(changes) {
		op := c.UndoAction
		if redoView {
			op = reapplyOp(c.Action)
		}
		if keptSet[c.Path] {
			op = "keep"
		}
		effects = append(effects, engine.Effect{Path: c.Path, Op: op})
		oldB, newB := []byte(nil), []byte(nil)
		if e != nil {
			oldB, newB = e.ReadBlob(c.Post), e.ReadBlob(c.Pre)
		}
		text, err := snapshot.Unified("now/"+c.Path, "undo/"+c.Path, string(oldB), string(newB), 3)
		if err != nil || text == "" {
			text = op + " " + c.Path
		}
		if len(diffs) < 20 {
			diffs = append(diffs, fmt.Sprintf(" #%d · %s %s\n%s", t.Seq, op, c.Path, text))
		}
	}
	var b strings.Builder
	if partial {
		fmt.Fprintf(&b, "  #%d is undone (partial).\n", t.Seq)
		b.WriteString(keptPreview(t.Stats.Kept))
		fmt.Fprintf(&b, "  Redo #%d would re-apply:\n\n", t.Seq)
	} else if redoView {
		fmt.Fprintf(&b, "  #%d is already undone.\n  Redo #%d would re-apply:\n\n", t.Seq, t.Seq)
	} else {
		fmt.Fprintf(&b, "  Undo #%d would:\n\n", t.Seq)
	}
	lines := previewEffectLines(effects)
	if len(lines) == 0 {
		b.WriteString("    nothing to change\n")
	}
	for _, line := range lines {
		b.WriteString("    " + line + "\n")
	}
	if t.Prompt != "" {
		b.WriteString("\n  ran: " + t.Prompt + "\n")
	}
	return b.String(), diffs
}

// previewFromPlan renders the action Undo or Redo would take. effects come from
// the same planner those commands apply.
func previewFromPlan(e *engine.Engine, t meta.Turn, changes []snapshot.Change, effects []engine.Effect, redo bool) (string, []string) {
	shown := mapPreviewEffects(effects, redo)
	diffs := planDiffs(e, t, changes, effects, redo)
	var b strings.Builder
	switch {
	case t.Status == "kept":
		fmt.Fprintf(&b, "  #%d is undone (partial).\n  Redo #%d would re-apply:\n\n", t.Seq, t.Seq)
	case redo:
		fmt.Fprintf(&b, "  #%d is already undone.\n  Redo #%d would re-apply:\n\n", t.Seq, t.Seq)
	default:
		fmt.Fprintf(&b, "  Undo #%d would:\n\n", t.Seq)
	}
	lines := previewEffectLines(shown)
	if len(lines) == 0 {
		b.WriteString("    nothing to change\n")
	}
	for _, line := range lines {
		b.WriteString("    " + line + "\n")
	}
	if t.Prompt != "" {
		b.WriteString("\n  ran: " + t.Prompt + "\n")
	}
	return b.String(), diffs
}

// planDiffs is current disk versus the bytes applying the plan would write.
// A kept file shows that same comparison and says it will not change.
func planDiffs(e *engine.Engine, t meta.Turn, changes []snapshot.Change, effects []engine.Effect, redo bool) []string {
	post := map[string]*snapshot.Meta{}
	for _, c := range changes {
		if c.Post != nil {
			post[c.Path] = c.Post
		}
	}
	var diffs []string
	for _, ef := range effects {
		if ef.Dir {
			continue
		}
		nowB := e.ReadBlob(ef.Before)
		want := ef.Want
		if redo && (ef.Op == "kept" || ef.Op == "unprotected") {
			if p := post[ef.Path]; p != nil {
				want = p
			}
		}
		wantB := e.ReadBlob(want)
		label := previewVerb(ef.Op, redo)
		body, err := snapshot.Unified("now/"+ef.Path, "after/"+ef.Path, string(nowB), string(wantB), 3)
		if err != nil {
			body = ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, " #%d · %s %s\n", t.Seq, label, ef.Path)
		if ef.Op == "kept" || ef.Op == "unprotected" {
			b.WriteString("kept — will not change\n")
		}
		if body == "" {
			b.WriteString(label + " " + ef.Path + "\n")
		} else {
			b.WriteString(body)
		}
		if len(diffs) < 20 {
			diffs = append(diffs, b.String())
		}
	}
	return diffs
}

func mapPreviewEffects(effects []engine.Effect, redo bool) []engine.Effect {
	out := make([]engine.Effect, 0, len(effects))
	for _, ef := range effects {
		ef.Op = previewVerb(ef.Op, redo)
		out = append(out, ef)
	}
	return out
}

func previewVerb(op string, redo bool) string {
	switch op {
	case "kept", "unprotected":
		return "keep"
	}
	if !redo {
		return op
	}
	switch op {
	case "restore":
		return "add"
	case "revert":
		return "modify"
	default:
		return op
	}
}

// planSummary is the one-line form of the preview counts.
func planSummary(effects []engine.Effect, redo bool) string {
	var parts []string
	for _, line := range previewEffectLines(mapPreviewEffects(effects, redo)) {
		if strings.HasPrefix(line, "  ") {
			continue
		}
		parts = append(parts, strings.Join(strings.Fields(line), " "))
	}
	if len(parts) == 0 {
		return "nothing to change"
	}
	return strings.Join(parts, ", ")
}

// reapplyOp is what redo does to put a turn's edit back.
func reapplyOp(action string) string {
	switch action {
	case "added":
		return "add"
	case "deleted":
		return "remove"
	default:
		return "modify"
	}
}

func undoNotice(res engine.UndoResult) string {
	kept := keptEffectPaths(res.Effects)
	if len(kept) == 0 {
		return fmt.Sprintf("Undid #%d", res.Turn.Seq)
	}
	return fmt.Sprintf("Undid #%d, except %s", res.Turn.Seq, strings.Join(kept, ", "))
}

func redoNotice(res engine.UndoResult) string {
	kept := keptEffectPaths(res.Effects)
	if len(kept) == 0 {
		return "Redid the last change"
	}
	return fmt.Sprintf("Redid the last change, except %s you changed afterwards: %s", ui.Plural(len(kept), "file", "files"), strings.Join(kept, ", "))
}

func keptEffectPaths(effects []engine.Effect) []string {
	var kept []string
	for _, ef := range effects {
		if ef.Dir {
			continue
		}
		if ef.Op == "kept" || ef.Op == "unprotected" {
			kept = append(kept, ef.Path)
		}
	}
	return kept
}

// keptPreview lists files a partial undo left in place. The path stays in the
// preview so the row can keep a short marker.
func keptPreview(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  Kept %s\n", ui.Plural(len(paths), "file", "files"))
	n := len(paths)
	extra := 0
	if n > previewPathCap {
		extra = n - previewPathCap
		n = previewPathCap
	}
	for _, p := range paths[:n] {
		b.WriteString("    " + p + "\n")
	}
	if extra > 0 {
		fmt.Fprintf(&b, "    +%d more\n", extra)
	}
	return b.String()
}

func applyTimeline(s *session, a tui.Action) ([]tui.Row, string, error) {
	ctx := context.Background()
	var notice string
	var actErr error
	switch a.Kind {
	case "undo":
		res, err := s.e.Undo(ctx, engine.UndoRequest{Workspace: s.ws, Seq: a.Seq})
		if err != nil {
			actErr = err
		} else {
			notice = undoNotice(res)
		}
	case "rewind":
		res, err := s.e.Rewind(ctx, s.ws, a.Seq, false, false)
		if err != nil {
			actErr = err
		} else {
			n := a.Seq
			if len(res.Seqs) > 0 {
				n = res.Seqs[0]
			}
			notice = fmt.Sprintf("Rewound to before #%d", n)
		}
	case "redo":
		res, err := s.e.Redo(ctx, s.ws, false)
		if err != nil {
			actErr = err
		} else {
			notice = redoNotice(res)
		}
	}
	rows, err := timelineRows(s)
	if err != nil {
		return nil, "", err
	}
	if actErr != nil {
		return rows, "", actErr
	}
	return rows, notice, nil
}
