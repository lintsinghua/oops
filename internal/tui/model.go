// Package tui is the Bubble Tea timeline.
package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lintsinghua/oops/internal/ui"
)

// Row is one timeline line prepared by the CLI.
type Row struct {
	Gap     bool
	Seq     int
	Agent   string
	When    string
	Prompt  string
	Files   int
	Status  string
	Preview string
	Diff    string
	Diffs   []string
}

// Action is what the user confirmed, if anything.
type Action struct {
	Kind string // undo, rewind, redo
	Seq  int
}

// Apply runs undo, rewind, or redo and returns the refreshed rows plus a one-line notice.
// When it is set, the timeline stays open. When it is nil, a confirmed action quits.
type ApplyFunc func(Action) ([]Row, string, error)

type refreshedMsg struct {
	rows   []Row
	notice string
	err    error
	seq    int
}

// Model is the timeline.
type Model struct {
	Rows      []Row
	Header    string
	Width     int
	Height    int
	styles    ui.Styles
	sym       ui.Symbols
	cursor    int
	focus     string
	filter    string
	filtering bool
	confirm   bool
	pending   string
	yes       bool
	help      bool
	diff      bool
	diffVP    viewport.Model
	action    Action
	quit      bool
	notice    string
	noticeOK  bool
	fileIdx   int
	Apply     ApplyFunc
}

// New builds a model. rows are newest-first.
func New(rows []Row, header, notice string, color, ascii bool) Model {
	m := Model{
		Rows: rows, Header: header, notice: notice,
		styles: ui.NewStyles(color), sym: ui.Glyphs(ascii),
		focus: "list", Width: 100, Height: 30, yes: true,
	}
	m.cursor = firstSelectable(rows)
	m.diffVP = viewport.New(40, 10)
	return m
}

func firstSelectable(rows []Row) int {
	for i, r := range rows {
		if !r.Gap {
			return i
		}
	}
	return 0
}

func cursorForSeq(rows []Row, seq int) int {
	if seq > 0 {
		for i, r := range rows {
			if !r.Gap && r.Seq == seq {
				return i
			}
		}
	}
	return firstSelectable(rows)
}

// Action returns a confirmed command, or a zero action when the user quit.
func (m Model) Action() Action { return m.action }

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case refreshedMsg:
		m.confirm = false
		m.pending = ""
		m.action = Action{}
		m.diff = false
		if msg.err != nil {
			m.notice = msg.err.Error()
			m.noticeOK = false
		} else {
			m.notice = msg.notice
			m.noticeOK = msg.notice != ""
		}
		if msg.rows != nil {
			m.Rows = msg.rows
			m.cursor = cursorForSeq(m.visible(), msg.seq)
			m.fileIdx = 0
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
		m.diffVP.Width = msg.Width - 2
		m.diffVP.Height = max(1, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m Model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch msg.Type {
		case tea.KeyEsc:
			m.filtering = false
			m.filter = ""
		case tea.KeyEnter:
			m.filtering = false
		case tea.KeyBackspace:
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
			}
		case tea.KeyRunes:
			m.filter += string(msg.Runes)
		}
		return m, nil
	}
	if m.help {
		if msg.String() == "esc" || msg.String() == "q" || msg.String() == "?" {
			m.help = false
		}
		return m, nil
	}
	if m.diff {
		switch msg.String() {
		case "esc", "q":
			m.diff = false
		case "j", "down":
			m.diffVP.ScrollDown(1)
		case "k", "up":
			m.diffVP.ScrollUp(1)
		case " ":
			m.diffVP.ScrollDown(m.diffVP.Height)
		case "n":
			m.stepFile(1)
		case "p":
			m.stepFile(-1)
		}
		return m, nil
	}
	if m.confirm {
		switch msg.String() {
		case "esc", "n", "q":
			m.confirm = false
		case "left", "right", "tab":
			m.yes = !m.yes
		case "y":
			m.yes = true
			return m.confirmDo()
		case "enter":
			if m.yes {
				return m.confirmDo()
			}
			m.confirm = false
		}
		return m, nil
	}
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "g":
		m.cursor = firstSelectable(m.visible())
	case "G":
		vis := m.visible()
		if len(vis) > 0 {
			m.cursor = len(vis) - 1
			for m.cursor > 0 && vis[m.cursor].Gap {
				m.cursor--
			}
		}
	case "enter", "u":
		row, ok := m.selected()
		if ok && !row.Gap {
			if row.Status == "running" {
				m.notice = fmt.Sprintf("#%d is still running (%s). Stop the agent first, then press enter.", row.Seq, row.Agent)
				m.noticeOK = false
				return m, nil
			}
			m.pending = "undo"
			if row.Status == "undone" {
				m.pending = "redo"
			}
			m.confirm = true
			m.yes = true
		}
	case "d":
		if row, ok := m.selected(); ok && !row.Gap {
			m.diff = true
			m.fileIdx = 0
			m.showDiff(row)
		}
	case "r":
		if row, ok := m.selected(); ok && !row.Gap {
			if row.Status == "running" {
				m.notice = fmt.Sprintf("#%d is still running (%s). Stop the agent first, then press enter.", row.Seq, row.Agent)
				m.noticeOK = false
				return m, nil
			}
			m.confirm = true
			m.pending = "rewind"
			m.yes = true
		}
	case "shift+enter":
		if row, ok := m.selected(); ok && row.Status == "running" {
			m.confirm = true
			m.pending = "undo"
			m.yes = true
		}
	case "R":
		seq := 0
		if row, ok := m.selected(); ok {
			seq = row.Seq
		}
		if m.Apply == nil {
			m.action = Action{Kind: "redo", Seq: seq}
			return m, tea.Quit
		}
		return m, m.runApply(Action{Kind: "redo", Seq: seq})
	case "/":
		m.filtering = true
	case "?":
		m.help = true
	case "tab":
		if m.focus == "list" {
			m.focus = "files"
		} else {
			m.focus = "list"
		}
	}
	return m, nil
}

func (m Model) confirmDo() (tea.Model, tea.Cmd) {
	row, ok := m.selected()
	if !ok {
		m.confirm = false
		return m, nil
	}
	kind := m.pending
	if kind == "" {
		kind = "undo"
	}
	action := Action{Kind: kind, Seq: row.Seq}
	if m.Apply == nil {
		m.action = action
		return m, tea.Quit
	}
	m.confirm = false
	return m, m.runApply(action)
}

func (m Model) runApply(action Action) tea.Cmd {
	apply := m.Apply
	seq := action.Seq
	return func() tea.Msg {
		rows, notice, err := apply(action)
		return refreshedMsg{rows: rows, notice: notice, err: err, seq: seq}
	}
}

func (m *Model) showDiff(row Row) {
	parts := fileDiffs(row)
	if len(parts) == 0 {
		m.diffVP.SetContent("  no diff\n")
		return
	}
	if m.fileIdx < 0 {
		m.fileIdx = 0
	}
	if m.fileIdx >= len(parts) {
		m.fileIdx = len(parts) - 1
	}
	m.diffVP.SetContent(parts[m.fileIdx])
}

func (m *Model) stepFile(d int) {
	row, ok := m.selected()
	if !ok {
		return
	}
	n := len(fileDiffs(row))
	if n == 0 {
		return
	}
	m.fileIdx += d
	if m.fileIdx < 0 {
		m.fileIdx = 0
	}
	if m.fileIdx >= n {
		m.fileIdx = n - 1
	}
	m.showDiff(row)
}

func (m *Model) move(d int) {
	vis := m.visible()
	if len(vis) == 0 {
		return
	}
	next := m.cursor + d
	for next >= 0 && next < len(vis) {
		if !vis[next].Gap {
			m.cursor = next
			return
		}
		next += d
	}
}

func (m Model) visible() []Row {
	if m.filter == "" {
		if len(m.Rows) > 50 {
			return m.Rows[:50]
		}
		return m.Rows
	}
	q := strings.ToLower(m.filter)
	var out []Row
	for _, r := range m.Rows {
		blob := strings.ToLower(r.Prompt + " " + r.Agent + " " + r.Preview)
		if strings.Contains(blob, q) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) selected() (Row, bool) {
	vis := m.visible()
	if len(vis) == 0 || m.cursor < 0 || m.cursor >= len(vis) {
		return Row{}, false
	}
	return vis[m.cursor], true
}

// View implements tea.Model.
func (m Model) View() string {
	if len(m.Rows) == 0 {
		return "  No snapshots here yet.\n\n  oops protects this folder once an agent runs in it.\n  Not set up?  oops init        Any other tool?  oops run -- <command>\n"
	}
	if m.help {
		return "  ↑↓/jk select   enter undo   d diff   r rewind to here   / filter   R redo   q quit\n  esc closes this help\n"
	}
	if m.diff {
		row, _ := m.selected()
		title := fmt.Sprintf(" #%d · diff", row.Seq)
		return title + "\n" + strings.Repeat("─", max(8, m.Width)) + "\n" + m.diffVP.View() + "\n  n next file   p prev file   esc back\n"
	}
	body := m.body()
	if m.confirm {
		body = lipgloss.Place(max(m.Width, 20), max(m.Height, 10), lipgloss.Center, lipgloss.Center, body+"\n"+m.card())
	}
	return body
}

func (m Model) body() string {
	var b strings.Builder
	if m.Height >= 15 {
		b.WriteString(m.styles.Bold.Render(" oops") + "  " + m.Header + "\n")
		b.WriteString(m.styles.Muted.Render(strings.Repeat("─", max(8, m.Width))) + "\n")
	}
	// One full-width row, blank until a toast arrives, so the list does not jump.
	b.WriteString(m.noticeLine() + "\n")
	vis := m.visible()
	preview := ""
	if row, ok := m.selected(); ok {
		preview = row.Preview
	}
	leftW, rightW, stacked := paneWidths(m.Width)
	markW := ui.DisplayWidth(" " + m.sym.Selected)
	if markW < 2 {
		markW = 2
	}
	var left, right strings.Builder
	for i, r := range vis {
		mark := strings.Repeat(" ", markW)
		if i == m.cursor && !r.Gap {
			mark = " " + m.sym.Selected
			if pad := markW - ui.DisplayWidth(mark); pad > 0 {
				mark += strings.Repeat(" ", pad)
			}
		}
		line := mark + m.line(r, listLineWidth(leftW, markW))
		if i == m.cursor && !r.Gap {
			line = m.styles.Select.Render(line)
		} else if r.Gap || r.Status == "undone" {
			line = m.styles.Muted.Render(line)
		}
		left.WriteString(line + "\n")
	}
	previewWidth := m.Width
	if !stacked {
		previewWidth = rightW
	}
	if strings.TrimSpace(preview) != "" {
		right.WriteString(fitPreview(preview, previewWidth))
	}
	if stacked {
		b.WriteString(left.String())
		if right.Len() > 0 {
			b.WriteString(right.String())
			b.WriteByte('\n')
		}
	} else {
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left.String(), m.styles.Muted.Render(" │ "), right.String()))
		b.WriteByte('\n')
	}
	b.WriteString(m.styles.Muted.Render(strings.Repeat("─", max(8, m.Width))) + "\n")
	b.WriteString(m.styles.Muted.Render("  ↑↓ select   enter undo   d diff   r rewind to here   R redo   / filter   ? help   q quit"))
	if m.filtering {
		b.WriteString("\n  filter: " + m.filter)
	}
	return b.String()
}

func (m Model) line(r Row, width int) string {
	if r.Gap {
		s := fmt.Sprintf(" %s    %-7s %-11s %s", m.sym.Gap, ui.Pad(r.Agent, 7), ui.Pad(r.When, 11), r.Prompt)
		return ui.Truncate(s, max(10, width))
	}
	files := ui.Plural(r.Files, "file", "files")
	status := ""
	if r.Status == "running" || r.Status == "undone" {
		status = "  " + r.Status
	}
	prefix := fmt.Sprintf(" #%-4d %-7s %-11s ", r.Seq, ui.Pad(r.Agent, 7), ui.Pad(r.When, 11))
	suffix := " " + files + status
	prompt := ""
	remain := width - ui.DisplayWidth(prefix) - ui.DisplayWidth(suffix)
	if r.Prompt != "" && remain > 2 {
		// The prompt column is whatever is left on this line. Quotes cost two
		// columns, and an ellipsis costs one more when the text does not fit.
		inner := remain - 2
		if ui.DisplayWidth(r.Prompt) > inner && inner > 1 {
			inner--
		}
		if inner < 1 {
			inner = 1
		}
		prompt = ui.Quote(r.Prompt, inner)
		for ui.DisplayWidth(prefix+prompt+suffix) > width && inner > 1 {
			inner--
			prompt = ui.Quote(r.Prompt, inner)
		}
	}
	s := prefix + prompt + suffix
	if width > 0 && ui.DisplayWidth(s) > width {
		s = prefix + suffix
		if ui.DisplayWidth(s) > width {
			return ui.Truncate(prefix, width)
		}
	}
	return s
}

// noticeLine is always one terminal row. A single space is not a row: some
// terminals collapse it, and the list then jumps down when the toast appears.
func (m Model) noticeLine() string {
	width := m.Width
	if width < 1 {
		width = 1
	}
	if m.notice == "" {
		return strings.Repeat(" ", width)
	}
	label := "  " + m.sym.Warn + " " + m.notice
	style := m.styles.Warning
	if m.noticeOK {
		label = "  " + m.sym.OK + " " + m.notice
		style = m.styles.Success
	}
	text := style.Render(label)
	line := lipgloss.NewStyle().Width(width).MaxWidth(width).MaxHeight(1).Render(text)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if line == "" {
		line = text
	}
	if pad := width - ui.DisplayWidth(line); pad > 0 && !m.styles.Enabled {
		line += strings.Repeat(" ", pad)
	}
	return line
}

// listLineWidth is the width of the text after the selection mark.
// The prompt column is whatever remains inside that width after the fixed
// turn, agent, time, and file-count columns.
func listLineWidth(left, mark int) int {
	w := left - mark
	if w < 10 {
		return 10
	}
	return w
}

// paneWidths splits the terminal into the list and the preview.
// The preview column is reserved only when the prompt still has room beside
// it. On a 120-column window that column would leave the list short and the
// row partly empty, so the list takes the full width and the preview sits
// underneath. fixed columns are the mark, turn, agent, time, quotes, and file count.
func paneWidths(width int) (left, right int, stacked bool) {
	if width < 80 {
		return width, 0, true
	}
	const (
		sep       = 3
		listFixed = 2 + 27 + 7 + 2 // mark, prefix, " 1 file", quotes
		minPrompt = 72
	)
	right = 34
	if width >= 160 {
		right = 42
	}
	if width-right-sep-listFixed < minPrompt {
		return width, 0, true
	}
	return width - right - sep, right, false
}

func fitPreview(preview string, width int) string {
	if width < 8 {
		width = 8
	}
	var out []string
	for _, line := range strings.Split(preview, "\n") {
		out = append(out, fitPreviewParts(line, width)...)
	}
	return strings.Join(out, "\n")
}

func fitPreviewParts(line string, width int) []string {
	if strings.Contains(line, "ran:") {
		return wrapRan(line, width)
	}
	if label, path, ok := splitPreviewPath(line); ok && strings.Contains(path, ", ") {
		parts := []string{fitPreviewLine(strings.TrimRight(label, " "), width)}
		for _, p := range strings.Split(path, ", ") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			parts = append(parts, fitPreviewLine("  "+p, width))
		}
		return parts
	}
	return []string{fitPreviewLine(line, width)}
}

// wrapRan breaks the prompt onto more lines. A middle ellipsis would hide the end.
func wrapRan(line string, width int) []string {
	if ui.DisplayWidth(line) <= width {
		return []string{line}
	}
	i := strings.Index(line, "ran:")
	prefix := line[:i+len("ran:")]
	if !strings.HasSuffix(prefix, " ") {
		prefix += " "
	}
	if width < ui.DisplayWidth(prefix)+1 {
		width = ui.DisplayWidth(prefix) + 1
	}
	payload := strings.TrimSpace(line[i+len("ran:"):])
	cont := strings.Repeat(" ", ui.DisplayWidth(prefix))
	return wrapWords(prefix, cont, payload, width)
}

func wrapWords(prefix, cont, text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{strings.TrimRight(prefix, " ")}
	}
	var lines []string
	gap := prefix
	var b strings.Builder
	b.WriteString(gap)
	used := ui.DisplayWidth(gap)
	started := false
	flush := func() {
		lines = append(lines, strings.TrimRight(b.String(), " "))
		b.Reset()
		gap = cont
		b.WriteString(gap)
		used = ui.DisplayWidth(gap)
		started = false
	}
	for _, word := range words {
		for word != "" {
			room := width - used
			extra := 0
			if started {
				extra = 1
			}
			if ui.DisplayWidth(word)+extra <= room {
				if started {
					b.WriteByte(' ')
					used++
				}
				b.WriteString(word)
				used += ui.DisplayWidth(word)
				started = true
				word = ""
				continue
			}
			if started || room < 1 {
				flush()
				continue
			}
			part := takeCols(word, room)
			if part == "" {
				return lines
			}
			b.WriteString(part)
			used += ui.DisplayWidth(part)
			word = word[len(part):]
			started = true
			if word != "" {
				flush()
			}
		}
	}
	if started {
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	return lines
}

func takeCols(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	w := 0
	end := 0
	for i, r := range s {
		rw := ui.DisplayWidth(string(r))
		if w+rw > n {
			break
		}
		w += rw
		end = i + utf8.RuneLen(r)
	}
	if end == 0 {
		_, size := utf8.DecodeRuneInString(s)
		if size <= 0 {
			return ""
		}
		return s[:size]
	}
	return s[:end]
}

// fitPreviewLine middle-truncates a path after the count label.
// The count ("5 files") is never part of the truncated span.
func fitPreviewLine(line string, width int) string {
	if ui.DisplayWidth(line) <= width {
		return line
	}
	label, path, ok := splitPreviewPath(line)
	if !ok {
		return ui.MiddleTruncate(line, width)
	}
	room := width - ui.DisplayWidth(label)
	if room < 1 {
		return label
	}
	return label + ui.MiddleTruncate(path, room)
}

func splitPreviewPath(line string) (label, path string, ok bool) {
	i := strings.LastIndex(line, "   ")
	if i < 0 {
		return "", "", false
	}
	path = line[i+3:]
	label = line[:i+3]
	if path == "" || !strings.Contains(label, " file") {
		return "", "", false
	}
	return label, path, true
}

func fileDiffs(row Row) []string {
	parts := row.Diffs
	if len(parts) == 0 && row.Diff != "" {
		parts = []string{row.Diff}
	}
	var out []string
	for _, p := range parts {
		if dirDiff(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func dirDiff(s string) bool {
	if strings.Contains(s, "@@") {
		return false
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "/") {
			return true
		}
	}
	return false
}

func (m Model) card() string {
	row, ok := m.selected()
	if !ok {
		return ""
	}
	label := "Undo"
	title := fmt.Sprintf("Undo turn #%d?", row.Seq)
	switch {
	case m.pending == "rewind":
		label = "Rewind"
		title = fmt.Sprintf("Rewind to before #%d?", row.Seq)
	case m.pending == "redo" || row.Status == "undone":
		label = "Redo"
		title = fmt.Sprintf("Redo turn #%d?", row.Seq)
	}
	undo := "[ " + label + " ]"
	cancel := "Cancel"
	if m.yes {
		undo = m.styles.Accent.Render(undo)
		cancel = m.styles.Muted.Render(cancel)
	} else {
		cancel = m.styles.Accent.Render(cancel)
		undo = m.styles.Muted.Render(undo)
	}
	inner := fmt.Sprintf("  %s\n  %s · %s\n\n  %s\n  Your changes after this turn are kept.\n\n        %s    %s  ",
		title, ui.Quote(row.Prompt, 40), row.Agent, ui.Plural(row.Files, "file", "files"), undo, cancel)
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.Colors.Muted).Render(inner)
	return box
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
