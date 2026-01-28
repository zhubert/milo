package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// Header renders the top bar of the TUI.
type Header struct {
	workDir string
}

// NewHeader creates a header showing the working directory.
func NewHeader(workDir string) *Header {
	return &Header{workDir: workDir}
}

// View renders the header as a string.
func (h *Header) View() string {
	ctx := GetViewContext()
	title := HeaderStyle.Render("milo")
	dir := DimStyle.Render(fmt.Sprintf(" %s", h.workDir))

	line := title + dir
	padding := ctx.TerminalWidth - lipgloss.Width(line)
	if padding > 0 {
		line += lipgloss.NewStyle().Width(padding).Render("")
	}

	return line
}
