package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxInputHeight = 2

// chatMessage stores a rendered message in the history.
type chatMessage struct {
	role    string // "user" or "assistant"
	content string // rendered content
}

// Chat is the main chat component with message viewport and input area.
type Chat struct {
	viewport viewport.Model
	input    textarea.Model
	width    int
	height   int
	focused  bool

	maxVPHeight int // maximum viewport height based on terminal size

	welcomeContent string // logo and info shown initially
	messages       []chatMessage
	streaming      string // accumulates current assistant response
	spinner        *SpinnerState
	waiting        bool // true after sending, before first token

	permissionMode bool   // true when waiting for permission response
	permToolName   string // tool name for permission prompt
}

// NewChat creates a new chat component.
func NewChat() *Chat {
	ti := textarea.New()
	ti.Prompt = "> "
	ti.Placeholder = "Try \"create a function that...\""
	ti.CharLimit = 0
	ti.ShowLineNumbers = false
	ti.SetHeight(1)
	ti.MaxHeight = maxInputHeight

	// Remove background styling to use terminal's default
	ti.FocusedStyle.Base = lipgloss.NewStyle()
	ti.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ti.FocusedStyle.Text = lipgloss.NewStyle()
	ti.BlurredStyle.Base = lipgloss.NewStyle()
	ti.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ti.BlurredStyle.Text = lipgloss.NewStyle()

	ti.Focus()

	vp := viewport.New(80, 20)

	return &Chat{
		viewport: vp,
		input:    ti,
		focused:  true,
	}
}

// SetSize updates the chat component dimensions.
func (c *Chat) SetSize(width, height int) {
	c.width = width
	c.height = height

	// Max viewport height (leaving room for input area).
	c.maxVPHeight = height - maxInputHeight - 2
	if c.maxVPHeight < 1 {
		c.maxVPHeight = 1
	}

	c.viewport.Width = width
	c.input.SetWidth(width - 2)
	c.input.MaxHeight = maxInputHeight

	c.updateContent()
}

// Focus gives focus to the text input.
func (c *Chat) Focus() tea.Cmd {
	c.focused = true
	return c.input.Focus()
}

// Blur removes focus from the text input.
func (c *Chat) Blur() {
	c.focused = false
	c.input.Blur()
}

// InputValue returns the current text input value.
func (c *Chat) InputValue() string {
	return c.input.Value()
}

// ResetInput clears the text input.
func (c *Chat) ResetInput() {
	c.input.Reset()
}

// SetWaiting marks the chat as waiting for a response (shows spinner).
func (c *Chat) SetWaiting(on bool) {
	c.waiting = on
	if on {
		c.spinner = NewSpinnerState()
	}
}

// IsWaiting returns whether the chat is waiting for a response.
func (c *Chat) IsWaiting() bool {
	return c.waiting
}

// IsStreaming returns whether the chat is actively receiving tokens.
func (c *Chat) IsStreaming() bool {
	return c.streaming != ""
}

// AddUserMessage appends a user message to the history.
func (c *Chat) AddUserMessage(text string) {
	rendered := RenderUserMessage(text)
	c.messages = append(c.messages, chatMessage{role: "user", content: rendered})
	c.updateContent()
}

// AddErrorMessage appends an error message to the history.
func (c *Chat) AddErrorMessage(text string) {
	rendered := RenderErrorMessage(text)
	c.messages = append(c.messages, chatMessage{role: "assistant", content: rendered})
	c.updateContent()
}

// AddSystemMessage appends a system message to the history.
func (c *Chat) AddSystemMessage(text string) {
	rendered := RenderSystemMessage(text)
	c.messages = append(c.messages, chatMessage{role: "system", content: rendered})
	c.updateContent()
}

// AppendStreaming appends text to the current streaming response.
func (c *Chat) AppendStreaming(text string) {
	if c.waiting {
		c.waiting = false
	}
	c.streaming += text
	c.updateContent()
}

// AppendToolUse adds a tool use notification to the streaming content.
func (c *Chat) AppendToolUse(name, input string) {
	c.streaming += RenderToolUse(name, input)
	c.updateContent()
}

// AppendToolResult adds a tool result to the streaming content.
func (c *Chat) AppendToolResult(name, output string, isError bool) {
	c.streaming += RenderToolResult(name, output, isError)
	c.updateContent()
}

// FinishStreaming moves the accumulated streaming content into the message history.
func (c *Chat) FinishStreaming() {
	c.waiting = false
	if c.streaming != "" {
		content := RenderAssistantLabel() + RenderMarkdown(c.streaming, c.width-4)
		c.messages = append(c.messages, chatMessage{role: "assistant", content: content})
		c.streaming = ""
	}
	c.spinner = nil
	c.updateContent()
}

// Update handles messages for the chat component.
func (c *Chat) Update(msg tea.Msg) (*Chat, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg.(type) {
	case StopwatchTickMsg:
		if c.spinner != nil && (c.waiting || c.streaming != "") {
			c.spinner.Advance()
			c.updateContent()
			cmds = append(cmds, StopwatchTick())
		}
	}

	if c.focused {
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return c, tea.Batch(cmds...)
}

// SetPermissionMode enables/disables the permission prompt display.
func (c *Chat) SetPermissionMode(on bool, toolName string) {
	c.permissionMode = on
	c.permToolName = toolName
}

// View renders the chat component.
func (c *Chat) View() string {
	vpView := c.viewport.View()
	inputView := c.input.View()

	// Create horizontal line for input borders with dark color.
	lineStyle := lipgloss.NewStyle().Foreground(ColorLineDark)
	line := lineStyle.Render(strings.Repeat("─", c.width))

	// Build permission prompt if active.
	var permPrompt string
	if c.permissionMode {
		permPrompt = c.renderPermissionPrompt()
	}

	if permPrompt != "" {
		return lipgloss.JoinVertical(lipgloss.Left, vpView, permPrompt, line, inputView, line)
	}
	return lipgloss.JoinVertical(lipgloss.Left, vpView, line, inputView, line)
}

func (c *Chat) renderPermissionPrompt() string {
	label := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("Allow " + c.permToolName + "?")
	keys := FooterStyle.Render("y") + DimStyle.Render("es  ") +
		FooterStyle.Render("n") + DimStyle.Render("o  ") +
		FooterStyle.Render("a") + DimStyle.Render("lways")
	return label + "  " + keys
}

// SetWelcomeContent sets the welcome content (logo/info) to show initially.
func (c *Chat) SetWelcomeContent(content string) {
	c.welcomeContent = content
	c.updateContent()
}

func (c *Chat) updateContent() {
	var parts []string

	// Always show welcome content at the top (scrolls up as messages arrive).
	if c.welcomeContent != "" {
		parts = append(parts, c.welcomeContent)
	}

	for _, msg := range c.messages {
		parts = append(parts, msg.content)
	}

	// Show streaming content, rendering only complete sections.
	// Incomplete parts are buffered until a newline arrives.
	if c.streaming != "" {
		complete, _ := splitCompleteMarkdown(c.streaming)
		if complete != "" {
			rendered := RenderAssistantLabel() + RenderMarkdown(complete, c.width-4)
			parts = append(parts, rendered)
		} else {
			// Show label with spinner-like indicator while buffering
			parts = append(parts, RenderAssistantLabel()+DimStyle.Render("..."))
		}
	}

	// Show spinner if waiting.
	if c.waiting && c.spinner != nil {
		verb := "Thinking"
		parts = append(parts, c.spinner.RenderSpinner(verb))
	}

	content := strings.Join(parts, "\n\n")

	// Size viewport based on content, up to max height.
	contentLines := strings.Count(content, "\n") + 1
	vpHeight := contentLines
	if vpHeight > c.maxVPHeight {
		vpHeight = c.maxVPHeight
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	c.viewport.Height = vpHeight

	c.viewport.SetContent(content)
	c.viewport.GotoBottom()
}

// splitCompleteMarkdown splits streaming content into complete (renderable)
// and incomplete (raw) parts. This prevents markdown flashing during streaming.
func splitCompleteMarkdown(content string) (complete, incomplete string) {
	// Count code block fences to see if we're inside an unclosed block
	fenceCount := strings.Count(content, "```")
	inCodeBlock := fenceCount%2 == 1

	if inCodeBlock {
		// Find the last opening fence and don't render anything after it
		lastFence := strings.LastIndex(content, "```")
		if lastFence > 0 {
			return content[:lastFence], content[lastFence:]
		}
		return "", content
	}

	// Not in a code block - split at the last newline
	lastNewline := strings.LastIndex(content, "\n")
	if lastNewline == -1 {
		// No newlines yet, everything is incomplete
		return "", content
	}

	return content[:lastNewline+1], content[lastNewline+1:]
}
