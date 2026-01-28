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
	flash string
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

// View renders the footer as a string.
func (f *Footer) View() string {
	ctx := GetViewContext()

	var content string
	if f.flash != "" {
		content = f.flash
	} else {
		content = FooterStyle.Render("esc") + DimStyle.Render(" cancel  ") +
			FooterStyle.Render("ctrl+c") + DimStyle.Render(" quit")
	}

	padding := ctx.TerminalWidth - lipgloss.Width(content)
	if padding > 0 {
		content += lipgloss.NewStyle().Width(padding).Render("")
	}

	return content
}
