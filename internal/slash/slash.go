// Package slash runs /oops inside an agent session. It never opens a TUI.
package slash

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/ui"
)

// Run executes a /oops invocation and returns text plus an exit code.
func Run(e *engine.Engine, ws engine.Workspace, args []string, now time.Time) (string, int) {
	if err := e.WaitPending(context.Background(), ws); err != nil {
		return err.Error(), 1
	}
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "undo":
		seq := 0
		if len(args) > 1 {
			seq = parseSeq(args[1])
		}
		if seq == 0 {
			if open, err := e.DB.RunningTurns(ws.ID); err == nil && len(open) > 0 {
				t := open[len(open)-1]
				return fmt.Sprintf("#%d is still running (%s). Stop the agent first.", t.Seq, t.Agent), 4
			}
			if op, err := e.DB.LatestStackOp(ws.ID); err != nil || op.Applied {
				latest, lerr := e.DB.LatestFinished(ws.ID)
				target, terr := e.DB.LatestUndoable(ws.ID)
				if lerr != nil || terr != nil {
					return "Nothing to undo here yet. oops protects this folder once an agent runs in it.", 4
				}
				if target.Seq != latest.Seq {
					return fmt.Sprintf("oops undo %d needs confirmation. Re-run with -y in a terminal.", target.Seq), 2
				}
			}
		} else {
			latest, err := e.DB.LatestFinished(ws.ID)
			if err != nil {
				return "Nothing to undo here yet. oops protects this folder once an agent runs in it.", 4
			}
			if seq != latest.Seq {
				return fmt.Sprintf("oops undo %d needs confirmation. Re-run with -y in a terminal.", seq), 2
			}
		}
		res, err := e.Undo(context.Background(), engine.UndoRequest{Workspace: ws, Seq: seq})
		if err != nil {
			return err.Error(), 1
		}
		return formatUndo(res), codeOf(res)
	case "redo":
		res, err := e.Redo(context.Background(), ws, false)
		if err != nil {
			if meta.IsNotFound(err) {
				return "Nothing to redo.", 4
			}
			return err.Error(), 1
		}
		return "✓ oops · redid the last undo. /oops undo", codeOf(res)
	case "diff":
		seq := 0
		if len(args) > 1 {
			seq = parseSeq(args[1])
		}
		t, err := load(e, ws, seq)
		if err != nil {
			return err.Error(), 4
		}
		changes, err := e.ChangesBetween(t)
		if err != nil {
			return err.Error(), 1
		}
		var b strings.Builder
		fmt.Fprintf(&b, "✓ oops · diff #%d\n", t.Seq)
		files := snapshot.FileChanges(changes)
		if len(files) == 0 {
			b.WriteString("  no file changes\n")
		}
		for _, c := range files {
			fmt.Fprintf(&b, "  %s  %s\n", c.Action, c.Path)
		}
		b.WriteString("  type oops in a terminal for the full timeline")
		return b.String(), 0
	default:
		turns, err := e.DB.ListTurns(ws.ID, 5, false)
		if err != nil {
			return err.Error(), 1
		}
		if len(turns) == 0 {
			return "✓ oops · no snapshots yet. Not set up? oops init. Any other tool? oops run -- <command>", 0
		}
		var b strings.Builder
		b.WriteString("✓ oops · last turns\n")
		n := 0
		var first meta.Turn
		for _, t := range turns {
			if t.Kind == "safety" {
				continue
			}
			if n == 0 {
				first = t
			}
			if n == 5 {
				break
			}
			n++
			fmt.Fprintf(&b, "  #%d  %s   %s    %s\n", t.Seq, ui.RelTime(now, t.StartedAt), ui.Quote(t.Prompt, 60), ui.Plural(t.Stats.Files, "file", "files"))
		}
		verb := "undo"
		if first.Status == "undone" {
			verb = "redo"
		}
		fmt.Fprintf(&b, "  /oops %s  #%d   ·   /oops diff  preview   ·   type oops in a terminal for the full timeline", verb, first.Seq)
		return b.String(), 0
	}
}

func load(e *engine.Engine, ws engine.Workspace, seq int) (meta.Turn, error) {
	if seq == 0 {
		return e.DB.LatestFinished(ws.ID)
	}
	return e.DB.GetTurn(ws.ID, seq)
}

func parseSeq(s string) int {
	s = strings.TrimPrefix(s, "#")
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func formatUndo(res engine.UndoResult) string {
	n := engine.CountFileEffects(res.Effects)
	if n == 0 && res.Result == "partial" {
		return "✓ oops · nothing was written"
	}
	if n == 0 {
		return fmt.Sprintf("✓ oops · undid #%d. /oops redo", res.Turn.Seq)
	}
	return fmt.Sprintf("✓ oops · undid #%d (%s). /oops redo", res.Turn.Seq, ui.Plural(n, "file", "files"))
}

func codeOf(res engine.UndoResult) int {
	if res.Result == "partial" {
		return 3
	}
	return 0
}
