package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zhubert/milo/internal/agent"
	"github.com/zhubert/milo/internal/permission"
	"github.com/zhubert/milo/internal/ui"
)

// AgentInterface defines the interface that the app needs from an agent.
type AgentInterface interface {
	ModelDisplayName() string
	Permissions() *permission.Checker
	SendMessage(ctx context.Context, msg string) <-chan agent.StreamChunk
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
}

// New creates the root app model.
func New(ag AgentInterface, workDir string) *Model {
	header := ui.NewHeader(workDir)
	chat := ui.NewChat()
	chat.SetWelcomeContent(header.WelcomeContent())

	return &Model{
		agent:  ag,
		header: header,
		footer: ui.NewFooter(),
		chat:   chat,
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return m.chat.Focus()
}
