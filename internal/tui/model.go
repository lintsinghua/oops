// Package tui is the Bubble Tea timeline.
package tui

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/lintsinghua/oops/internal/ui"
)

// Row is one timeline line prepared by the CLI.
type Row struct {
	Gap        bool
	Seq        int
	Agent      string
	When       string
	Prompt     string
	Files      int
	Insertions int
	Deletions  int
	Status     string
	Preview    string
	Plan       string
	Diff       string
	Diffs      []string
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
	// The diff viewport renders through the default lipgloss profile. A
	// non-terminal stdout would strip the added and removed colors.
	if color && os.Getenv("NO_COLOR") == "" {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}
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
			// A partial undo or redo says "except" and uses the warning glyph.
			m.noticeOK = msg.notice != "" && !strings.Contains(msg.notice, "except")
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
			if row.Status == "undone" || row.Status == "kept" {
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

// paintDiff colors added lines with Success and removed lines with Danger.
// Headers (+++) and context lines stay plain. NO_COLOR leaves the text as-is.
func paintDiff(text string, st ui.Styles) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trim := strings.TrimLeft(line, " ")
		switch {
		case strings.HasPrefix(trim, "+") && !strings.HasPrefix(trim, "+++"):
			lines[i] = st.Success.Render(line)
		case strings.HasPrefix(trim, "-") && !strings.HasPrefix(trim, "---") && !strings.HasPrefix(trim, "--"):
			lines[i] = st.Danger.Render(line)
		}
	}
	return strings.Join(lines, "\n")
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
	m.diffVP.SetContent(paintDiff(parts[m.fileIdx], m.styles))
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
		return m.overlayCard(body)
	}
	return body
}

// overlayCard keeps the title on screen. The card is centered in the rows
// under the title and clipped to the height that remains.
func (m Model) overlayCard(body string) string {
	height := m.Height
	if height < 1 {
		height = 24
	}
	width := m.Width
	if width < 1 {
		width = 80
	}
	lines := strings.Split(body, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	cardLines := strings.Split(m.card(), "\n")
	top := 0
	if strings.Contains(lines[0], "oops") {
		top = 1
		if len(lines) > 1 && strings.Contains(lines[1], "─") {
			top = 2
		}
	}
	room := height - top
	if room < 1 {
		room = height
		top = 0
	}
	if len(cardLines) > room {
		cardLines = cardLines[:room]
	}
	start := top
	if gap := room - len(cardLines); gap > 0 {
		start = top + gap/2
	}
	for i, line := range cardLines {
		idx := start + i
		if idx < 0 || idx >= len(lines) {
			continue
		}
		lines[idx] = centerLine(line, width)
	}
	return strings.Join(lines[:height], "\n")
}

func centerLine(line string, width int) string {
	w := ui.DisplayWidth(line)
	if w >= width {
		return line
	}
	pad := width - w
	left := pad / 2
	return strings.Repeat(" ", left) + line + strings.Repeat(" ", pad-left)
}

func (m Model) body() string {
	width := m.Width
	if width < 1 {
		width = 80
	}
	height := m.Height
	if height < 1 {
		height = 24
	}
	rule := m.styles.Muted.Render(strings.Repeat("─", max(8, width)))
	var head, foot []string
	if height >= 15 {
		head = append(head, m.styles.Bold.Render(" oops")+"  "+m.Header)
		head = append(head, rule)
	}
	// One full-width row, blank until a toast arrives, so the list does not jump.
	head = append(head, m.noticeLine())
	foot = append(foot, rule)
	foot = append(foot, m.styles.Muted.Render(fitFooter(width)))
	if m.filtering {
		foot = append(foot, "  filter: "+m.filter)
	}
	budget := height - len(head) - len(foot)
	if budget < 1 {
		budget = 1
	}
	vis := m.visible()
	preview := ""
	if row, ok := m.selected(); ok {
		preview = row.Preview
	}
	leftW, rightW, stacked := paneWidths(width)
	markW := ui.DisplayWidth(" " + m.sym.Selected)
	if markW < 2 {
		markW = 2
	}
	cursor := m.cursor
	if cursor < 0 || cursor >= len(vis) {
		cursor = 0
	}
	var mid []string
	if stacked {
		mid = m.stackedLines(vis, cursor, preview, leftW, markW, width, budget)
	} else {
		mid = m.sideLines(vis, cursor, preview, leftW, rightW, markW, budget)
	}
	if len(mid) > budget {
		mid = mid[:budget]
	}
	all := append(append(head, mid...), foot...)
	if len(all) > height {
		all = all[:height]
	}
	return strings.Join(all, "\n")
}

// stackedLines draws the list, a rule, then a preview clipped to the rows left.
func (m Model) stackedLines(vis []Row, cursor int, preview string, leftW, markW, width, budget int) []string {
	fitted := ensureBlankBeforeRan(previewLines(preview, width))
	listBudget := budget
	previewBudget := 0
	if len(fitted) > 0 {
		switch {
		case len(vis)+1+len(fitted) <= budget:
			listBudget = len(vis)
			previewBudget = len(fitted)
		case len(vis)+1 < budget:
			listBudget = len(vis)
			previewBudget = budget - len(vis) - 1
		default:
			previewBudget = 1
			if budget >= 8 {
				previewBudget = min(4, budget/3)
			}
			if previewBudget >= budget {
				previewBudget = max(0, budget-1)
			}
			listBudget = budget - 1 - previewBudget
		}
		if listBudget < 1 {
			listBudget = min(1, budget)
			previewBudget = max(0, budget-listBudget-1)
		}
	}
	shown, highlight := windowRows(vis, cursor, listBudget)
	out := m.renderList(shown, highlight, leftW, markW, width)
	if previewBudget > 0 && len(fitted) > 0 {
		out = append(out, m.styles.Muted.Render(strings.Repeat("─", max(8, width))))
		out = append(out, clipPreview(fitted, previewBudget)...)
	}
	return out
}

// sideLines keeps the preview beside the list. The divider is one row per
// line of the taller pane, so it reaches the bottom of that block.
func (m Model) sideLines(vis []Row, cursor int, preview string, leftW, rightW, markW, budget int) []string {
	shown, highlight := windowRows(vis, cursor, budget)
	list := m.renderList(shown, highlight, leftW, markW, leftW)
	right := clipPreview(previewLines(preview, rightW), budget)
	n := max(len(list), len(right))
	if n > budget {
		n = budget
	}
	if len(list) > n {
		list = list[:n]
	}
	if len(right) > n {
		right = clipPreview(right, n)
	}
	for len(list) < n {
		list = append(list, padDisplay("", leftW))
	}
	for len(right) < n {
		right = append(right, "")
	}
	sep := make([]string, n)
	bar := m.styles.Muted.Render(" │ ")
	for i := range sep {
		sep[i] = bar
	}
	if n == 0 {
		return nil
	}
	joined := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(list, "\n"), strings.Join(sep, "\n"), strings.Join(right, "\n"))
	return strings.Split(joined, "\n")
}

func (m Model) renderList(rows []Row, highlight, lineW, markW, paneW int) []string {
	out := make([]string, 0, len(rows))
	for i, r := range rows {
		mark := strings.Repeat(" ", markW)
		if i == highlight && !r.Gap {
			mark = " " + m.sym.Selected
			if pad := markW - ui.DisplayWidth(mark); pad > 0 {
				mark += strings.Repeat(" ", pad)
			}
		}
		line := padDisplay(mark+m.line(r, listLineWidth(lineW, markW), statusWidth(rows)), paneW)
		if i == highlight && !r.Gap {
			line = m.styles.Select.Render(line)
		} else if r.Gap {
			line = m.styles.Muted.Render(line)
		}
		out = append(out, line)
	}
	return out
}

const (
	fileColWidth   = 10
	statusColWidth = 7
	statusGap      = 1
)

func (m Model) line(r Row, width, statusW int) string {
	if statusW < statusColWidth {
		statusW = statusColWidth
	}
	tailW := fileColWidth + statusGap + statusW
	if r.Gap {
		s := fmt.Sprintf(" %s    %-7s %-11s %s", m.sym.Gap, ui.Pad(r.Agent, 7), ui.Pad(r.When, 11), r.Prompt)
		room := width - tailW
		if room < 1 {
			return ui.Truncate(s, max(1, width))
		}
		return padDisplay(ui.Truncate(s, room), room) + rightCol("", fileColWidth) + strings.Repeat(" ", statusGap) + rightCol("", statusW)
	}
	agent := ui.Paint(m.styles.Muted, ui.Pad(r.Agent, 7))
	when := ui.Paint(m.styles.Muted, ui.Pad(r.When, 11))
	prefix := fmt.Sprintf(" #%-4d %s %s ", r.Seq, agent, when)
	file := rightCol(ui.Plural(r.Files, "file", "files"), fileColWidth)
	delta := m.fileDelta(r)
	statusText := rowStatus(r.Status)
	if statusText == "undone" || statusText == "partial" {
		statusText = ui.Paint(m.styles.Warning, statusText)
	}
	if ui.DisplayWidth(statusText) > statusW {
		statusText = ui.Truncate(statusText, statusW)
	}
	status := rightCol(statusText, statusW)
	tail := file + delta + strings.Repeat(" ", statusGap) + status
	promptWidth := width - ui.DisplayWidth(prefix) - ui.DisplayWidth(tail)
	if promptWidth < 0 {
		promptWidth = 0
	}
	return prefix + promptField(r.Prompt, promptWidth) + tail
}

// promptField quotes and truncates the prompt to width, then pads so the
// file-count and status columns stay on the right.
func promptField(prompt string, width int) string {
	if width <= 0 {
		return ""
	}
	if prompt == "" {
		return strings.Repeat(" ", width)
	}
	inner := width - 2
	if inner < 1 {
		return padDisplay(ui.Truncate(prompt, width), width)
	}
	if ui.DisplayWidth(prompt) > inner && inner > 1 {
		inner--
	}
	quoted := ui.Quote(prompt, inner)
	for ui.DisplayWidth(quoted) > width && inner > 1 {
		inner--
		quoted = ui.Quote(prompt, inner)
	}
	if ui.DisplayWidth(quoted) > width {
		quoted = ui.Truncate(quoted, width)
	}
	return padDisplay(quoted, width)
}

func (m Model) fileDelta(r Row) string {
	if r.Insertions == 0 && r.Deletions == 0 {
		return ""
	}
	plus := ui.Paint(m.styles.Success, fmt.Sprintf("+%d", r.Insertions))
	minus := ui.Paint(m.styles.Danger, fmt.Sprintf("%s%d", m.sym.Minus, r.Deletions))
	return " " + plus + " " + minus
}

func rightCol(s string, width int) string {
	if ui.DisplayWidth(s) > width {
		return ui.Truncate(s, width)
	}
	return strings.Repeat(" ", width-ui.DisplayWidth(s)) + s
}

func padDisplay(s string, width int) string {
	w := ui.DisplayWidth(s)
	if w > width {
		return ui.Truncate(s, width)
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

func windowRows(rows []Row, cursor, limit int) (shown []Row, highlight int) {
	if limit < 1 || len(rows) == 0 {
		return nil, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(rows) {
		cursor = len(rows) - 1
	}
	if len(rows) <= limit {
		return rows, cursor
	}
	start := cursor - limit/2
	if start < 0 {
		start = 0
	}
	if start+limit > len(rows) {
		start = len(rows) - limit
	}
	return rows[start : start+limit], cursor - start
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
		sep        = 3
		mark       = 2
		prefix     = 27
		quotes     = 2
		minPrompt  = 72
		longPrompt = 96
		minPreview = 28
		maxPreview = 42
	)
	fixed := mark + prefix + fileColWidth + statusGap + statusColWidth + quotes
	if width-sep-minPreview-fixed < minPrompt {
		return width, 0, true
	}
	// The preview stays narrow until the prompt has a long column. Extra
	// width goes to the prompt first, then to the preview.
	right = minPreview
	promptText := width - sep - right - fixed
	if promptText > longPrompt {
		grow := promptText - longPrompt
		if minPreview+grow > maxPreview {
			grow = maxPreview - minPreview
		}
		right = minPreview + grow
	}
	return width - right - sep, right, false
}

// fitFooter keeps ? help and q quit, and drops lower-priority hints until the
// line fits. The drop order is rewind, filter, diff, redo, enter, then select.
func fitFooter(width int) string {
	optional := []string{"↑↓ select", "enter undo", "R redo", "d diff", "/ filter", "r rewind to here"}
	required := []string{"? help", "q quit"}
	for {
		parts := append(append([]string{}, optional...), required...)
		line := "  " + strings.Join(parts, "   ")
		if width < 1 || ui.DisplayWidth(line) <= width || len(optional) == 0 {
			if width > 0 && ui.DisplayWidth(line) > width {
				return ui.Truncate(line, width)
			}
			return line
		}
		optional = optional[:len(optional)-1]
	}
}

func statusWidth([]Row) int {
	return statusColWidth
}

func rowStatus(status string) string {
	switch status {
	case "", "done", "partial":
		return ""
	case "kept":
		return "partial"
	default:
		return status
	}
}

func ensureBlankBeforeRan(lines []string) []string {
	for i, line := range lines {
		if !strings.Contains(line, "ran:") {
			continue
		}
		if i > 0 && strings.TrimSpace(lines[i-1]) == "" {
			return lines
		}
		out := append([]string{}, lines[:i]...)
		out = append(out, "")
		return append(out, lines[i:]...)
	}
	return lines
}

func previewLines(preview string, width int) []string {
	if strings.TrimSpace(preview) == "" {
		return nil
	}
	text := normalizePreview(fitPreview(preview, width))
	lines := strings.Split(text, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// normalizePreview gives the pane one indent for prose and one for paths.
func normalizePreview(preview string) string {
	lines := strings.Split(preview, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			out = append(out, "")
			continue
		}
		indent := "  "
		if isPreviewPath(line) || isMoreLine(line) {
			indent = "    "
		}
		out = append(out, indent+trim)
	}
	return strings.Join(out, "\n")
}

// clipPreview keeps the header and shortens the file list so the block fits
// in max lines. A tight row becomes `src/api.ts … +3 more`.
func clipPreview(lines []string, max int) []string {
	if max <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) <= max {
		return lines
	}
	start, end := pathSpan(lines)
	if start < 0 {
		return lines[:max]
	}
	head := append([]string(nil), lines[:start]...)
	mid := append([]string(nil), lines[start:end]...)
	tail := append([]string(nil), lines[end:]...)
	extra := 0
	if len(mid) > 0 {
		if n, ok := moreCount(mid[len(mid)-1]); ok {
			extra = n
			mid = mid[:len(mid)-1]
		}
	}
	// Keep the ran line, with the blank line that separates it from the paths.
	var keptTail []string
	for _, line := range tail {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "ran:") {
			keptTail = append(keptTail, "")
		}
		keptTail = append(keptTail, line)
		break
	}
	tail = keptTail
	for len(head)+len(tail) >= max && len(tail) > 0 {
		tail = tail[:len(tail)-1]
	}
	for len(head)+len(tail) >= max && len(head) > 1 {
		// Drop a blank header line before dropping the title.
		dropped := false
		for i := len(head) - 1; i > 0; i-- {
			if strings.TrimSpace(head[i]) == "" {
				head = append(head[:i], head[i+1:]...)
				dropped = true
				break
			}
		}
		if !dropped {
			break
		}
	}
	room := max - len(head) - len(tail)
	if room < 1 {
		if len(head) >= max {
			return head[:max]
		}
		out := append(head, tail...)
		if len(out) > max {
			return out[:max]
		}
		return out
	}
	if extra == 0 && len(mid) <= room {
		return append(append(head, mid...), tail...)
	}
	if room == 1 {
		k := extra
		if len(mid) > 1 {
			k += len(mid) - 1
		} else if len(mid) == 0 {
			k = extra
		}
		return append(append(head, compactPaths(mid, k)), tail...)
	}
	keep := room - 1
	if keep > len(mid) {
		keep = len(mid)
	}
	k := extra + len(mid) - keep
	// "+1 more" would spend a row hiding the one file that row could show.
	if k == 1 && keep < len(mid) {
		keep++
		k = extra + len(mid) - keep
	}
	kept := append([]string(nil), mid[:keep]...)
	if k > 0 {
		kept = append(kept, moreLineText(k, mid))
	}
	out := append(append(head, kept...), tail...)
	if len(out) > max {
		return out[:max]
	}
	return out
}

func compactPaths(mid []string, extra int) string {
	if len(mid) == 0 {
		return fmt.Sprintf("    +%d more", extra)
	}
	path := strings.TrimSpace(mid[0])
	if extra <= 0 {
		return "    " + path
	}
	return fmt.Sprintf("    %s … +%d more", path, extra)
}

func moreLineText(k int, mid []string) string {
	indent := "    "
	if len(mid) > 0 {
		line := mid[0]
		if i := len(line) - len(strings.TrimLeft(line, " ")); i > 0 {
			indent = line[:i]
		}
	}
	return indent + fmt.Sprintf("+%d more", k)
}

func pathSpan(lines []string) (start, end int) {
	start = -1
	for i, line := range lines {
		if isPreviewPath(line) || isMoreLine(line) {
			if start < 0 {
				start = i
			}
			end = i + 1
			continue
		}
		if start >= 0 {
			return start, end
		}
	}
	return start, end
}

func isMoreLine(line string) bool {
	_, ok := moreCount(line)
	return ok
}

func moreCount(line string) (int, bool) {
	trim := strings.TrimSpace(line)
	if !strings.HasPrefix(trim, "+") || !strings.HasSuffix(trim, " more") {
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(trim, "+%d more", &n); err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func isPreviewPath(line string) bool {
	trim := strings.TrimSpace(line)
	if trim == "" || isMoreLine(line) {
		return false
	}
	if strings.Contains(line, "ran:") || strings.Contains(trim, " file") || strings.Contains(trim, "would") || strings.Contains(trim, "nothing") || strings.Contains(trim, "undone") || strings.Contains(trim, "re-apply") {
		return false
	}
	if !strings.HasPrefix(line, " ") {
		return false
	}
	if strings.Contains(trim, " ") && !strings.Contains(trim, "/") && !strings.Contains(trim, "…") {
		return false
	}
	return true
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
	case m.pending == "redo" || row.Status == "undone" || row.Status == "kept":
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
	count := ui.Plural(row.Files, "file", "files")
	if m.pending != "rewind" && row.Plan != "" {
		count = row.Plan
	}
	inner := fmt.Sprintf("  %s\n  %s · %s\n\n  %s\n  Your changes after this turn are kept.\n\n        %s    %s  ",
		title, ui.Quote(row.Prompt, 40), row.Agent, count, undo, cancel)
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.Colors.Muted).Render(inner)
	return box
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
