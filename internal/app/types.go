package app

import (
	"github.com/zhubert/looper/internal/agent"
)

// StreamChunkMsg wraps an agent StreamChunk as a bubbletea message.
type StreamChunkMsg struct {
	Chunk agent.StreamChunk
}

// SendMsg signals that the user wants to send their input.
type SendMsg struct {
	Text string
}
