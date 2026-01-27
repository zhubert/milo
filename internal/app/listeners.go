package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/zhubert/looper/internal/agent"
)

// listenForChunks returns a command that reads from the agent's stream channel
// and converts each chunk into a bubbletea message.
func listenForChunks(ch <-chan agent.StreamChunk) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			// Channel closed.
			return StreamChunkMsg{Chunk: agent.StreamChunk{Type: agent.ChunkDone}}
		}
		return StreamChunkMsg{Chunk: chunk}
	}
}
