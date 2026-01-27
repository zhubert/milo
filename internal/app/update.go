package app

import (
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

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case ui.FlashTickMsg:
		m.footer.ClearFlash()
	}

	return m, tea.Batch(cmds...)
}
