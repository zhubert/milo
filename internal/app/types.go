package app

import "github.com/zhubert/milo/internal/agent"

// SendMsg signals that the user wants to send their input.
type SendMsg struct {
	Text string
}

// StreamChunkMsg wraps a chunk from the agent stream.
type StreamChunkMsg struct {
	Chunk agent.StreamChunk
}
