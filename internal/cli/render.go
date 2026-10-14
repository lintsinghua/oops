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
		writeUndoJSON(w, res, verb)
		return
	}
	if opt.quiet {
		return
	}
	if writtenCount(res.Effects) == 0 && res.Result == "partial" {
		renderNothingWritten(w, opt, res, verb)
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
	kept := 0
	for _, ef := range res.Effects {
		if ef.Dir {
			continue
		}
		if ef.Op == "kept" || ef.Op == "unprotected" {
			kept++
		}
	}
	if verb == "redo" {
		title = fmt.Sprintf("%s Redid the last change — back to how it was (%s).", sym.OK, ui.Plural(engine.CountFileEffects(res.Effects), "file", "files"))
		if res.Result == "partial" {
			title = fmt.Sprintf("%s Redid the last change, except %s you changed afterwards.", sym.Warn, ui.Plural(kept, "file", "files"))
		}
	} else if res.Result == "partial" {
		title = fmt.Sprintf("%s Undid turn #%d, except %s you changed afterwards.", sym.Warn, res.Turn.Seq, ui.Plural(kept, "file", "files"))
	}
	fmt.Fprintln(w, st.Success.Render(title))
	fmt.Fprintln(w)
	for _, line := range groupLines(res.Effects, true, pastLabels(verb)) {
		fmt.Fprintln(w, "  "+line)
	}
	if res.Result == "partial" {
		writeKeptHints(w, res, verb)
	}
	fmt.Fprintln(w)
	if verb == "redo" {
		fmt.Fprintln(w, "  Changed your mind? oops undo")
		return
	}
	fmt.Fprintln(w, "  Changed your mind? oops redo")
}

func writeUndoJSON(w io.Writer, res engine.UndoResult, verb string) {
	restored := pathsOf(res.Effects, "restore")
	reverted := pathsOf(res.Effects, "revert")
	commands := keptFileCommands(res)
	if verb == "redo" {
		// A redo puts the agent's bytes back. That is a restore, not a revert,
		// and the hint must not tell the user to undo the file they just kept.
		restored = append(restored, reverted...)
		sort.Strings(restored)
		reverted = []string{}
		commands = []string{}
	}
	out := map[string]any{
		"schema_version": 1,
		"result":         res.Result,
		"restored":       restored,
		"removed":        pathsOf(res.Effects, "remove"),
		"reverted":       reverted,
		"kept":           keptOf(res.Effects),
		"commands":       commands,
		"next":           nextCommand(verb, res.DryRun),
		"dry_run":        res.DryRun,
	}
	if writtenCount(res.Effects) == 0 && res.Result == "partial" {
		out["note"] = "Nothing was written."
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// nextCommand is the command the text output tells the user to run next.
// A dry run points at the command that applies it. An applied undo points at
// redo, and an applied redo points at undo.
func nextCommand(verb string, dry bool) string {
	if dry {
		if verb == "redo" {
			return "oops redo"
		}
		return "oops undo"
	}
	if verb == "redo" {
		return "oops undo"
	}
	return "oops redo"
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
	for _, path := range keptFilePaths(effects) {
		reason := ""
		for _, ef := range effects {
			if ef.Path == path && (ef.Op == "kept" || ef.Op == "unprotected") {
				reason = ef.Reason
				break
			}
		}
		out = append(out, map[string]string{"path": path, "reason": reason})
	}
	if out == nil {
		out = []map[string]string{}
	}
	return out
}

// keptFilePaths is the files a partial undo or redo left in place.
// Directories and parent paths are omitted: a directory hint would overwrite
// the user's file inside it.
func keptFilePaths(effects []engine.Effect) []string {
	var paths []string
	seen := map[string]bool{}
	for _, ef := range effects {
		if ef.Dir || strings.HasSuffix(ef.Path, "/") {
			continue
		}
		if ef.Op != "kept" && ef.Op != "unprotected" {
			continue
		}
		if seen[ef.Path] {
			continue
		}
		seen[ef.Path] = true
		paths = append(paths, ef.Path)
	}
	var out []string
	for _, p := range paths {
		if parentOfAny(p, paths) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func parentOfAny(path string, paths []string) bool {
	prefix := strings.TrimSuffix(path, "/") + "/"
	for _, other := range paths {
		if other != path && strings.HasPrefix(other, prefix) {
			return true
		}
	}
	return false
}

func keptFileCommands(res engine.UndoResult) []string {
	paths := keptFilePaths(res.Effects)
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, fmt.Sprintf("oops undo %d --file %s --force", res.Turn.Seq, p))
	}
	return out
}

func writeKeptHints(w io.Writer, res engine.UndoResult, verb string) {
	if verb == "redo" {
		return
	}
	paths := keptFilePaths(res.Effects)
	if len(paths) == 0 || res.Turn.Seq <= 0 {
		return
	}
	fmt.Fprintln(w)
	for _, p := range paths {
		fmt.Fprintf(w, "  Undo it too (your version is saved):  oops undo %d --file %s --force\n", res.Turn.Seq, p)
	}
}

func renderNothingWritten(w io.Writer, opt *options, res engine.UndoResult, verb string) {
	st, sym := stylesOf(opt)
	fmt.Fprintln(w, st.Success.Render(fmt.Sprintf("%s Nothing was written.", sym.Warn)))
	fmt.Fprintln(w)
	for _, line := range groupLines(res.Effects, true, pastLabels(verb)) {
		fmt.Fprintln(w, "  "+line)
	}
	writeKeptHints(w, res, verb)
	fmt.Fprintln(w)
}

func writtenCount(effects []engine.Effect) int {
	n := 0
	for _, ef := range effects {
		switch ef.Op {
		case "restore", "remove", "revert":
			n++
		}
	}
	return n
}

const previewPathCap = 8

// previewEffectLines is the timeline preview. Each path is its own line so a
// narrow column cannot glue two paths into one truncated name. The count stays
// on the verb line. Paths past previewPathCap collapse to "+K more".
func previewEffectLines(effects []engine.Effect) []string {
	labels := map[string]string{
		"restore": "restore", "remove": "remove", "revert": "revert",
		"add": "add", "modify": "modify",
		"keep": "keep", "kept": "keep", "unprotected": "keep",
	}
	buckets := map[string][]string{}
	for _, ef := range effects {
		if ef.Op == "skip" || ef.Dir {
			continue
		}
		op := ef.Op
		if op == "unprotected" || op == "kept" {
			op = "keep"
		}
		buckets[op] = append(buckets[op], ef.Path)
	}
	var lines []string
	for _, op := range []string{"restore", "remove", "revert", "add", "modify", "keep"} {
		paths := buckets[op]
		if len(paths) == 0 {
			continue
		}
		sort.Strings(paths)
		line := fmt.Sprintf("%-8s %s", labels[op], ui.Plural(len(paths), "file", "files"))
		if op == "keep" {
			line += " (yours)"
		}
		lines = append(lines, line)
		n := len(paths)
		if n > previewPathCap {
			n = previewPathCap
		}
		for _, p := range paths[:n] {
			lines = append(lines, "  "+p)
		}
		if extra := len(paths) - n; extra > 0 {
			lines = append(lines, fmt.Sprintf("  +%d more", extra))
		}
	}
	return lines
}

// pastLabels is how an applied redo names a content change. Dry runs stay
// in the present tense ("re-apply"); the applied line is "re-applied".
func pastLabels(verb string) map[string]string {
	if verb != "redo" {
		return nil
	}
	return map[string]string{"revert": "re-applied"}
}

func groupLines(effects []engine.Effect, past bool, override map[string]string) []string {
	labels := map[string]string{
		"restore": "restore", "remove": "remove", "revert": "revert",
		"add": "add", "modify": "modify",
		"kept": "kept", "unprotected": "kept",
	}
	if past {
		labels["restore"] = "restored"
		labels["remove"] = "removed"
		labels["revert"] = "reverted"
		labels["add"] = "added"
		labels["modify"] = "modified"
	}
	for op, label := range override {
		labels[op] = label
	}
	buckets := map[string][]string{}
	for _, ef := range effects {
		if ef.Op == "skip" || ef.Dir {
			continue
		}
		op := ef.Op
		if op == "unprotected" {
			op = "kept"
		}
		buckets[op] = append(buckets[op], ef.Path)
	}
	var lines []string
	for _, op := range []string{"restore", "remove", "revert", "add", "modify", "kept"} {
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
			if n := snapshot.CountFileChanges(gap); err == nil && n > 0 {
				fmt.Fprintf(w, "  %s    %-7s %-11s %s\n", sym.Gap, "you", ui.Pad(ui.RelTime(now, turns[i].StartedAt), 11), ui.Plural(n, "file changed outside agents", "files changed outside agents"))
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
		_ = writeShowJSON(w, t, changes)
		return
	}
	st, sym := stylesOf(opt)
	when := t.StartedAt.Format("15:04")
	fmt.Fprintf(w, "Turn #%d %s %s %s %s\n", t.Seq, sym.Sep, t.Agent, sym.Sep, when)
	if t.Prompt != "" {
		fmt.Fprintln(w, ui.Quote(t.Prompt, 0))
	}
	fmt.Fprintln(w)
	for _, c := range snapshot.FileChanges(changes) {
		action := c.Action
		extra := ""
		if c.Binary {
			extra = "  binary"
		} else if c.Insertions != 0 || c.Deletions != 0 {
			extra = fmt.Sprintf("  +%d %s%d", c.Insertions, sym.Minus, c.Deletions)
		}
		if c.Ignored {
			extra += "  ignored"
		}
		if c.Untracked {
			extra += "  untracked"
		}
		fmt.Fprintf(w, "  %-10s %s%s\n", action, c.Path, extra)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Undo: oops undo %d      Diff: oops diff %d\n", t.Seq, t.Seq)
	_ = st
}

func writeShowJSON(w io.Writer, t meta.Turn, changes []snapshot.Change) error {
	type row struct {
		Path       string `json:"path"`
		Action     string `json:"action"`
		Insertions int    `json:"insertions"`
		Deletions  int    `json:"deletions"`
		Binary     bool   `json:"binary,omitempty"`
		Untracked  bool   `json:"untracked,omitempty"`
		Ignored    bool   `json:"ignored,omitempty"`
	}
	kept := snapshot.FileChanges(changes)
	files := make([]row, 0, len(kept))
	for _, c := range kept {
		files = append(files, row{Path: c.Path, Action: c.Action, Insertions: c.Insertions, Deletions: c.Deletions, Binary: c.Binary, Untracked: c.Untracked, Ignored: c.Ignored})
	}
	ended := ""
	if !t.EndedAt.IsZero() {
		ended = t.EndedAt.Format(time.RFC3339)
	}
	out := struct {
		SchemaVersion int        `json:"schema_version"`
		Turn          int        `json:"turn"`
		Agent         string     `json:"agent"`
		Kind          string     `json:"kind"`
		Status        string     `json:"status"`
		StartedAt     string     `json:"started_at"`
		EndedAt       string     `json:"ended_at,omitempty"`
		Prompt        string     `json:"prompt"`
		Stats         meta.Stats `json:"stats"`
		Files         []row      `json:"files"`
	}{
		SchemaVersion: 1, Turn: t.Seq, Agent: t.Agent, Kind: t.Kind, Status: t.Status,
		StartedAt: t.StartedAt.Format(time.RFC3339), EndedAt: ended, Prompt: t.Prompt, Stats: t.Stats, Files: files,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func renderDry(w io.Writer, res engine.UndoResult, sym ui.Symbols, now time.Time, verb string) {
	name := "Undo"
	apply := "undo"
	switch verb {
	case "redo":
		name = "Redo"
		apply = "redo"
	case "rewind":
		name = "Rewind"
		apply = "rewind"
	}
	fmt.Fprintf(w, "%s turn #%d would %s %s %s %s %s %s\n\n", name, res.Turn.Seq, sym.Sep, res.Turn.Agent, sym.Sep, ui.RelTime(now, res.Turn.StartedAt), sym.Sep, ui.Quote(res.Turn.Prompt, 60))
	var labels map[string]string
	if verb == "redo" {
		labels = map[string]string{"revert": "re-apply"}
	}
	for _, line := range groupLines(res.Effects, false, labels) {
		fmt.Fprintln(w, "  "+line)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Dry run — nothing changed. Run oops %s to apply.\n", apply)
}
