package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/zhubert/looper/internal/ui"
)

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		ctx := ui.GetViewContext()
		ctx.UpdateTerminalSize(msg.Width, msg.Height)
		m.chat.SetSize(msg.Width, ctx.ContentHeight)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if !m.streaming && !m.permPending {
				text := strings.TrimSpace(m.chat.InputValue())
				if text != "" {
					m.chat.ResetInput()
					cmds = append(cmds, func() tea.Msg {
						return SendMsg{Text: text}
					})
				}
			}
		}

	case ui.FlashTickMsg:
		m.footer.ClearFlash()
	}

	// Delegate to chat component.
	chat, cmd := m.chat.Update(msg)
	m.chat = chat
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
