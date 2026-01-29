package app

import "github.com/charmbracelet/lipgloss"

// View implements tea.Model.
func (m *Model) View() string {
	if m.quitting {
		return "Goodbye.\n"
	}

	header := m.header.View()
	footer := m.footer.View()
	chatView := m.chat.View()

	return lipgloss.JoinVertical(lipgloss.Left, header, chatView, footer)
}
