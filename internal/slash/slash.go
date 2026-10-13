// Package slash runs /oops inside an agent session. It never opens a TUI.
package slash

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/ui"
)

// Run executes a /oops invocation and returns text plus an exit code.
func Run(e *engine.Engine, ws engine.Workspace, args []string, now time.Time) (string, int) {
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
		return "✓ oops · redid the last undo. /oops undo to flip it back", codeOf(res)
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
		fmt.Fprintf(&b, "oops · diff #%d\n", t.Seq)
		if len(changes) == 0 {
			b.WriteString("  no file changes\n")
		}
		for _, c := range changes {
			if c.Pre != nil && c.Pre.Kind == 2 && (c.Post == nil || c.Post.Kind == 2) {
				continue
			}
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
			return "No snapshots here yet.\n  Not set up?  oops init        Any other tool?  oops run -- <command>", 0
		}
		var b strings.Builder
		b.WriteString("oops · last turns\n")
		n := 0
		for _, t := range turns {
			if t.Kind == "safety" {
				continue
			}
			if n == 5 {
				break
			}
			n++
			fmt.Fprintf(&b, "  #%d  %s   %s    %s\n", t.Seq, ui.RelTime(now, t.StartedAt), ui.Quote(t.Prompt, 60), ui.Plural(t.Stats.Files, "file", "files"))
		}
		b.WriteString("  /oops undo  undo #" + fmt.Sprint(turns[0].Seq) + "   ·   /oops diff  preview   ·   type oops in a terminal for the full timeline")
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
	rest, rem, rev := 0, 0, 0
	for _, ef := range res.Effects {
		switch ef.Op {
		case "restore":
			rest++
		case "remove":
			rem++
		case "revert":
			rev++
		}
	}
	return fmt.Sprintf("✓ oops · undid #%d — restored %d, removed %d, reverted %d.  /oops redo to bring it back", res.Turn.Seq, rest, rem, rev)
}

func codeOf(res engine.UndoResult) int {
	if res.Result == "partial" {
		return 3
	}
	return 0
}
