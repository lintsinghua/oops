// Package tui is the Bubble Tea timeline.
package tui

import (
	"fmt"
	"strings"

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
		} else {
			m.notice = msg.notice
		}
		if msg.rows != nil {
			m.Rows = msg.rows
			m.cursor = firstSelectable(m.visible())
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
		if m.Apply == nil {
			m.action = Action{Kind: "redo"}
			return m, tea.Quit
		}
		return m, m.runApply(Action{Kind: "redo"})
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
	return func() tea.Msg {
		rows, notice, err := apply(action)
		return refreshedMsg{rows: rows, notice: notice, err: err}
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
	if m.notice != "" {
		b.WriteString(m.styles.Warning.Render("  "+m.sym.Warn+" "+m.notice) + "\n")
	}
	vis := m.visible()
	leftW, rightW, narrow := paneWidths(m.Width)
	var left, right strings.Builder
	for i, r := range vis {
		mark := "  "
		if i == m.cursor && !r.Gap {
			mark = " " + m.sym.Selected
		}
		line := mark + m.line(r, leftW-2)
		if i == m.cursor && !r.Gap {
			line = m.styles.Select.Render(line)
		} else if r.Gap || r.Status == "undone" {
			line = m.styles.Muted.Render(line)
		}
		left.WriteString(line + "\n")
	}
	if row, ok := m.selected(); ok {
		right.WriteString(fitPreview(row.Preview, rightW))
	}
	if narrow {
		b.WriteString(left.String())
	} else {
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left.String(), m.styles.Muted.Render(" │ "), right.String()))
		b.WriteByte('\n')
	}
	b.WriteString(m.styles.Muted.Render(strings.Repeat("─", max(8, m.Width))) + "\n")
	b.WriteString(m.styles.Muted.Render("  ↑↓ select   enter undo   d diff   r rewind to here   / filter   ? help   q quit"))
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
	if r.Prompt != "" {
		inner := width - ui.DisplayWidth(prefix) - ui.DisplayWidth(suffix) - 2
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
		return ui.Truncate(s, width)
	}
	return s
}

func paneWidths(width int) (left, right int, narrow bool) {
	if width < 80 {
		return width, 0, true
	}
	right = width * 34 / 100
	if right > 52 {
		right = 52
	}
	if right < 30 {
		right = 30
	}
	left = width - right - 3
	if left < 48 {
		return width, 0, true
	}
	return left, right, false
}

func fitPreview(preview string, width int) string {
	if width < 8 {
		width = 8
	}
	lines := strings.Split(preview, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ui.MiddleTruncate(line, width))
	}
	return b.String()
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
