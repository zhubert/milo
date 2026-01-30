package ui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderMarkdown does a basic markdown-to-styled-text conversion.
// Handles headers, code blocks, bold, inline code, and bullet lists.
func RenderMarkdown(content string, width int) string {
	if width < 10 {
		width = 10
	}

	var result strings.Builder
	lines := strings.Split(content, "\n")
	inCodeBlock := false
	var codeLines []string
	var codeLang string

	for _, line := range lines {
		// Code block fences.
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				inCodeBlock = true
				codeLang = strings.TrimPrefix(line, "```")
				codeLines = nil
			} else {
				result.WriteString(renderCodeBlock(codeLines, codeLang, width))
				inCodeBlock = false
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Headers.
		if strings.HasPrefix(line, "### ") {
			text := strings.TrimPrefix(line, "### ")
			result.WriteString(lipgloss.NewStyle().Bold(true).Render(text))
			result.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			text := strings.TrimPrefix(line, "## ")
			result.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(text))
			result.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "# ") {
			text := strings.TrimPrefix(line, "# ")
			result.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(text))
			result.WriteString("\n")
			continue
		}

		// Bullet lists.
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result.WriteString("  • " + renderInline(line[2:]))
			result.WriteString("\n")
			continue
		}

		// Regular line with inline formatting.
		result.WriteString(renderInline(line))
		result.WriteString("\n")
	}

	// Close unclosed code block.
	if inCodeBlock && len(codeLines) > 0 {
		result.WriteString(renderCodeBlock(codeLines, codeLang, width))
	}

	return result.String()
}

func renderCodeBlock(lines []string, lang string, width int) string {
	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A5F3FC")).
		Background(lipgloss.Color("#1E293B")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorDim).
		Background(lipgloss.Color("#1E293B"))

	content := strings.Join(lines, "\n")

	var b strings.Builder
	if lang != "" {
		b.WriteString(labelStyle.Render(fmt.Sprintf(" %s ", lang)))
		b.WriteString("\n")
	}
	b.WriteString(codeStyle.Render(content))
	b.WriteString("\n")
	return b.String()
}

func renderInline(text string) string {
	// Bold: **text**
	text = renderPattern(text, "**", "**", func(s string) string {
		return lipgloss.NewStyle().Bold(true).Render(s)
	})

	// Inline code: `text`
	text = renderPattern(text, "`", "`", func(s string) string {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A5F3FC")).
			Background(lipgloss.Color("#1E293B")).
			Render(s)
	})

	return text
}

func renderPattern(text, open, close string, style func(string) string) string {
	var result strings.Builder
	for {
		start := strings.Index(text, open)
		if start == -1 {
			result.WriteString(text)
			break
		}
		end := strings.Index(text[start+len(open):], close)
		if end == -1 {
			result.WriteString(text)
			break
		}
		end += start + len(open)

		result.WriteString(text[:start])
		inner := text[start+len(open) : end]
		result.WriteString(style(inner))
		text = text[end+len(close):]
	}
	return result.String()
}

// extractFilePath attempts to extract a file_path from JSON input.
func extractFilePath(input string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return ""
	}
	if fp, ok := data["file_path"].(string); ok {
		return fp
	}
	return ""
}

// shortenPath returns the last n components of a path.
func shortenPath(path string, components int) string {
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) <= components {
		return path
	}
	return "…/" + strings.Join(parts[len(parts)-components:], string(filepath.Separator))
}

// formatToolSummary creates a brief summary of what the tool did.
func formatToolSummary(name, input string) string {
	filePath := extractFilePath(input)
	if filePath != "" {
		short := shortenPath(filePath, 3)
		return DimStyle.Render(short)
	}

	// For other tools, show truncated input.
	if len(input) > 60 {
		input = input[:60] + "…"
	}
	return DimStyle.Render(input)
}

// RenderToolResult formats a tool execution result with clean visual display.
func RenderToolResult(name, input, output string, isError bool) string {
	var b strings.Builder

	// Add spacing before tool block.
	b.WriteString("\n")

	// Status icon and tool name with file context.
	var statusIcon string
	if isError {
		statusIcon = ErrorStyle.Render("✗")
	} else {
		statusIcon = SuccessStyle.Render("✓")
	}

	toolName := ToolNameStyle.Render(name)
	summary := formatToolSummary(name, input)

	// Show line count if output is present (collapsed by default).
	lineCount := ""
	if output != "" {
		lines := strings.Split(output, "\n")
		if len(lines) == 1 {
			lineCount = DimStyle.Render(" (1 line)")
		} else {
			lineCount = DimStyle.Render(fmt.Sprintf(" (%d lines)", len(lines)))
		}
	}

	b.WriteString(statusIcon + " " + toolName + "  " + summary + lineCount + "\n")

	return b.String()
}

// RenderErrorMessage formats an error message for the chat area.
func RenderErrorMessage(text string) string {
	label := ErrorStyle.Bold(true).Render("Error")
	return label + "\n" + ErrorStyle.Render(text) + "\n"
}

// RenderUserMessage formats a user message.
func RenderUserMessage(text string) string {
	label := lipgloss.NewStyle().Bold(true).Foreground(ColorText).Render("You")
	return label + "\n" + text + "\n"
}

// RenderAssistantLabel renders the assistant message label.
func RenderAssistantLabel() string {
	return lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("Claude") + "\n"
}

// RenderSystemMessage formats a system message (for command output).
func RenderSystemMessage(text string) string {
	label := lipgloss.NewStyle().Bold(true).Foreground(ColorDim).Render("System")
	return label + "\n" + text + "\n"
}
