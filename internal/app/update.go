package app

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/zhubert/milo/internal/agent"
	"github.com/zhubert/milo/internal/ui"
)

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		ctx := ui.GetViewContext()
		ctx.UpdateTerminalSize(msg.Width, msg.Height)
		m.chat.SetSize(msg.Width, ctx.ContentHeight)

	case tea.KeyPressMsg:
		// Handle permission prompt keys first.
		if m.permPending {
			return m.handlePermissionKey(msg)
		}

		switch msg.String() {
		case "ctrl+c":
			m.cancelStream()
			m.quitting = true
			return m, tea.Quit

		case "escape":
			if m.streaming {
				m.cancelStream()
				m.chat.FinishStreaming()
				m.streaming = false
				m.footer.SetFlash(ui.ErrorStyle.Render("Cancelled"))
				cmds = append(cmds, ui.FlashTick())
			}

		case "enter":
			if !m.streaming && !m.permPending {
				text := strings.TrimSpace(m.chat.InputValue())
				if text != "" {
					m.chat.ResetInput()
					// Check for slash commands
					if strings.HasPrefix(text, "/") {
						cmds = append(cmds, m.handleSlashCommand(text))
					} else {
						cmds = append(cmds, func() tea.Msg {
							return SendMsg{Text: text}
						})
					}
				}
			}
		}

	case SendMsg:
		cmds = append(cmds, m.startAgent(msg.Text))

	case StreamChunkMsg:
		cmds = append(cmds, m.handleStreamChunk(msg.Chunk))

	case ui.FlashTickMsg:
		m.footer.ClearFlash()
	}

	// Delegate to chat component.
	chat, cmd := m.chat.Update(msg)
	m.chat = chat
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) startAgent(text string) tea.Cmd {
	m.streaming = true
	m.chat.AddUserMessage(text)
	m.chat.SetWaiting(true)

	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	ch := m.agent.SendMessage(ctx, text)
	m.streamCh = ch

	return tea.Batch(listenForChunks(ch), ui.StopwatchTick())
}

func (m *Model) cancelStream() {
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
}

func (m *Model) handleStreamChunk(chunk agent.StreamChunk) tea.Cmd {
	switch chunk.Type {
	case agent.ChunkText:
		m.chat.AppendStreaming(chunk.Text)
		return listenForChunks(m.streamCh)

	case agent.ChunkToolUse:
		m.chat.AppendToolUse(chunk.ToolName, chunk.ToolInput)
		return listenForChunks(m.streamCh)

	case agent.ChunkToolResult:
		if chunk.Result != nil {
			m.chat.AppendToolResult(chunk.ToolName, chunk.Result.Output, chunk.Result.IsError)
		}
		return listenForChunks(m.streamCh)

	case agent.ChunkPermissionRequest:
		m.permPending = true
		m.permToolName = chunk.ToolName
		m.footer.SetPermissionMode(true)
		m.chat.Blur()
		// Don't listen for next chunk yet — we block the agent goroutine
		// until the user responds.
		return nil

	case agent.ChunkDone:
		m.streaming = false
		m.chat.FinishStreaming()
		m.streamCancel = nil
		m.footer.SetFlash(ui.SuccessStyle.Render("Done"))
		return ui.FlashTick()

	case agent.ChunkError:
		m.streaming = false
		m.chat.FinishStreaming()
		m.streamCancel = nil
		errMsg := "Something went wrong"
		if chunk.Err != nil {
			errMsg = chunk.Err.Error()
		}
		m.chat.AddErrorMessage(errMsg)
		return nil
	}

	return nil
}

func (m *Model) handlePermissionKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var resp agent.PermissionResponse

	switch msg.String() {
	case "y":
		resp = agent.PermissionGranted
	case "a":
		resp = agent.PermissionGrantedAlways
	case "n":
		resp = agent.PermissionDenied
	case "escape", "ctrl+c":
		resp = agent.PermissionDenied
	default:
		return m, nil // Ignore other keys.
	}

	m.permPending = false
	m.permToolName = ""
	m.footer.SetPermissionMode(false)

	// Send permission response to the agent (non-blocking).
	m.agent.PermResp <- resp

	// Resume listening for chunks and refocus input.
	return m, tea.Batch(listenForChunks(m.streamCh), m.chat.Focus())
}
