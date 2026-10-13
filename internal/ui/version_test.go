package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lintsinghua/oops/internal/version"
)

func TestVersionModelView(t *testing.T) {
	info := version.Info{Version: "v1.2.3", Commit: "abc123", Date: "2026-10-13T00:00:00Z"}
	got := stripANSI(NewVersionModel(info).View())

	for _, want := range []string{"oops v1.2.3", "commit: abc123", "built: 2026-10-13T00:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("View() = %q, missing %q", got, want)
		}
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("View() = %q, want a trailing newline", got)
	}
}

func TestVersionModelQuits(t *testing.T) {
	m := NewVersionModel(version.Current())
	if cmd := m.Init(); cmd != nil {
		t.Fatal("Init() = non-nil cmd, want nil")
	}

	_, cmd := m.Update(nil)
	if cmd == nil {
		t.Fatal("Update() cmd = nil, want quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("Update() cmd produced %T, want tea.QuitMsg", cmd())
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}
