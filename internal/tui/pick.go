package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Pick is a small multi-select used by oops init.
type Pick struct {
	title     string
	labels    []string
	On        []bool
	cursor    int
	done      bool
	cancelled bool
}

// NewPick starts a multi-select. on is the initial selection.
func NewPick(title string, labels []string, on []bool) Pick {
	if len(on) != len(labels) {
		on = make([]bool, len(labels))
		for i := range on {
			on[i] = true
		}
	}
	return Pick{title: title, labels: labels, On: append([]bool(nil), on...)}
}

// Cancelled reports that the user pressed esc.
func (m Pick) Cancelled() bool { return m.cancelled }

// Init implements tea.Model.
func (m Pick) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Pick) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "ctrl+c":
		m.cancelled = true
		m.done = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor+1 < len(m.labels) {
			m.cursor++
		}
	case " ":
		if len(m.On) > 0 {
			m.On[m.cursor] = !m.On[m.cursor]
		}
	case "enter":
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

// View implements tea.Model.
func (m Pick) View() string {
	if m.done {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  %s\n\n", m.title)
	for i, label := range m.labels {
		box := "[ ]"
		if i < len(m.On) && m.On[i] {
			box = "[x]"
		}
		mark := "  "
		if i == m.cursor {
			mark = "> "
		}
		fmt.Fprintf(&b, "  %s%s %s\n", mark, box, label)
	}
	return b.String()
}
