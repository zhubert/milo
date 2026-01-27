package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	chatView := m.chat.View()

	view := lipgloss.JoinVertical(lipgloss.Left, header, chatView, footer)
	v.SetContent(view)
	return v
}
