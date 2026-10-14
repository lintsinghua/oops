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

func TestTimelineUndoneMarker(t *testing.T) {
	rows := sampleRows()
	rows[0].Status = "undone"
	rows[0].Files = 1
	m := New(rows, "proj", "", false, true)
	m.Width = 120
	m.Height = 24
	view := m.View()
	if !strings.Contains(view, "undone") || !strings.Contains(view, "1 file") {
		t.Fatalf("marker missing:\n%s", view)
	}
	var model tea.Model = m
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if m.pending != "redo" || !strings.Contains(m.View(), "Redo turn #2?") {
		t.Fatalf("pending %q\n%s", m.pending, m.View())
	}
}

func TestTimelineDiffSkipsDirectory(t *testing.T) {
	rows := []Row{{
		Seq: 2, Agent: "claude", When: "1m ago", Prompt: "edit", Files: 1, Status: "done",
		Diffs: []string{"restore sub/\n", "diff a.txt\n@@ -1 +1 @@\n+a\n"},
	}}
	m := New(rows, "proj", "", false, true)
	var model tea.Model = m
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = model.(Model)
	view := m.View()
	if strings.Contains(view, "restore sub/") || !strings.Contains(view, "diff a.txt") {
		t.Fatalf("view:\n%s", view)
	}
}

func TestTimelineRefreshInPlace(t *testing.T) {
	m := New(sampleRows(), "proj", "", false, true)
	m.Apply = func(a Action) ([]Row, string, error) {
		if a.Kind != "undo" || a.Seq != 2 {
			t.Fatalf("action %+v", a)
		}
		next := sampleRows()
		next[0].Status = "undone"
		return next, "Undid #2", nil
	}
	var model tea.Model = m
	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("first enter should only confirm")
	}
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected apply")
	}
	model, cmd = model.Update(cmd())
	if cmd != nil {
		t.Fatal("refresh should stay open")
	}
	m = model.(Model)
	if m.quit || m.confirm || m.Action().Kind != "" {
		t.Fatalf("quit=%v confirm=%v action=%+v", m.quit, m.confirm, m.Action())
	}
	view := m.View()
	if !strings.Contains(view, "undone") || !strings.Contains(view, "Undid #2") {
		t.Fatalf("view:\n%s", view)
	}
}

func TestTimelinePromptUsesRemainingWidth(t *testing.T) {
	prompt := "please refactor the authentication middleware and add tests for the session store"
	path := "internal/very/long/path/that/keeps/going/until/the/filename/handler.go"
	rows := []Row{{
		Seq: 3, Agent: "claude", When: "1 min ago", Prompt: prompt, Files: 1, Status: "undone",
		Preview: "  revert  " + path,
	}}
	m := New(rows, "proj", "", false, true)
	m.Width = 120
	m.Height = 24
	view := m.View()
	if !strings.Contains(view, "1 file") || !strings.Contains(view, "undone") {
		t.Fatalf("columns clipped:\n%s", view)
	}
	if !strings.Contains(view, "please refactor the auth") {
		t.Fatalf("prompt too short:\n%s", view)
	}
	left, right, _ := strings.Cut(view, " │ ")
	if strings.Count(left, "…") > 1 {
		t.Fatalf("extra ellipsis column:\n%s", left)
	}
	if !strings.Contains(right, "handler.go") || strings.Contains(right, path) {
		t.Fatalf("path:\n%s", right)
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
