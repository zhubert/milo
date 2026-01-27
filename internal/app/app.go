package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/zhubert/looper/internal/agent"
	"github.com/zhubert/looper/internal/ui"
)

// Model is the top-level bubbletea model for the looper TUI.
type Model struct {
	agent  *agent.Agent
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
func New(ag *agent.Agent, workDir string) *Model {
	return &Model{
		agent:  ag,
		header: ui.NewHeader(workDir),
		footer: ui.NewFooter(),
		chat:   ui.NewChat(),
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return m.chat.Focus()
}
