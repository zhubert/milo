package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/zhubert/looper/internal/ui"
)

// View implements tea.Model.
func (m *Model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	if m.quitting {
		v.SetContent("Goodbye.\n")
		return v
	}

	header := m.header.View()
	footer := m.footer.View()

	ctx := ui.GetViewContext()
	chatHeight := ctx.ContentHeight
	if chatHeight < 1 {
		chatHeight = 1
	}

	// Placeholder chat area until Phase 7.
	chat := lipgloss.NewStyle().
		Width(m.width).
		Height(chatHeight).
		Render("Type a message to get started...")

	view := lipgloss.JoinVertical(lipgloss.Left, header, chat, footer)
	v.SetContent(view)
	return v
}
