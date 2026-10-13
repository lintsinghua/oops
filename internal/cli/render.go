package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
	"github.com/lintsinghua/oops/internal/snapshot"
	"github.com/lintsinghua/oops/internal/ui"
)

func stylesOf(opt *options) (ui.Styles, ui.Symbols) {
	ascii := ui.ASCIIMode() || !opt.colorOn()
	// Glyphs follow the terminal, not the color flag. NO_COLOR keeps Unicode
	// unless ASCII mode is on. Color-off still uses the success glyph.
	_ = ascii
	return ui.NewStyles(opt.colorOn()), ui.Glyphs(ui.ASCIIMode())
}

func renderUndo(w io.Writer, opt *options, res engine.UndoResult, verb string) {
	if opt.json {
		writeUndoJSON(w, res)
		return
	}
	if opt.quiet {
		return
	}
	st, sym := stylesOf(opt)
	title := fmt.Sprintf("%s Undid turn #%d", sym.OK, res.Turn.Seq)
	if res.Turn.Agent != "" {
		title += " " + sym.Sep + " " + res.Turn.Agent
	}
	if res.Turn.Prompt != "" {
		title += " " + sym.Sep + " " + ui.Quote(res.Turn.Prompt, 60)
	}
	if verb == "rewind" {
		title = fmt.Sprintf("%s Rewound to before #%d", sym.OK, res.Seqs[0])
	}
	if verb == "redo" {
		n := 0
		for _, ef := range res.Effects {
			if ef.Op != "kept" && ef.Op != "skip" {
				n++
			}
		}
		title = fmt.Sprintf("%s Redid the last change — back to how it was (%s).", sym.OK, ui.Plural(n, "file", "files"))
	}
	if res.Result == "partial" {
		kept := countOp(res.Effects, "kept") + countOp(res.Effects, "unprotected")
		title = fmt.Sprintf("%s Undid turn #%d, except %s you changed afterwards.", sym.Warn, res.Turn.Seq, ui.Plural(kept, "file", "files"))
	}
	fmt.Fprintln(w, st.Success.Render(title))
	fmt.Fprintln(w)
	for _, line := range groupLines(res.Effects, true) {
		fmt.Fprintln(w, "  "+line)
	}
	if res.Result == "partial" {
		fmt.Fprintln(w)
		for _, ef := range res.Effects {
			if ef.Op == "kept" || ef.Op == "unprotected" {
				fmt.Fprintf(w, "  Undo it too (your version is saved):  oops undo %d --file %s --force\n", res.Turn.Seq, ef.Path)
			}
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Changed your mind? oops redo")
}

func writeUndoJSON(w io.Writer, res engine.UndoResult) {
	out := map[string]any{
		"schema_version": 1,
		"result":         res.Result,
		"restored":       pathsOf(res.Effects, "restore"),
		"removed":        pathsOf(res.Effects, "remove"),
		"reverted":       pathsOf(res.Effects, "revert"),
		"kept":           keptOf(res.Effects),
		"redo":           "oops redo",
		"dry_run":        res.DryRun,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func pathsOf(effects []engine.Effect, op string) []string {
	var out []string
	for _, ef := range effects {
		if ef.Op == op {
			out = append(out, ef.Path)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func keptOf(effects []engine.Effect) []map[string]string {
	var out []map[string]string
	for _, ef := range effects {
		if ef.Op == "kept" || ef.Op == "unprotected" {
			out = append(out, map[string]string{"path": ef.Path, "reason": ef.Reason})
		}
	}
	if out == nil {
		out = []map[string]string{}
	}
	return out
}

func countOp(effects []engine.Effect, op string) int {
	n := 0
	for _, ef := range effects {
		if ef.Op == op {
			n++
		}
	}
	return n
}

func groupLines(effects []engine.Effect, past bool) []string {
	labels := map[string]string{
		"restore": "restore", "remove": "remove", "revert": "revert",
		"kept": "kept", "unprotected": "kept",
	}
	if past {
		labels["restore"] = "restored"
		labels["remove"] = "removed"
		labels["revert"] = "reverted"
	}
	buckets := map[string][]string{}
	for _, ef := range effects {
		if ef.Op == "skip" {
			continue
		}
		op := ef.Op
		if op == "unprotected" {
			op = "kept"
		}
		buckets[op] = append(buckets[op], ef.Path)
	}
	var lines []string
	for _, op := range []string{"restore", "remove", "revert", "kept"} {
		paths := buckets[op]
		if len(paths) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%-8s %s   %s", labels[op], ui.Plural(len(paths), "file", "files"), collapsePaths(paths)))
	}
	return lines
}

func collapsePaths(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	sort.Strings(paths)
	if len(paths) == 1 {
		return paths[0]
	}
	groups := map[string][]string{}
	var order []string
	for _, p := range paths {
		var dir string
		if i := strings.LastIndex(p, "/"); i >= 0 {
			dir = p[:i+1]
		}
		if _, ok := groups[dir]; !ok {
			order = append(order, dir)
		}
		groups[dir] = append(groups[dir], p)
	}
	var parts []string
	for _, dir := range order {
		g := groups[dir]
		if dir != "" && len(g) >= 3 {
			parts = append(parts, fmt.Sprintf("%s (%d)", dir, len(g)))
			continue
		}
		if len(g) > 4 {
			parts = append(parts, fmt.Sprintf("%s (%d)", dir, len(g)))
			continue
		}
		parts = append(parts, strings.Join(g, ", "))
	}
	return strings.Join(parts, ", ")
}

func renderList(w io.Writer, opt *options, e *engine.Engine, ws engine.Workspace, turns []meta.Turn, now time.Time) error {
	if opt.json {
		return writeListJSON(w, ws, turns)
	}
	st, sym := stylesOf(opt)
	if len(turns) == 0 {
		fmt.Fprintln(w, "No snapshots here yet.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  oops protects this folder once an agent runs in it.")
		fmt.Fprintln(w, "  Not set up?  oops init        Any other tool?  oops run -- <command>")
		return nil
	}
	for i, t := range turns {
		if t.Kind == "safety" && !opt.json {
			fmt.Fprintf(w, "  %s    safety   %s   undo data\n", sym.Gap, ui.RelTime(now, t.StartedAt))
			continue
		}
		status := ""
		if t.Status == "undone" || t.Status == "running" {
			status = "  " + t.Status
		}
		delta := ""
		if t.Stats.Insertions != 0 || t.Stats.Deletions != 0 {
			delta = fmt.Sprintf("  +%d %s%d", t.Stats.Insertions, sym.Minus, t.Stats.Deletions)
		}
		line := fmt.Sprintf("  #%d  %-7s %-11s %-40s %s%s%s", t.Seq, ui.Pad(t.Agent, 7), ui.Pad(ui.RelTime(now, t.StartedAt), 11), ui.Quote(t.Prompt, 36), ui.Plural(t.Stats.Files, "file", "files"), delta, status)
		if t.Status == "undone" {
			line = st.Muted.Render(line)
		}
		fmt.Fprintln(w, line)
		if i+1 < len(turns) {
			gap, err := e.GapFiles(turns[i+1], turns[i])
			if err == nil && len(gap) > 0 {
				fmt.Fprintf(w, "  %s    %-7s %-11s %s\n", sym.Gap, "you", ui.Pad(ui.RelTime(now, turns[i].StartedAt), 11), ui.Plural(len(gap), "file changed outside agents", "files changed outside agents"))
			}
		}
	}
	return nil
}

func writeListJSON(w io.Writer, ws engine.Workspace, turns []meta.Turn) error {
	type row struct {
		ID        int        `json:"id"`
		Agent     string     `json:"agent"`
		SessionID string     `json:"session_id"`
		Kind      string     `json:"kind"`
		Status    string     `json:"status"`
		StartedAt string     `json:"started_at"`
		EndedAt   string     `json:"ended_at,omitempty"`
		Prompt    string     `json:"prompt"`
		Stats     meta.Stats `json:"stats"`
	}
	out := struct {
		SchemaVersion int    `json:"schema_version"`
		Workspace     string `json:"workspace"`
		Turns         []row  `json:"turns"`
	}{SchemaVersion: 1, Workspace: ws.Path}
	for _, t := range turns {
		ended := ""
		if !t.EndedAt.IsZero() {
			ended = t.EndedAt.Format(time.RFC3339)
		}
		out.Turns = append(out.Turns, row{
			ID: t.Seq, Agent: t.Agent, SessionID: t.SessionID, Kind: t.Kind, Status: t.Status,
			StartedAt: t.StartedAt.Format(time.RFC3339), EndedAt: ended, Prompt: t.Prompt, Stats: t.Stats,
		})
	}
	if out.Turns == nil {
		out.Turns = []row{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func renderShow(w io.Writer, opt *options, t meta.Turn, changes []snapshot.Change) {
	if opt.json {
		_ = writeListJSON(w, engine.Workspace{}, []meta.Turn{t})
		return
	}
	st, sym := stylesOf(opt)
	when := t.StartedAt.Format("15:04")
	fmt.Fprintf(w, "Turn #%d %s %s %s %s\n", t.Seq, sym.Sep, t.Agent, sym.Sep, when)
	if t.Prompt != "" {
		fmt.Fprintln(w, ui.Quote(t.Prompt, 0))
	}
	fmt.Fprintln(w)
	for _, c := range changes {
		if c.Pre != nil && c.Pre.Kind == snapshot.KindDir && (c.Post == nil || c.Post.Kind == snapshot.KindDir) {
			continue
		}
		action := c.Action
		extra := ""
		if c.Binary {
			extra = "  binary"
		} else if c.Insertions != 0 || c.Deletions != 0 {
			extra = fmt.Sprintf("  +%d %s%d", c.Insertions, sym.Minus, c.Deletions)
		}
		fmt.Fprintf(w, "  %-10s %s%s\n", action, c.Path, extra)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Undo: oops undo %d      Diff: oops diff %d\n", t.Seq, t.Seq)
	_ = st
}

func renderDry(w io.Writer, res engine.UndoResult, sym ui.Symbols, now time.Time) {
	fmt.Fprintf(w, "Undo turn #%d %s %s %s %s %s %s\n\n", res.Turn.Seq, sym.Sep, res.Turn.Agent, sym.Sep, ui.RelTime(now, res.Turn.StartedAt), sym.Sep, ui.Quote(res.Turn.Prompt, 60))
	for _, line := range groupLines(res.Effects, false) {
		fmt.Fprintln(w, "  "+line)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Dry run — nothing changed. Run oops undo to apply.")
}
