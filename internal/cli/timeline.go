package cli

import (
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
	prog := bubbletea.NewProgram(model, bubbletea.WithInput(opt.stdin), bubbletea.WithOutput(opt.stdout), bubbletea.WithMouseCellMotion())
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
	var rows []tui.Row
	for i, t := range turns {
		if t.Kind == "safety" {
			continue
		}
		row := tui.Row{
			Seq: t.Seq, Agent: t.Agent, When: ui.RelTime(now, t.StartedAt),
			Prompt: t.Prompt, Files: t.Stats.Files, Status: t.Status,
		}
		changes, err := s.e.ChangesBetween(t)
		if err == nil {
			row.Preview, row.Diffs = previewTurn(s.e, t, changes)
			if len(row.Diffs) > 0 {
				row.Diff = row.Diffs[0]
			}
		}
		rows = append(rows, row)
		if i+1 < len(turns) {
			gap, err := s.e.GapFiles(turns[i+1], turns[i])
			if err == nil && len(gap) > 0 {
				rows = append(rows, tui.Row{
					Gap: true, Agent: "you", When: ui.RelTime(now, turns[i].StartedAt),
					Prompt: ui.Plural(len(gap), "file changed outside agents", "files changed outside agents"),
				})
			}
		}
	}
	return rows, nil
}

func previewTurn(e *engine.Engine, t meta.Turn, changes []snapshot.Change) (string, []string) {
	var effects []engine.Effect
	var diffs []string
	for _, c := range changes {
		if c.Pre != nil && c.Pre.Kind == snapshot.KindDir && (c.Post == nil || c.Post.Kind == snapshot.KindDir) {
			continue
		}
		effects = append(effects, engine.Effect{Path: c.Path, Op: c.UndoAction})
		oldB, newB := e.ReadBlob(c.Post), e.ReadBlob(c.Pre)
		text, err := snapshot.Unified("now/"+c.Path, "undo/"+c.Path, string(oldB), string(newB), 3)
		if err != nil || text == "" {
			text = c.UndoAction + " " + c.Path
		}
		if len(diffs) < 20 {
			diffs = append(diffs, fmt.Sprintf(" #%d · %s %s\n%s", t.Seq, c.UndoAction, c.Path, text))
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  Undo #%d would:\n\n", t.Seq)
	lines := groupLines(effects, false)
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
