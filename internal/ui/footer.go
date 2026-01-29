package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// ModelUsage tracks token usage for a specific model.
type ModelUsage struct {
	InputTokens  int64
	OutputTokens int64
}

// Footer renders the bottom bar with keybindings and optional flash messages.
type Footer struct {
	flash string

	// Token usage tracking for the session, keyed by model ID.
	usageByModel map[string]*ModelUsage
}

// NewFooter creates a new footer.
func NewFooter() *Footer {
	return &Footer{
		usageByModel: make(map[string]*ModelUsage),
	}
}

// SetFlash sets a temporary flash message.
func (f *Footer) SetFlash(msg string) {
	f.flash = msg
}

// ClearFlash removes the flash message.
func (f *Footer) ClearFlash() {
	f.flash = ""
}

// AddUsage accumulates token usage from a turn for a specific model.
func (f *Footer) AddUsage(model string, inputTokens, outputTokens int64) {
	if f.usageByModel == nil {
		f.usageByModel = make(map[string]*ModelUsage)
	}
	usage, ok := f.usageByModel[model]
	if !ok {
		usage = &ModelUsage{}
		f.usageByModel[model] = usage
	}
	usage.InputTokens += inputTokens
	usage.OutputTokens += outputTokens
}

// UsageByModel returns the usage map for all models.
func (f *Footer) UsageByModel() map[string]*ModelUsage {
	return f.usageByModel
}

// TotalTokens returns the total tokens across all models.
func (f *Footer) TotalTokens() int64 {
	var total int64
	for _, usage := range f.usageByModel {
		total += usage.InputTokens + usage.OutputTokens
	}
	return total
}

// View renders the footer as a string.
func (f *Footer) View() string {
	ctx := GetViewContext()

	var leftContent string
	if f.flash != "" {
		leftContent = f.flash
	} else {
		leftContent = FooterStyle.Render("esc") + DimStyle.Render(" cancel  ") +
			FooterStyle.Render("ctrl+c") + DimStyle.Render(" quit")
	}

	// Show token usage on the right side if we have any.
	var rightContent string
	if total := f.TotalTokens(); total > 0 {
		rightContent = DimStyle.Render(formatTokenCount(total) + " tokens")
	}

	// Calculate padding to right-align the token count.
	leftWidth := lipgloss.Width(leftContent)
	rightWidth := lipgloss.Width(rightContent)
	padding := ctx.TerminalWidth - leftWidth - rightWidth
	if padding < 1 {
		padding = 1
	}

	if rightContent != "" {
		return leftContent + lipgloss.NewStyle().Width(padding).Render("") + rightContent
	}

	paddingTotal := ctx.TerminalWidth - leftWidth
	if paddingTotal > 0 {
		leftContent += lipgloss.NewStyle().Width(paddingTotal).Render("")
	}

	return leftContent
}

// formatTokenCount formats a token count with K suffix for thousands.
func formatTokenCount(count int64) string {
	if count >= 1000 {
		return fmt.Sprintf("%.1fK", float64(count)/1000)
	}
	return fmt.Sprintf("%d", count)
}
