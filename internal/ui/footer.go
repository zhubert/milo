package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const flashDuration = 3 * time.Second

// FlashTickMsg signals the flash message should be cleared.
type FlashTickMsg struct{}

// FlashTick returns a command that clears the flash after a delay.
func FlashTick() tea.Cmd {
	return tea.Tick(flashDuration, func(time.Time) tea.Msg {
		return FlashTickMsg{}
	})
}

// Footer renders the bottom bar with keybindings and optional flash messages.
type Footer struct {
	flash    string
	showPerm bool
}

// NewFooter creates a new footer.
func NewFooter() *Footer {
	return &Footer{}
}

// SetFlash sets a temporary flash message.
func (f *Footer) SetFlash(msg string) {
	f.flash = msg
}

// ClearFlash removes the flash message.
func (f *Footer) ClearFlash() {
	f.flash = ""
}

// SetPermissionMode enables the permission prompt display.
func (f *Footer) SetPermissionMode(on bool) {
	f.showPerm = on
}

// View renders the footer as a string.
func (f *Footer) View() string {
	ctx := GetViewContext()

	var content string
	if f.flash != "" {
		content = f.flash
	} else if f.showPerm {
		content = FooterStyle.Render("y") + DimStyle.Render("es  ") +
			FooterStyle.Render("n") + DimStyle.Render("o  ") +
			FooterStyle.Render("a") + DimStyle.Render("lways")
	} else {
		content = DimStyle.Render("enter") + FooterStyle.Render(" send  ") +
			DimStyle.Render("esc") + FooterStyle.Render(" cancel  ") +
			DimStyle.Render("ctrl+c") + FooterStyle.Render(" quit")
	}

	padding := ctx.TerminalWidth - lipgloss.Width(content)
	if padding > 0 {
		content += lipgloss.NewStyle().Width(padding).Render("")
	}

	return content
}
