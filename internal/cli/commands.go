package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/restore"
	"github.com/lintsinghua/oops/internal/run"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/ui"
)

func newUndoCmd(opt *options) *cobra.Command {
	var files []string
	var dry, force, yes, running bool
	cmd := &cobra.Command{
		Use:   "undo [turn]",
		Short: "Undo the last turn, or a specific one",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			seq := 0
			if len(args) == 1 {
				n, err := parseSeq(args[0])
				if err != nil {
					return err
				}
				seq = n
			}
			return cmdUndo(opt, seq, files, dry, force, yes, running)
		},
	}
	cmd.Flags().StringArrayVarP(&files, "file", "f", nil, "Only undo these paths")
	cmd.Flags().BoolVarP(&dry, "dry-run", "n", false, "Preview without changing files")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite files you changed after the turn")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	cmd.Flags().BoolVar(&running, "running", false, "Close a still-running turn, then undo it")
	cmd.Example = "  oops undo\n  oops undo 12\n  oops undo --file src/app.ts"
	return cmd
}

func cmdUndo(opt *options, seq int, files []string, dry, force, yes, running bool) error {
	opt.noLazyClose = true
	s, err := begin(opt)
	if err != nil {
		return err
	}
	defer s.e.Close()
	if !dry {
		if err := confirmUndo(opt, s, seq, files, force, yes, running); err != nil {
			return err
		}
	}
	res, err := s.e.Undo(context.Background(), engine.UndoRequest{
		Workspace: s.ws, Seq: seq, Files: files, Force: force, DryRun: dry, Running: running,
	})
	if err != nil {
		return explain(s, seq, err)
	}
	if dry {
		if opt.json {
			writeUndoJSON(opt.stdout, res)
			return nil
		}
		_, sym := stylesOf(opt)
		renderDry(opt.stdout, res, sym, time.Now())
		return nil
	}
	renderUndo(opt.stdout, opt, res, "undo")
	return resultCode(res)
}

func confirmUndo(opt *options, s *session, seq int, files []string, force, yes, running bool) error {
	if seq == 0 && len(files) == 0 && !running {
		open, err := s.e.DB.RunningTurns(s.ws.ID)
		if err != nil {
			return err
		}
		if len(open) > 0 {
			t := open[len(open)-1]
			return fail(4, fmt.Sprintf("#%d is still running (%s). Stop the agent first, then run oops undo --running.", t.Seq, t.Agent))
		}
		if op, err := s.e.DB.LatestStackOp(s.ws.ID); err == nil && !op.Applied {
			return nil
		}
	}
	latest, err := s.e.DB.LatestFinished(s.ws.ID)
	if err != nil {
		if meta.IsNotFound(err) {
			if running {
				if open, oerr := s.e.DB.RunningTurns(s.ws.ID); oerr == nil && len(open) > 0 {
					return nil
				}
			}
			return fail(4, "Nothing to undo here yet. oops protects this folder once an agent runs in it.")
		}
		return err
	}
	target := latest
	if seq == 0 {
		if done, derr := s.e.DB.LatestUndoable(s.ws.ID); derr == nil {
			target = done
		}
	} else if t, terr := s.e.DB.GetTurn(s.ws.ID, seq); terr == nil {
		target = t
	}
	if target.Status == "running" && !running {
		return fail(4, fmt.Sprintf("#%d is still running (%s). Stop the agent first, then run oops undo --running.", target.Seq, target.Agent))
	}
	needs := force || target.Seq != latest.Seq
	if !needs || yes {
		return nil
	}
	if !canAsk(opt) {
		return fail(2, "oops undo needs confirmation. Re-run with -y to proceed, or -n to preview.")
	}
	res, err := s.e.Undo(context.Background(), engine.UndoRequest{Workspace: s.ws, Seq: seq, Force: force, DryRun: true})
	if err != nil {
		return explain(s, seq, err)
	}
	_, sym := stylesOf(opt)
	renderDry(opt.stderr, res, sym, time.Now())
	fmt.Fprint(opt.stderr, "Undo? [Y/n] ")
	ok, err := ask(opt.stdin, true)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(opt.stdout, "Nothing changed.")
		return errStop
	}
	return nil
}

func newRewindCmd(opt *options) *cobra.Command {
	var dry, force, yes bool
	cmd := &cobra.Command{
		Use:   "rewind <turn>",
		Short: "Go back to before a turn",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			seq, err := parseSeq(args[0])
			if err != nil {
				return err
			}
			if seq == 0 {
				return fail(2, "oops rewind needs a turn number. See oops list.")
			}
			return cmdRewind(opt, seq, dry, force, yes)
		},
	}
	cmd.Flags().BoolVarP(&dry, "dry-run", "n", false, "Preview without changing files")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite files you changed after these turns")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	cmd.Example = "  oops rewind 12\n  oops rewind 12 -n"
	return cmd
}

func cmdRewind(opt *options, seq int, dry, force, yes bool) error {
	opt.noLazyClose = true
	s, err := begin(opt)
	if err != nil {
		return err
	}
	defer s.e.Close()
	if !dry && !yes {
		if !canAsk(opt) {
			return fail(2, "oops rewind needs confirmation. Re-run with -y to proceed, or -n to preview.")
		}
		turns, err := s.e.DB.ListTurns(s.ws.ID, 0, false)
		if err != nil {
			return err
		}
		var picked []meta.Turn
		for _, t := range turns {
			if t.Seq >= seq && t.Status != "running" {
				picked = append(picked, t)
			}
		}
		if len(picked) == 0 {
			return missingTurn(s, seq)
		}
		fmt.Fprintf(opt.stderr, "Go back to before turn #%d? This undoes %s:\n\n", seq, ui.Plural(len(picked), "turn", "turns"))
		for i := len(picked) - 1; i >= 0; i-- {
			t := picked[i]
			fmt.Fprintf(opt.stderr, "  #%d  %-8s %s\n", t.Seq, t.Agent, ui.Quote(t.Prompt, 48))
		}
		fmt.Fprintln(opt.stderr)
		fmt.Fprintln(opt.stderr, "  Your own edits in between are kept.")
		fmt.Fprintln(opt.stderr)
		fmt.Fprint(opt.stderr, "Rewind? [Y/n] ")
		ok, err := ask(opt.stdin, true)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(opt.stdout, "Nothing changed.")
			return errStop
		}
	}
	res, err := s.e.Rewind(context.Background(), s.ws, seq, force, dry)
	if err != nil {
		return explain(s, seq, err)
	}
	if dry {
		_, sym := stylesOf(opt)
		renderDry(opt.stdout, res, sym, time.Now())
		return nil
	}
	renderUndo(opt.stdout, opt, res, "rewind")
	return resultCode(res)
}

func newRedoCmd(opt *options) *cobra.Command {
	var dry bool
	cmd := &cobra.Command{
		Use:   "redo",
		Short: "Reverse the last undo, rewind or restore",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := begin(opt)
			if err != nil {
				return err
			}
			defer s.e.Close()
			res, err := s.e.Redo(context.Background(), s.ws, dry)
			if err != nil {
				if meta.IsNotFound(err) {
					return fail(4, "Nothing to redo.")
				}
				return explain(s, 0, err)
			}
			if dry {
				if opt.json {
					writeUndoJSON(opt.stdout, res)
					return nil
				}
				_, sym := stylesOf(opt)
				renderDry(opt.stdout, res, sym, time.Now())
				return nil
			}
			renderUndo(opt.stdout, opt, res, "redo")
			return resultCode(res)
		},
	}
	cmd.Flags().BoolVarP(&dry, "dry-run", "n", false, "Preview without changing files")
	cmd.Example = "  oops redo\n  oops redo -n"
	return cmd
}

func newRestoreCmd(opt *options) *cobra.Command {
	var at int
	var force, yes, dry bool
	cmd := &cobra.Command{
		Use:   "restore <path>... --at <turn>",
		Short: "Bring files back as they were before a turn",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if at < 1 {
				return fail(2, "oops restore needs --at <turn>.")
			}
			return cmdRestore(opt, at, args, force, yes, dry)
		},
	}
	cmd.Flags().IntVar(&at, "at", 0, "Turn to restore from (the files as they were before it)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite a file that differs")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	cmd.Flags().BoolVarP(&dry, "dry-run", "n", false, "Preview without changing files")
	cmd.Example = "  oops restore src/app.ts --at 12"
	return cmd
}

func cmdRestore(opt *options, seq int, paths []string, force, yes, dry bool) error {
	s, err := begin(opt)
	if err != nil {
		return err
	}
	defer s.e.Close()
	res, err := s.e.RestoreFiles(context.Background(), s.ws, seq, paths, force, true)
	if err != nil {
		return explain(s, seq, err)
	}
	if dry {
		renderUndo(opt.stdout, opt, res, "restore")
		fmt.Fprintln(opt.stdout, "Dry run — nothing changed.")
		return nil
	}
	needs := false
	for _, ef := range res.Effects {
		if ef.Op == "kept" {
			needs = true
		}
	}
	applyForce := force
	if needs && !force {
		if yes {
			applyForce = true
		} else if !canAsk(opt) {
			return fail(2, "oops restore needs confirmation. Re-run with --force to overwrite, or -n to preview.")
		} else {
			for _, ef := range res.Effects {
				if ef.Op != "kept" {
					continue
				}
				fmt.Fprintf(opt.stderr, "Overwrite %s? [y/N] ", ef.Path)
				ok, err := ask(opt.stdin, false)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(opt.stdout, "Nothing changed.")
					return errStop
				}
			}
			applyForce = true
		}
	}
	res, err = s.e.RestoreFiles(context.Background(), s.ws, seq, paths, applyForce, false)
	if err != nil {
		return explain(s, seq, err)
	}
	if opt.json {
		renderUndo(opt.stdout, opt, res, "restore")
		return resultCode(res)
	}
	if !opt.quiet {
		_, sym := stylesOf(opt)
		var shown []string
		for _, ef := range res.Effects {
			if ef.Op == "restore" || ef.Op == "revert" {
				shown = append(shown, ef.Path)
			}
		}
		fmt.Fprintf(opt.stdout, "%s Restored %s as it was before turn #%d.\n", sym.OK, strings.Join(shown, ", "), seq)
		fmt.Fprintln(opt.stdout, "  Changed your mind? oops redo")
	}
	return resultCode(res)
}

func newListCmd(opt *options) *cobra.Command {
	var n int
	var all bool
	var agent, since string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List recent turns",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdList(opt, n, all, agent, since)
		},
	}
	cmd.Flags().IntVarP(&n, "n", "n", 10, "How many turns to show")
	cmd.Flags().BoolVar(&all, "all", false, "Include safety snapshots")
	cmd.Flags().StringVar(&agent, "agent", "", "Only this agent")
	cmd.Flags().StringVar(&since, "since", "", "Only turns newer than this duration (for example 24h)")
	cmd.Example = "  oops list\n  oops list -n 5\n  oops list --json"
	return cmd
}

func cmdList(opt *options, n int, all bool, agent, since string) error {
	s, err := begin(opt)
	if err != nil {
		return err
	}
	defer s.e.Close()
	limit := n
	if all && n == 10 {
		limit = 0
	}
	turns, err := s.e.DB.ListTurns(s.ws.ID, 0, all)
	if err != nil {
		return err
	}
	turns = filterTurns(turns, agent, since, time.Now())
	if limit > 0 && len(turns) > limit {
		turns = turns[:limit]
	}
	return renderList(opt.stdout, opt, s.e, s.ws, turns, time.Now())
}

func filterTurns(turns []meta.Turn, agent, since string, now time.Time) []meta.Turn {
	var cutoff time.Time
	if since != "" {
		if d, err := time.ParseDuration(since); err == nil {
			cutoff = now.Add(-d)
		}
	}
	var out []meta.Turn
	for _, t := range turns {
		if agent != "" && !strings.EqualFold(t.Agent, agent) {
			continue
		}
		if !cutoff.IsZero() && t.StartedAt.Before(cutoff) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func newShowCmd(opt *options) *cobra.Command {
	return &cobra.Command{
		Use:   "show [turn]",
		Short: "Show what a turn changed",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			seq := 0
			if len(args) == 1 {
				n, err := parseSeq(args[0])
				if err != nil {
					return err
				}
				seq = n
			}
			return cmdShow(opt, seq)
		},
		Example: "  oops show\n  oops show 12",
	}
}

func cmdShow(opt *options, seq int) error {
	s, err := begin(opt)
	if err != nil {
		return err
	}
	defer s.e.Close()
	t, err := loadTurn(s, seq)
	if err != nil {
		return err
	}
	changes, err := s.e.ChangesBetween(t)
	if err != nil {
		return err
	}
	annotateGit(s.ws.Path, changes)
	var skipped []meta.Skipped
	seen := map[string]bool{}
	for _, id := range []string{t.PreSnap, t.PostSnap} {
		if _, row, err := s.e.Files(id); err == nil {
			for _, sk := range row.Skipped {
				if seen[sk.Path] {
					continue
				}
				seen[sk.Path] = true
				skipped = append(skipped, sk)
			}
		}
	}
	renderShow(opt.stdout, opt, t, changes)
	if !opt.json && !opt.quiet {
		for _, sk := range skipped {
			fmt.Fprintf(opt.stdout, "  ! skipped %s — %s, not protected\n", sk.Path, ui.Size(sk.Size))
		}
	}
	return nil
}

func newDiffCmd(opt *options) *cobra.Command {
	var stat, names, reverse, noPager bool
	cmd := &cobra.Command{
		Use:   "diff [turn] [-- path...]",
		Short: "Show a turn's diff",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			seq, paths := splitSpec(args)
			return cmdDiff(opt, seq, paths, stat, names, reverse, noPager)
		},
	}
	cmd.Flags().BoolVar(&stat, "stat", false, "Show a summary instead of a patch")
	cmd.Flags().BoolVar(&names, "name-only", false, "List paths only")
	cmd.Flags().BoolVar(&reverse, "reverse", false, "Show the undo direction")
	cmd.Flags().BoolVar(&noPager, "no-pager", false, "Print the diff without a pager")
	cmd.Example = "  oops diff\n  oops diff 12 --stat\n  oops diff -- src/app.ts"
	return cmd
}

func splitSpec(args []string) (int, []string) {
	if len(args) == 0 {
		return 0, nil
	}
	n, err := parseSeq(args[0])
	if err != nil {
		return 0, args
	}
	return n, args[1:]
}

func cmdDiff(opt *options, seq int, paths []string, stat, names, reverse, noPager bool) error {
	s, err := begin(opt)
	if err != nil {
		return err
	}
	defer s.e.Close()
	t, err := loadTurn(s, seq)
	if err != nil {
		return err
	}
	changes, err := s.e.ChangesBetween(t)
	if err != nil {
		return err
	}
	if len(paths) > 0 {
		changes = filterChanges(changes, paths)
	}
	if opt.json {
		return writeDiffJSON(opt.stdout, t, changes)
	}
	var buf bytes.Buffer
	if len(changes) == 0 {
		fmt.Fprintln(&buf, "No file changes.")
		return pageOrPrint(opt, noPager, buf.Bytes())
	}
	for _, c := range changes {
		if c.Pre != nil && c.Pre.Kind == snapshot.KindDir && (c.Post == nil || c.Post.Kind == snapshot.KindDir) {
			continue
		}
		if names {
			fmt.Fprintln(&buf, c.Path)
			continue
		}
		if stat {
			extra := ""
			if c.Binary {
				extra = "  binary"
			} else if c.Insertions != 0 || c.Deletions != 0 {
				extra = fmt.Sprintf("  +%d −%d", c.Insertions, c.Deletions)
			}
			fmt.Fprintf(&buf, "%s %s%s\n", c.Action, c.Path, extra)
			continue
		}
		oldB, newB := s.e.ReadBlob(c.Pre), s.e.ReadBlob(c.Post)
		oldName, newName := "a/"+c.Path, "b/"+c.Path
		if reverse {
			oldB, newB = newB, oldB
			oldName, newName = newName, oldName
		}
		if c.Binary {
			fmt.Fprintf(&buf, "diff %s\n", c.Path)
			continue
		}
		text, err := snapshot.Unified(oldName, newName, string(oldB), string(newB), 3)
		if err != nil || text == "" {
			fmt.Fprintf(&buf, "%s %s\n", c.Action, c.Path)
			continue
		}
		fmt.Fprintln(&buf, text)
	}
	return pageOrPrint(opt, noPager, buf.Bytes())
}

func pageOrPrint(opt *options, noPager bool, b []byte) error {
	lines := bytes.Count(b, []byte{'\n'})
	if noPager || !isTTY(opt.stdout) || lines < 40 {
		_, err := opt.stdout.Write(b)
		return err
	}
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}
	var args []string
	if filepath.Base(pager) == "less" {
		args = []string{"-FRX"}
	}
	cmd := exec.Command(pager, args...)
	cmd.Stdin = bytes.NewReader(b)
	cmd.Stdout = opt.stdout
	cmd.Stderr = opt.stderr
	if err := cmd.Run(); err != nil {
		_, werr := opt.stdout.Write(b)
		return werr
	}
	return nil
}

func filterChanges(changes []snapshot.Change, paths []string) []snapshot.Change {
	var out []snapshot.Change
	for _, c := range changes {
		for _, p := range paths {
			if c.Path == p || strings.HasPrefix(c.Path, strings.TrimSuffix(p, "/")+"/") {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func writeDiffJSON(w io.Writer, t meta.Turn, changes []snapshot.Change) error {
	type row struct {
		Path       string `json:"path"`
		Action     string `json:"action"`
		Insertions int    `json:"insertions"`
		Deletions  int    `json:"deletions"`
		Binary     bool   `json:"binary,omitempty"`
	}
	out := struct {
		SchemaVersion int   `json:"schema_version"`
		Turn          int   `json:"turn"`
		Files         []row `json:"files"`
	}{SchemaVersion: 1, Turn: t.Seq}
	for _, c := range changes {
		out.Files = append(out.Files, row{Path: c.Path, Action: c.Action, Insertions: c.Insertions, Deletions: c.Deletions, Binary: c.Binary})
	}
	if out.Files == nil {
		out.Files = []row{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func newSnapCmd(opt *options) *cobra.Command {
	var message string
	var dry bool
	cmd := &cobra.Command{
		Use:   "snap",
		Short: "Save a checkpoint now",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dry {
				if opt.json {
					fmt.Fprintln(opt.stdout, `{"schema_version":1,"dry_run":true,"action":"snap"}`)
					return nil
				}
				fmt.Fprintln(opt.stdout, "Dry run — would save a checkpoint. Nothing changed.")
				return nil
			}
			s, err := begin(opt)
			if err != nil {
				return err
			}
			defer s.e.Close()
			t, err := s.e.ManualSnap(context.Background(), s.ws, message)
			if err != nil {
				return err
			}
			if opt.json {
				return writeListJSON(opt.stdout, s.ws, []meta.Turn{t})
			}
			if opt.quiet {
				return nil
			}
			_, sym := stylesOf(opt)
			q := ""
			if t.Prompt != "" {
				q = " " + ui.Quote(t.Prompt, 60)
			}
			fmt.Fprintf(opt.stdout, "%s Saved checkpoint #%d%s.  Go back: oops rewind %d\n", sym.OK, t.Seq, q, t.Seq)
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Note stored with the checkpoint")
	cmd.Flags().BoolVarP(&dry, "dry-run", "n", false, "Preview without writing a checkpoint")
	cmd.Example = "  oops snap -m \"before the experiment\""
	return cmd
}

func newRunCmd(opt *options) *cobra.Command {
	var name, idle string
	cmd := &cobra.Command{
		Use:   "run -- <command> [args...]",
		Short: "Protect any tool that has no hooks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdRun(opt, name, idle, args)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Label stored on the turn")
	cmd.Flags().StringVar(&idle, "idle", "", "Silence before an automatic checkpoint (default 3s)")
	cmd.Example = "  oops run -- claude\n  oops run --idle 3s -- npm test"
	return cmd
}

func newGCCmd(opt *options) *cobra.Command {
	var dry, all, global, yes bool
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Free up snapshot storage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdGC(opt, dry, all, global, yes)
		},
	}
	cmd.Flags().BoolVarP(&dry, "dry-run", "n", false, "Show what would be freed")
	cmd.Flags().BoolVar(&all, "all", false, "Delete this workspace's snapshots")
	cmd.Flags().BoolVar(&global, "global", false, "With --all, delete every workspace")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	cmd.Example = "  oops gc -n\n  oops gc"
	return cmd
}

func cmdGC(opt *options, dry, all, global, yes bool) error {
	e, err := openEngine(opt)
	if err != nil {
		return err
	}
	defer e.Close()
	if all && !dry && !yes {
		if !canAsk(opt) {
			return fail(2, "oops gc --all needs confirmation. Re-run with -y to proceed, or -n to preview.")
		}
		what := "this workspace's snapshots"
		if global {
			what = "snapshots for every workspace"
		}
		fmt.Fprintf(opt.stderr, "Delete %s? This cannot be undone. [y/N] ", what)
		ok, err := ask(opt.stdin, false)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(opt.stdout, "Nothing changed.")
			return errStop
		}
	}
	var targets []engine.Workspace
	if global {
		rows, err := e.DB.Workspaces()
		if err != nil {
			return err
		}
		for _, w := range rows {
			targets = append(targets, engine.Workspace{ID: w.ID, Path: w.Path})
		}
	} else {
		ws, err := loadWS(opt, e)
		if err != nil {
			return err
		}
		targets = []engine.Workspace{ws}
	}
	total := engine.GCResult{DryRun: dry}
	for _, ws := range targets {
		res, err := e.GC(ws, dry, all)
		if err != nil {
			return err
		}
		total.TurnsRemoved += res.TurnsRemoved
		total.ObjectsRemoved += res.ObjectsRemoved
		total.BytesFreed += res.BytesFreed
	}
	if opt.json {
		out := map[string]any{
			"schema_version":  1,
			"turns_removed":   total.TurnsRemoved,
			"objects_removed": total.ObjectsRemoved,
			"bytes_freed":     total.BytesFreed,
			"dry_run":         dry,
		}
		enc := json.NewEncoder(opt.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	if opt.quiet {
		return nil
	}
	verb := "Freed"
	if dry {
		verb = "Would free"
	}
	fmt.Fprintf(opt.stdout, "%s %s. %s removed, %s removed.\n", verb, ui.Size(total.BytesFreed), ui.Plural(total.TurnsRemoved, "turn", "turns"), ui.Plural(total.ObjectsRemoved, "object", "objects"))
	return nil
}

type session struct {
	opt *options
	e   *engine.Engine
	ws  engine.Workspace
}

func begin(opt *options) (*session, error) {
	e, err := openEngine(opt)
	if err != nil {
		return nil, err
	}
	ws, err := loadWS(opt, e)
	if err != nil {
		_ = e.Close()
		return nil, err
	}
	if err := e.WaitPending(context.Background(), ws); err != nil {
		_ = e.Close()
		return nil, err
	}
	return &session{opt: opt, e: e, ws: ws}, nil
}

func loadTurn(s *session, seq int) (meta.Turn, error) {
	var t meta.Turn
	var err error
	if seq == 0 {
		t, err = s.e.DB.LatestFinished(s.ws.ID)
	} else {
		t, err = s.e.DB.GetTurn(s.ws.ID, seq)
	}
	if err != nil {
		if meta.IsNotFound(err) {
			return t, missingTurn(s, seq)
		}
		return t, err
	}
	return t, nil
}

func missingTurn(s *session, seq int) error {
	latest, err := s.e.DB.LatestFinished(s.ws.ID)
	if err != nil {
		return fail(4, "Nothing to undo here yet. oops protects this folder once an agent runs in it.")
	}
	if seq == 0 {
		return fail(4, "Nothing to undo here yet. oops protects this folder once an agent runs in it.")
	}
	return fail(4, fmt.Sprintf("There's no turn #%d here. The latest is #%d — see oops list.", seq, latest.Seq))
}

func explain(s *session, seq int, err error) error {
	if err == nil {
		return nil
	}
	var run *engine.RunningError
	if errors.As(err, &run) {
		return fail(4, fmt.Sprintf("#%d is still running (%s). Stop the agent first, then run oops undo --running.", run.Turn.Seq, run.Turn.Agent))
	}
	var pe *engine.PathError
	if errors.As(err, &pe) {
		return fail(2, fmt.Sprintf("%s wasn't part of turn #%d.", strings.Join(pe.Missing, ", "), pe.Seq))
	}
	var busy *restore.BusyError
	if errors.As(err, &busy) {
		sym := ui.Glyphs(ui.ASCIIMode())
		return fail(1, fmt.Sprintf("%s Couldn't write %s — it's open in another program.\nClose it and run oops undo again. Nothing else was changed.", sym.Fail, busy.Path))
	}
	if errors.Is(err, syscall.ENOSPC) {
		return fail(1, "✗ Not enough disk space to undo safely. Nothing was changed. Free space or run oops gc.")
	}
	if meta.IsNotFound(err) {
		return missingTurn(s, seq)
	}
	if s != nil && s.opt.debug {
		return fail(1, err.Error())
	}
	return fail(1, err.Error())
}

func resultCode(res engine.UndoResult) error {
	if res.Result == "partial" {
		return fail(3, "")
	}
	return nil
}

func canAsk(opt *options) bool {
	if opt.json {
		return false
	}
	return isTTY(opt.stdin) && isTTY(opt.stdout)
}

// errStop ends a command after a user declined, without an error line.
var errStop = errors.New("stopped")

func cmdRun(opt *options, name, idle string, args []string) error {
	s, err := begin(opt)
	if err != nil {
		return err
	}
	defer s.e.Close()
	dur := 3 * time.Second
	if idle != "" {
		d, err := time.ParseDuration(idle)
		if err != nil {
			return fail(2, "oops run --idle wants a duration like 3s.")
		}
		dur = d
	}
	code := run.Run(s.e, s.ws, run.Options{
		Name: name, Argv: args, Idle: dur,
		Stdin: opt.stdin, Stdout: opt.stdout, Stderr: opt.stderr,
	})
	if code == 0 {
		return nil
	}
	return fail(code, "")
}
