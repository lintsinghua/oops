package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lintsinghua/oops/internal/ui"
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
	if strings.Contains(view, "! Undid") || !strings.Contains(view, "ok Undid") {
		t.Fatalf("success toast:\n%s", view)
	}
	if !strings.Contains(view, "R redo") {
		t.Fatalf("footer:\n%s", view)
	}
}

func TestTimelineSuccessMark(t *testing.T) {
	m := New(sampleRows(), "proj", "", false, false)
	m.notice = "Undid #2"
	m.noticeOK = true
	m.Width = 80
	m.Height = 20
	view := m.View()
	if !strings.Contains(view, "✓ Undid #2") || strings.Contains(view, "! Undid") {
		t.Fatalf("toast:\n%s", view)
	}
	if !strings.Contains(view, "R redo") {
		t.Fatalf("footer:\n%s", view)
	}
}

func TestTimelinePromptUsesRemainingWidth(t *testing.T) {
	prompt := "please refactor the authentication middleware and add tests for the session store"
	path := "internal/very/long/path/that/keeps/going/until/the/filename/handler.go"
	rows := []Row{{
		Seq: 3, Agent: "claude", When: "1 min ago", Prompt: prompt, Files: 1, Status: "done",
		Preview: "  revert  " + path,
	}}
	m := New(rows, "proj", "", false, true)
	m.Width = 120
	m.Height = 24
	view := m.View()
	if !strings.Contains(view, "1 file") {
		t.Fatalf("columns clipped:\n%s", view)
	}
	if !strings.Contains(view, "please refactor the authentication") {
		t.Fatalf("prompt too short:\n%s", view)
	}
	if strings.Contains(view, `"`+prompt+`"`) {
		t.Fatalf("prompt was not truncated to the remaining column:\n%s", view)
	}
	if !strings.Contains(view, "handler.go") {
		t.Fatalf("path:\n%s", view)
	}
	wide := New(rows, "proj", "", false, true)
	wide.Width = 170
	wide.Height = 24
	wideView := wide.View()
	if !strings.Contains(wideView, " │ ") || !strings.Contains(wideView, "handler.go") {
		t.Fatalf("wide preview column:\n%s", wideView)
	}
}

func TestTimelineStaysOnActedTurn(t *testing.T) {
	m := New(sampleRows(), "proj", "", false, true)
	m.Width = 120
	m.Height = 24
	m.Apply = func(a Action) ([]Row, string, error) {
		if a.Kind != "undo" || a.Seq != 1 {
			t.Fatalf("action %+v", a)
		}
		next := sampleRows()
		next[2].Status = "undone"
		next[2].Preview = "  #1 is already undone.\n  Redo #1 would re-apply:\n\n    add      1 file   a.txt\n"
		return next, "Undid #1", nil
	}
	var model tea.Model = m
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("first enter confirms")
	}
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected apply")
	}
	model, _ = model.Update(cmd())
	m = model.(Model)
	row, ok := m.selected()
	if !ok || row.Seq != 1 {
		t.Fatalf("cursor %+v", row)
	}
	view := m.View()
	if !strings.Contains(view, "Redo #1 would re-apply:") || !strings.Contains(view, "a.txt") {
		t.Fatalf("preview:\n%s", view)
	}
	if strings.Contains(view, "b.txt") {
		t.Fatalf("right pane left the acted-on turn:\n%s", view)
	}
}

func TestTimelinePathCountStaysWhole(t *testing.T) {
	prompt := "please refactor the authentication middleware and add tests for the session store"
	rows := []Row{{
		Seq: 4, Agent: "claude", When: "1 min ago", Prompt: prompt, Files: 5, Status: "done",
		Preview: "  Undo #4 would:\n\n    revert   5 files   web/src/styles/dark.css\n",
	}}
	m := New(rows, "proj", "", false, true)
	m.Width = 120
	m.Height = 24
	view := m.View()
	var preview string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "revert") {
			preview = line
			break
		}
	}
	if preview == "" || !strings.Contains(preview, "5 files") {
		t.Fatalf("count clipped:\n%s\nview:\n%s", preview, view)
	}
	if strings.Contains(preview, "5 fi…") || strings.Contains(preview, "5 fi...") {
		t.Fatalf("count clipped:\n%s\nview:\n%s", preview, view)
	}
	if !strings.Contains(preview, ".css") {
		t.Fatalf("path:\n%s", preview)
	}
	if !strings.Contains(view, "please refactor the auth") || !strings.Contains(view, "5 files") {
		t.Fatalf("prompt column:\n%s", view)
	}
}

func TestTimelineNoticeReservesRow(t *testing.T) {
	m := New(sampleRows(), "proj", "", false, true)
	m.Width = 100
	m.Height = 24
	before := m.View()
	without := lineNo(before, "edit b")
	m.notice = "Undid #2"
	m.noticeOK = true
	after := m.View()
	with := lineNo(after, "edit b")
	if without < 1 || without != with {
		t.Fatalf("list moved from line %d to %d", without, with)
	}
	blank := strings.Split(before, "\n")[without-1]
	toast := strings.Split(after, "\n")[with-1]
	if ui.DisplayWidth(blank) != m.Width || strings.TrimSpace(blank) != "" {
		t.Fatalf("blank toast row %q width %d", blank, ui.DisplayWidth(blank))
	}
	if ui.DisplayWidth(toast) != m.Width || !strings.Contains(toast, "Undid #2") {
		t.Fatalf("toast row %q width %d", toast, ui.DisplayWidth(toast))
	}
}

func TestTimelineLayoutAt120(t *testing.T) {
	prompt := "please refactor the authentication middleware and add tests for the session store"[:70]
	ranText := strings.Repeat("alpha ", 24) + "omega"
	rows := []Row{
		{
			Seq: 2, Agent: "claude", When: "1 min ago", Prompt: prompt, Files: 1, Status: "done",
			Preview: "  Undo #2 would:\n\n    remove   2 files   src/components/theme/Button.tsx, src/components/theme/Header.tsx\n\n  ran: " + ranText + "\n",
		},
		{Seq: 1, Agent: "codex", When: "2 min ago", Prompt: "edit a", Files: 1, Status: "done", Preview: "  Undo #1 would:\n\n    revert  1 file\n      a.txt\n"},
	}
	m := New(rows, "proj", "", false, true)
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(Model)
	before := m.View()
	listAt := lineNo(before, `"`+prompt+`"`)
	secondAt := lineNo(before, "edit a")
	if listAt != 3 || secondAt != listAt+1 {
		t.Fatalf("rows list=%d second=%d\n%s", listAt, secondAt, before)
	}
	blank := strings.Split(before, "\n")[listAt-1]
	if ui.DisplayWidth(blank) != 120 || strings.TrimSpace(blank) != "" {
		t.Fatalf("reserved row width %d %q", ui.DisplayWidth(blank), blank)
	}
	button, header := 0, 0
	var ranLines []string
	for _, line := range strings.Split(before, "\n") {
		if strings.Contains(line, "Button") && strings.Contains(line, "Header") {
			t.Fatalf("paths merged: %s", line)
		}
		if strings.Contains(line, "Button") {
			button++
		}
		if strings.Contains(line, "Header") {
			header++
		}
		trimmed := strings.TrimSpace(line)
		if strings.Contains(line, "ran:") || (len(ranLines) > 0 && (strings.HasPrefix(trimmed, "alpha") || strings.HasPrefix(trimmed, "omega"))) {
			ranLines = append(ranLines, line)
		}
	}
	if button != 1 || header != 1 {
		t.Fatalf("paths button=%d header=%d\n%s", button, header, before)
	}
	joined := strings.Join(ranLines, " ")
	if len(ranLines) < 2 || !strings.Contains(joined, "omega") || strings.Contains(joined, "…") {
		t.Fatalf("ran wrap:\n%s", strings.Join(ranLines, "\n"))
	}

	long := strings.Repeat("p", 95)
	longRows := []Row{{
		Seq: 2, Agent: "claude", When: "1 min ago", Prompt: long, Files: 1, Status: "done",
		Preview: "  Undo #2 would:\n",
	}}
	lm := New(longRows, "proj", "", false, true)
	lm.Width = 120
	lm.Height = 40
	var shown int
	var pLine string
	for _, line := range strings.Split(lm.View(), "\n") {
		if !strings.Contains(line, "#2") || !strings.Contains(line, `"`) {
			continue
		}
		shown = strings.Count(quoted(line), "p")
		pLine = line
	}
	if shown < 60 {
		t.Fatalf("95-char prompt showed %d chars: %s", shown, pLine)
	}
	if !strings.Contains(pLine, "1 file") {
		t.Fatalf("file column:\n%s", pLine)
	}

	m.Apply = func(a Action) ([]Row, string, error) {
		if a.Kind != "undo" || a.Seq != 2 {
			t.Fatalf("action %+v", a)
		}
		next := append([]Row(nil), rows...)
		next[0].Status = "undone"
		next[0].Preview = "  #2 is already undone.\n  Redo #2 would re-apply:\n\n    add  1 file\n      a.txt\n"
		return next, "Undid #2", nil
	}
	model = m
	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("first enter confirms")
	}
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected apply")
	}
	model, _ = model.Update(cmd())
	m = model.(Model)
	after := m.View()
	if got := lineNo(after, `"`+prompt+`"`); got != listAt {
		t.Fatalf("list moved from line %d to %d\nbefore:\n%s\nafter:\n%s", listAt, got, before, after)
	}
	if lineNo(after, "edit a") != secondAt {
		t.Fatalf("second row moved to %d", lineNo(after, "edit a"))
	}
	toast := strings.Split(after, "\n")[listAt-1]
	if ui.DisplayWidth(toast) != 120 || !strings.Contains(toast, "Undid #2") {
		t.Fatalf("toast %q width %d", toast, ui.DisplayWidth(toast))
	}
	if !strings.Contains(after, `"`+prompt+`"`) {
		t.Fatalf("prompt after undo:\n%s", after)
	}
}

func TestPreviewPathsTruncateSeparately(t *testing.T) {
	preview := "    remove   2 files   src/components/theme/Button.tsx, src/components/theme/Header.tsx\n"
	got := fitPreview(preview, 28)
	if strings.Contains(got, "Button") && strings.Contains(got, "Header") && strings.Count(got, "\n") < 1 {
		t.Fatalf("merged:\n%s", got)
	}
	lines := strings.Split(got, "\n")
	var paths int
	for _, line := range lines {
		if strings.Contains(line, "Button") && strings.Contains(line, "Header") {
			t.Fatalf("one line holds both paths: %s", line)
		}
		if strings.Contains(line, "Button") || strings.Contains(line, "Header") || strings.Contains(line, "…") {
			if strings.Contains(line, "2 files") && strings.Contains(line, "…") {
				t.Fatalf("count line truncated: %s", line)
			}
			if !strings.Contains(line, "2 files") {
				paths++
			}
		}
	}
	if paths < 2 {
		t.Fatalf("paths:\n%s", got)
	}
}

func TestFooterKeepsHelpAndQuit(t *testing.T) {
	m := New(sampleRows(), "proj", "", false, true)
	m.Width = 80
	m.Height = 24
	lines := strings.Split(m.View(), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "? help") || !strings.Contains(last, "q quit") {
		t.Fatalf("footer %q", last)
	}
	if ui.DisplayWidth(last) > 80 {
		t.Fatalf("footer width %d %q", ui.DisplayWidth(last), last)
	}
	if strings.Contains(last, "rewind") {
		t.Fatalf("low-priority hint kept: %q", last)
	}
}

func TestPromptUsesRemainingWidthAt175(t *testing.T) {
	prompt := strings.Repeat("p", 95)
	rows := []Row{{
		Seq: 2, Agent: "claude", When: "1 min ago", Prompt: prompt, Files: 1, Status: "done",
		Preview: "  Undo #2 would:\n",
	}}
	m := New(rows, "proj", "", false, true)
	m.Width = 175
	m.Height = 30
	view := m.View()
	if !strings.Contains(view, `"`+prompt+`"`) {
		t.Fatalf("prompt cut:\n%s", view)
	}
	if !strings.Contains(view, "1 file") {
		t.Fatalf("count:\n%s", view)
	}
}

func TestKeptFileShowsInStatus(t *testing.T) {
	rows := []Row{{
		Seq: 3, Agent: "claude", When: "1 min ago", Prompt: "edit readme", Files: 9, Status: "kept",
		Plan:    "add 8 files, keep 1 file (yours)",
		Preview: "  #3 is undone (partial).\n  Redo #3 would re-apply:\n\n    add      8 files\n      pkg/m2.go\n    keep     1 file (yours)\n      pkg/m1.go\n",
	}}
	m := New(rows, "proj", "", false, true)
	m.Width = 120
	m.Height = 24
	m.notice = "Undid #3, except pkg/m1.go"
	m.noticeOK = false
	view := m.View()
	var row string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "9 files") && strings.Contains(line, "partial") {
			row = line
			break
		}
	}
	if row == "" || strings.Contains(row, "pkg/m1.go") || strings.Contains(row, "9 filespartial") || !strings.Contains(row, " partial") {
		t.Fatalf("row %q\n%s", row, view)
	}
	if !strings.Contains(view, "pkg/m1.go") || !strings.Contains(view, "undone (partial)") || !strings.Contains(view, "Redo #3 would re-apply:") {
		t.Fatalf("preview:\n%s", view)
	}
	if strings.Contains(view, "Undo #3 would") {
		t.Fatalf("stale undo preview:\n%s", view)
	}
	if !strings.Contains(view, "! Undid #3, except pkg/m1.go") {
		t.Fatalf("toast:\n%s", view)
	}
	var model tea.Model = m
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := model.(Model)
	if got.pending != "redo" || !strings.Contains(got.View(), "Redo turn #3?") {
		t.Fatalf("pending %q\n%s", got.pending, got.View())
	}
	if !strings.Contains(got.View(), "add 8 files, keep 1 file (yours)") {
		t.Fatalf("confirm card:\n%s", got.View())
	}
}

func TestPartialRedoToastMatchesCLI(t *testing.T) {
	rows := sampleRows()
	notice := "Redid the last change, except 1 file you changed afterwards: pkg/m1.go"
	m := New(rows, "proj", "", false, false)
	m.Width = 160
	m.Height = 24
	m.Apply = func(a Action) ([]Row, string, error) {
		if a.Kind != "redo" {
			t.Fatalf("kind %s", a.Kind)
		}
		return rows, notice, nil
	}
	var model tea.Model = m
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if cmd == nil {
		t.Fatal("expected apply")
	}
	model, _ = model.Update(cmd())
	view := model.(Model).View()
	if !strings.Contains(view, "! Redid the last change, except 1 file you changed afterwards") || !strings.Contains(view, "pkg/m1.go") {
		t.Fatalf("toast:\n%s", view)
	}
	if strings.Contains(view, "✓") {
		t.Fatalf("success glyph on a partial redo:\n%s", view)
	}
}

func TestTimelineColumnsRightAlign(t *testing.T) {
	long := strings.Repeat("p", 90)
	rows := []Row{
		{Seq: 2, Agent: "claude", When: "1 min ago", Prompt: "short", Files: 1, Status: "done", Preview: "  Undo #2 would:\n"},
		{Seq: 1, Agent: "codex", When: "2 min ago", Prompt: long, Files: 12, Status: "undone", Preview: "  Undo #1 would:\n"},
	}
	m := New(rows, "proj", "", false, true)
	m.Width = 120
	m.Height = 24
	view := m.View()
	var one, many string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, `"short"`) {
			one = line
		}
		if strings.Contains(line, "undone") && strings.Contains(line, "12 files") {
			many = line
		}
	}
	if one == "" || many == "" {
		t.Fatalf("rows:\n%s", view)
	}
	if displayEnd(one, "1 file") != displayEnd(many, "12 files") {
		t.Fatalf("file columns\n%s\n%s", one, many)
	}
	if ui.DisplayWidth(one) != 120 || ui.DisplayWidth(many) != 120 {
		t.Fatalf("row widths %d %d\n%s\n%s", ui.DisplayWidth(one), ui.DisplayWidth(many), one, many)
	}
	if !strings.Contains(many, "…") {
		t.Fatalf("long prompt was not truncated:\n%s", many)
	}
	if displayEnd(many, "undone") <= displayEnd(many, "12 files") {
		t.Fatalf("undone column:\n%s", many)
	}
}

func TestTimelinePreviewFitsHeight(t *testing.T) {
	var b strings.Builder
	b.WriteString("  Undo #2 would:\n\n    remove   8 files\n")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&b, "      src/file%d.ts\n", i)
	}
	b.WriteString("\n  ran: tidy the api module\n")
	rows := make([]Row, 0, 8)
	rows = append(rows, Row{
		Seq: 9, Agent: "claude", When: "1 min ago", Prompt: "edit api", Files: 8, Status: "done", Preview: b.String(),
	})
	for i := 0; i < 7; i++ {
		rows = append(rows, Row{
			Seq: i + 1, Agent: "codex", When: "2 min ago", Prompt: "edit", Files: 1, Status: "done",
			Preview: "  Undo #1 would:\n\n    revert  1 file\n      a.txt\n",
		})
	}
	for _, sz := range []struct{ w, h int }{{120, 22}, {80, 24}} {
		m := New(rows, "proj", "", false, true)
		m.Width = sz.w
		m.Height = sz.h
		view := m.View()
		lines := strings.Split(view, "\n")
		if len(lines) > sz.h {
			t.Fatalf("%dx%d has %d lines\n%s", sz.w, sz.h, len(lines), view)
		}
		if !strings.Contains(lines[0], "oops") {
			t.Fatalf("%dx%d title %q\n%s", sz.w, sz.h, lines[0], view)
		}
		if !strings.Contains(lines[len(lines)-1], "select") {
			t.Fatalf("%dx%d footer %q", sz.w, sz.h, lines[len(lines)-1])
		}
		if strings.TrimSpace(lines[2]) != "" && !strings.Contains(lines[2], "Undid") {
			t.Fatalf("%dx%d toast row %q", sz.w, sz.h, lines[2])
		}
		shown := 0
		for i := 0; i < 8; i++ {
			if strings.Contains(view, fmt.Sprintf("src/file%d.ts", i)) {
				shown++
			}
		}
		if shown >= 8 || shown < 1 || !strings.Contains(view, "more") {
			t.Fatalf("%dx%d showed %d paths\n%s", sz.w, sz.h, shown, view)
		}
		ruleBeforePreview := false
		for i, line := range lines {
			if strings.Contains(line, "Undo #") && i > 0 && strings.Contains(lines[i-1], "─") {
				ruleBeforePreview = true
			}
		}
		if !ruleBeforePreview {
			t.Fatalf("%dx%d missing the rule between list and preview\n%s", sz.w, sz.h, view)
		}
	}
}

func TestClipPreviewTightRow(t *testing.T) {
	lines := []string{
		"  Undo #2 would:",
		"",
		"  remove   4 files",
		"    src/api.ts",
		"    src/db.ts",
		"    src/ui.ts",
		"    src/app.ts",
	}
	got := strings.Join(clipPreview(lines, 4), "\n")
	if !strings.Contains(got, "src/api.ts … +3 more") {
		t.Fatalf("tight row:\n%s", got)
	}
	if strings.Contains(got, "src/db.ts") {
		t.Fatalf("extra path:\n%s", got)
	}
}

func TestTimelineDividerFullHeight(t *testing.T) {
	var b strings.Builder
	b.WriteString("  Undo #3 would:\n\n    remove   4 files\n")
	for _, path := range []string{"src/api.ts", "src/db.ts", "src/ui.ts", "src/app.ts"} {
		b.WriteString("      " + path + "\n")
	}
	b.WriteString("\n  ran: wire the preview\n")
	rows := []Row{{
		Seq: 3, Agent: "claude", When: "1 min ago", Prompt: "short", Files: 4, Status: "done", Preview: b.String(),
	}}
	m := New(rows, "proj", "", false, true)
	m.Width = 170
	m.Height = 30
	view := m.View()
	col := -1
	bars := 0
	var indents []int
	for _, line := range strings.Split(view, "\n") {
		i := strings.Index(line, "│")
		if i < 0 {
			continue
		}
		if col < 0 {
			col = i
		}
		if i != col {
			t.Fatalf("divider column %d want %d\n%s", i, col, line)
		}
		bars++
		right := line[i+len("│"):]
		trim := strings.TrimSpace(right)
		if strings.Contains(trim, "src/") {
			indents = append(indents, len(right)-len(strings.TrimLeft(right, " ")))
		}
	}
	if bars < 4 {
		t.Fatalf("divider rows %d\n%s", bars, view)
	}
	if len(indents) < 2 {
		t.Fatalf("paths:\n%s", view)
	}
	for _, n := range indents[1:] {
		if n != indents[0] {
			t.Fatalf("path indents %v\n%s", indents, view)
		}
	}
}

func displayEnd(line, needle string) int {
	i := strings.Index(line, needle)
	if i < 0 {
		return -1
	}
	return ui.DisplayWidth(line[:i]) + ui.DisplayWidth(needle)
}

func quoted(line string) string {
	a := strings.Index(line, `"`)
	b := strings.LastIndex(line, `"`)
	if a < 0 || b <= a {
		return ""
	}
	return line[a+1 : b]
}

func lineNo(s, needle string) int {
	for i, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
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
