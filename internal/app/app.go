package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/anthropics/anthropic-sdk-go"

	"github.com/zhubert/milo/internal/agent"
	"github.com/zhubert/milo/internal/permission"
	"github.com/zhubert/milo/internal/session"
	"github.com/zhubert/milo/internal/ui"
)

// AgentInterface defines the interface that the app needs from an agent.
type AgentInterface interface {
	ModelDisplayName() string
	Permissions() *permission.Checker
	SendMessage(ctx context.Context, msg string) <-chan agent.StreamChunk
	Messages() []anthropic.MessageParam
	SetMessages(messages []anthropic.MessageParam)
}

// Model is the top-level bubbletea model for the milo TUI.
type Model struct {
	agent  AgentInterface
	header *ui.Header
	footer *ui.Footer
	chat   *ui.Chat

	width  int
	height int

	// streaming is true when the agent is actively processing.
	streaming bool
	// permPending is true when we're waiting for a permission response.
	permPending  bool
	permToolName string

	// streamCh is the current agent stream channel.
	streamCh <-chan agent.StreamChunk
	// streamCancel cancels the current agent context.
	streamCancel context.CancelFunc

	// quitting signals the app should exit.
	quitting bool

	// Session persistence.
	session      *session.Session
	sessionStore *session.Store
}

// New creates the root app model.
func New(ag AgentInterface, workDir string, store *session.Store, sess *session.Session) *Model {
	header := ui.NewHeader(workDir)
	chat := ui.NewChat()
	chat.SetWelcomeContent(header.WelcomeContent())

	// If resuming a session, restore the conversation.
	if sess != nil && len(sess.Messages) > 0 {
		ag.SetMessages(sess.Messages)
		// Show a message indicating the session was restored.
		chat.AddSystemMessage("Session restored (" + sess.ID + ") with " +
			formatMessageCount(len(sess.Messages)) + " messages")
	}

	return &Model{
		agent:        ag,
		header:       header,
		footer:       ui.NewFooter(),
		chat:         chat,
		session:      sess,
		sessionStore: store,
	}
}

// formatMessageCount returns a human-readable message count.
func formatMessageCount(n int) string {
	if n == 1 {
		return "1"
	}
	return fmt.Sprintf("%d", n)
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return m.chat.Focus()
}

// saveSession persists the current conversation to disk.
func (m *Model) saveSession() {
	if m.sessionStore == nil || m.session == nil {
		return
	}

	m.session.SetMessages(m.agent.Messages())

	// Generate a title from the first user message if not set.
	if m.session.Title == "" && len(m.session.Messages) > 0 {
		m.session.Title = extractSessionTitle(m.session.Messages)
	}

	// Save asynchronously to avoid blocking the UI.
	go func() {
		_ = m.sessionStore.Save(m.session)
	}()
}

// extractSessionTitle generates a title from the first user message.
func extractSessionTitle(messages []anthropic.MessageParam) string {
	for _, msg := range messages {
		if string(msg.Role) == "user" {
			for _, block := range msg.Content {
				if text := extractTextContent(block); text != "" {
					// Truncate to first 50 characters.
					if len(text) > 50 {
						return text[:47] + "..."
					}
					return text
				}
			}
		}
	}
	return "Untitled"
}

// extractTextContent gets text from a content block union.
func extractTextContent(block anthropic.ContentBlockParamUnion) string {
	// Try to get text from the block.
	if block.OfText != nil {
		return block.OfText.Text
	}
	return ""
}
