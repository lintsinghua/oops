// Package ui is the terminal UI stack (Bubble Tea and Lip Gloss).
// VersionModel is a styled placeholder for future interactive commands.
package ui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lintsinghua/oops/internal/version"
)

// VersionModel renders build metadata. Update quits immediately because the
// placeholder has no interaction yet.
type VersionModel struct {
	info version.Info
}

// NewVersionModel returns a model for info.
func NewVersionModel(info version.Info) VersionModel {
	return VersionModel{info: info}
}

var _ tea.Model = VersionModel{}

// Init implements tea.Model.
func (m VersionModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. The placeholder does not handle input.
func (m VersionModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, tea.Quit
}

// View implements tea.Model.
func (m VersionModel) View() string {
	title := lipgloss.NewStyle().Bold(true)
	label := lipgloss.NewStyle().Faint(true)

	return lipgloss.JoinVertical(lipgloss.Left,
		title.Render("oops "+m.info.Version),
		label.Render("commit: "+m.info.Commit),
		label.Render("built: "+m.info.Date),
	) + "\n"
}
