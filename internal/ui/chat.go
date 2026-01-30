package ui

import (
	"math/rand"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zhubert/milo/internal/agent"
)

const maxInputHeight = 10

// placeholderPhrases contains example prompts to inspire users.
var placeholderPhrases = []string{
	"Create a function that...",
	"Fix this bug in my code",
	"Write a unit test for...",
	"Refactor this function",
	"Add error handling to...",
	"Generate a REST API for...",
	"Optimize this algorithm",
	"Debug why this fails",
	"Add logging to...",
	"Create a struct for...",
	"Write documentation for...",
	"Implement a cache for...",
	"Add validation to...",
	"Create a CLI tool that...",
	"Parse this JSON into...",
	"Add middleware for...",
	"Create a database model for...",
	"Write a goroutine that...",
	"Add tests for edge cases",
	"Review this code for issues",
}

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

	modelSelectMode   bool                 // true when in model selection mode
	modelSelectModels []agent.ModelOption // available models
	modelSelectIndex  int                  // selected model index

	pendingToolName  string // tool currently being executed
	pendingToolInput string // input for pending tool (used for spinner display)

	// Parallel execution progress state.
	parallelProgress *ParallelProgressState
}

// ParallelProgressState tracks progress of parallel tool execution.
type ParallelProgressState struct {
	Total      int
	Completed  int
	InProgress []string
}

// randomPlaceholder returns a random placeholder phrase from the available options.
func randomPlaceholder() string {
	return placeholderPhrases[rand.Intn(len(placeholderPhrases))]
}

// NewChat creates a new chat component.
func NewChat() *Chat {
	ti := textarea.New()
	ti.Prompt = "> "
	ti.Placeholder = randomPlaceholder()
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

// AppendToolUse sets the pending tool (shown as spinner while executing).
func (c *Chat) AppendToolUse(name, input string) {
	c.pendingToolName = name
	c.pendingToolInput = input
	c.updateContent()
}

// AppendToolResult clears pending tool and adds the result to streaming content.
func (c *Chat) AppendToolResult(name, output string, isError bool) {
	// Clear pending tool state.
	input := c.pendingToolInput
	c.pendingToolName = ""
	c.pendingToolInput = ""

	// Clear parallel progress when a tool completes.
	c.parallelProgress = nil

	// Add result to streaming content with the original input for context.
	c.streaming += RenderToolResult(name, input, output, isError)
	c.updateContent()
}

// SetParallelProgress updates the parallel execution progress display.
func (c *Chat) SetParallelProgress(total, completed int, inProgress []string) {
	c.parallelProgress = &ParallelProgressState{
		Total:      total,
		Completed:  completed,
		InProgress: inProgress,
	}
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
		if c.spinner != nil && (c.waiting || c.streaming != "" || c.pendingToolName != "") {
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

	// Only forward key messages to viewport when input is not focused,
	// otherwise viewport keybindings (like 'u' for scroll up) interfere with typing.
	if _, isKey := msg.(tea.KeyMsg); !isKey || !c.focused {
		var cmd tea.Cmd
		c.viewport, cmd = c.viewport.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return c, tea.Batch(cmds...)
}

// SetPermissionMode enables/disables the permission prompt display.
func (c *Chat) SetPermissionMode(on bool, toolName string) {
	c.permissionMode = on
	c.permToolName = toolName
}

// SetModelSelectMode enables/disables the model selection display.
func (c *Chat) SetModelSelectMode(on bool, models []agent.ModelOption, index int) {
	c.modelSelectMode = on
	c.modelSelectModels = models
	c.modelSelectIndex = index
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

	// Build model selection interface if active.
	var modelPrompt string
	if c.modelSelectMode {
		modelPrompt = c.renderModelSelection()
	}

	if permPrompt != "" {
		return lipgloss.JoinVertical(lipgloss.Left, vpView, permPrompt, line, inputView, line)
	}
	if modelPrompt != "" {
		return lipgloss.JoinVertical(lipgloss.Left, vpView, modelPrompt, line, inputView, line)
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

func (c *Chat) renderModelSelection() string {
	if len(c.modelSelectModels) == 0 {
		return ""
	}

	var lines []string
	title := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("Select Model:")
	lines = append(lines, title)
	lines = append(lines, "")

	for i, model := range c.modelSelectModels {
		prefix := "  "
		style := lipgloss.NewStyle()
		
		if i == c.modelSelectIndex {
			prefix = "▶ "
			style = style.Foreground(ColorPrimary).Bold(true)
		}

		line := prefix + style.Render(model.DisplayName+" ") + DimStyle.Render("("+model.ID+")")
		lines = append(lines, line)
	}

	lines = append(lines, "")
	keys := FooterStyle.Render("↑↓") + DimStyle.Render(" navigate  ") +
		FooterStyle.Render("enter") + DimStyle.Render(" select  ") +
		FooterStyle.Render("esc") + DimStyle.Render(" cancel")
	lines = append(lines, keys)

	return strings.Join(lines, "\n")
}

// SetWelcomeContent sets the welcome content (logo/info) to show initially.
func (c *Chat) SetWelcomeContent(content string) {
	c.welcomeContent = content
	c.updateContent()
}

// formatToolVerb converts a tool name to a present participle for spinner display.
func formatToolVerb(name string) string {
	switch name {
	case "read":
		return "Reading"
	case "write":
		return "Writing"
	case "edit":
		return "Editing"
	case "bash":
		return "Running"
	case "glob":
		return "Searching"
	case "grep":
		return "Searching"
	default:
		return "Running " + name
	}
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

	// Show streaming content.
	if c.streaming != "" {
		rendered := RenderAssistantLabel() + RenderMarkdown(c.streaming, c.width-4)
		parts = append(parts, rendered)
	}

	// Show pending tool spinner if a tool is executing.
	if c.parallelProgress != nil && c.spinner != nil {
		// Show parallel execution progress.
		parts = append(parts, c.spinner.RenderParallelSpinner(
			c.parallelProgress.Total,
			c.parallelProgress.Completed,
			c.parallelProgress.InProgress,
		))
	} else if c.pendingToolName != "" && c.spinner != nil {
		verb := formatToolVerb(c.pendingToolName)
		parts = append(parts, c.spinner.RenderSpinner(verb))
	} else if c.waiting && c.spinner != nil {
		// Show thinking spinner if waiting for first response.
		parts = append(parts, c.spinner.RenderSpinner("Thinking"))
	}

	content := strings.Join(parts, "\n\n")

	// Size viewport based on content, up to max height.
	// Calculate available height dynamically based on actual input height.
	actualInputHeight := c.input.Height()
	if actualInputHeight < 1 {
		actualInputHeight = 1
	}
	availableHeight := c.height - actualInputHeight - 2 // 2 for border lines
	if availableHeight < 1 {
		availableHeight = 1
	}

	contentLines := strings.Count(content, "\n") + 1
	vpHeight := contentLines
	if vpHeight > availableHeight {
		vpHeight = availableHeight
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	c.viewport.Height = vpHeight

	c.viewport.SetContent(content)
	c.viewport.GotoBottom()
}
