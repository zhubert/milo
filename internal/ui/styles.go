package ui

import "github.com/charmbracelet/lipgloss"

// Color palette.
var (
	ColorPrimary  = lipgloss.Color("#7C3AED")
	ColorDim      = lipgloss.Color("#6B7280")
	ColorText     = lipgloss.Color("#E5E7EB")
	ColorBorder   = lipgloss.Color("#374151")
	ColorLineDark = lipgloss.Color("#374151")
	ColorError    = lipgloss.Color("#EF4444")
	ColorSuccess  = lipgloss.Color("#10B981")
	ColorToolName = lipgloss.Color("#06B6D4")
)

// Styles.
var (
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorDim)

	DimStyle = lipgloss.NewStyle().
			Foreground(ColorDim)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	ToolNameStyle = lipgloss.NewStyle().
			Foreground(ColorToolName).
			Bold(true)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	// Textarea styles
	TextareaStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	TextareaFocusedStyle = lipgloss.NewStyle().
				Foreground(ColorText)
)
