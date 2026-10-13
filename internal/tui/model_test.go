package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleRows() []Row {
	return []Row{
		{Seq: 2, Agent: "claude", When: "1m ago", Prompt: "edit b", Files: 1, Status: "done", Preview: "b.txt", Diff: "changed b", Diffs: []string{"diff b.txt\n+b"}},
		{Gap: true, Agent: "you", When: "2m ago", Prompt: "1 file changed outside agents"},
		{Seq: 1, Agent: "codex", When: "3m ago", Prompt: "edit a", Files: 1, Status: "done", Preview: "a.txt", Diffs: []string{"diff a.txt\n+a"}},
	}
}

func TestTimelineKeys(t *testing.T) {
	start := time.Now()
	m := New(sampleRows(), "proj", "", false, true)
	first := m.View()
	if time.Since(start) > 150*time.Millisecond {
		t.Fatalf("first view took %s", time.Since(start))
	}
	if !strings.Contains(first, "edit b") {
		t.Fatalf("first view:\n%s", first)
	}
	if m.Action().Kind != "" {
		t.Fatal("action set before confirm")
	}

	var model tea.Model = m
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = model.(Model)
	if !m.diff || !strings.Contains(m.View(), "diff b.txt") {
		t.Fatalf("diff view:\n%s", m.View())
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if !m.confirm || m.Action().Kind != "" {
		t.Fatalf("enter should confirm, not undo: confirm=%v action=%+v", m.confirm, m.Action())
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if m.Action().Kind != "undo" || m.Action().Seq != 2 {
		t.Fatalf("confirmed action %+v", m.Action())
	}
}

func TestTimelineNarrowAndEmpty(t *testing.T) {
	m := New(sampleRows(), "proj", "", false, true)
	m.Width = 40
	view := m.View()
	if strings.Contains(view, " │ ") {
		t.Fatalf("narrow view is two columns:\n%s", view)
	}
	empty := New(nil, "", "", false, true)
	text := empty.View()
	if !strings.Contains(text, "No snapshots here yet.") || !strings.Contains(text, "oops init") {
		t.Fatalf("empty:\n%s", text)
	}
}

func TestTimelineMoveAndRewind(t *testing.T) {
	m := New(sampleRows(), "proj", "", false, true)
	var model tea.Model = m
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = model.(Model)
	row, ok := m.selected()
	if !ok || row.Seq != 1 {
		t.Fatalf("selected %+v", row)
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = model.(Model)
	if !m.confirm || m.pending != "rewind" {
		t.Fatalf("rewind confirm pending=%q confirm=%v", m.pending, m.confirm)
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)
	if m.confirm || m.Action().Kind != "" {
		t.Fatal("esc should cancel")
	}
}
