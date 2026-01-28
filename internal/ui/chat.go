package ui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const inputHeight = 3

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

	messages  []chatMessage
	streaming string // accumulates current assistant response
	spinner   *SpinnerState
	waiting   bool // true after sending, before first token
}

// NewChat creates a new chat component.
func NewChat() *Chat {
	ti := textarea.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 0
	ti.ShowLineNumbers = false
	ti.Focus()

	vp := viewport.New()

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

	vpHeight := height - inputHeight
	if vpHeight < 1 {
		vpHeight = 1
	}

	c.viewport.SetWidth(width)
	c.viewport.SetHeight(vpHeight)
	c.input.SetWidth(width - 2)

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

// View renders the chat component.
func (c *Chat) View() string {
	vpView := c.viewport.View()
	inputView := c.input.View()

	return lipgloss.JoinVertical(lipgloss.Left, vpView, inputView)
}

func (c *Chat) updateContent() {
	var parts []string

	for _, msg := range c.messages {
		parts = append(parts, msg.content)
	}

	// Show streaming content with live markdown render.
	if c.streaming != "" {
		rendered := RenderAssistantLabel() + RenderMarkdown(c.streaming, c.width-4)
		parts = append(parts, rendered)
	}

	// Show spinner if waiting.
	if c.waiting && c.spinner != nil {
		verb := "Thinking"
		parts = append(parts, c.spinner.RenderSpinner(verb))
	}

	content := strings.Join(parts, "\n")
	c.viewport.SetContent(content)
	c.viewport.GotoBottom()
}
