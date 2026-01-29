package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/zhubert/milo/internal/agent"
	"github.com/zhubert/milo/internal/version"
)

// ASCII art logo for milo (lowercase).
var logo = []string{
	"┏┳┓┳┓  ┏━┓",
	"┃┃┃┃┃  ┃ ┃",
	"┛ ┗┻┗━╸┗━┛",
}

// Synthwave gradient colors (top to bottom).
var (
	logoColor1 = lipgloss.Color("#FF6AD5") // Hot pink
	logoColor2 = lipgloss.Color("#C774E8") // Purple
	logoColor3 = lipgloss.Color("#94D0FF") // Cyan
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	versionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF"))
)

// Header renders the top bar of the TUI.
type Header struct {
	workDir string
}

// NewHeader creates a header showing the working directory.
func NewHeader(workDir string) *Header {
	return &Header{workDir: workDir}
}

// View renders the header as a string (now empty since content moved to chat).
func (h *Header) View() string {
	return ""
}

// WelcomeContent renders the logo and welcome message for the chat viewport.
func (h *Header) WelcomeContent() string {
	// Shorten workDir to use ~ for home directory.
	displayDir := h.workDir
	if strings.HasPrefix(h.workDir, "/Users/") {
		parts := strings.SplitN(h.workDir, "/", 4)
		if len(parts) >= 3 {
			displayDir = "~"
			if len(parts) == 4 {
				displayDir = "~/" + parts[3]
			}
		}
	}

	// Build info lines.
	line1 := versionStyle.Render("v" + version.Version)
	line2 := infoStyle.Render(agent.ModelDisplayName())
	line3 := DimStyle.Render(displayDir)

	// Build logo column with synthwave gradient.
	colors := []lipgloss.TerminalColor{logoColor1, logoColor2, logoColor3}
	var logoLines []string
	for i, l := range logo {
		style := lipgloss.NewStyle().Foreground(colors[i%len(colors)])
		logoLines = append(logoLines, style.Render(l))
	}
	logoCol := strings.Join(logoLines, "\n")

	// Build info column.
	infoCol := strings.Join([]string{line1, line2, line3}, "\n")

	// Join logo and info side by side with some spacing.
	spacer := "  " // Add two spaces between logo and text
	return lipgloss.JoinHorizontal(lipgloss.Top, logoCol, spacer, infoCol)
}
