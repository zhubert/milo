package tool

import (
	"context"
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
)

// Tool defines the interface that all agent tools must implement.
type Tool interface {
	Name() string
	Description() string
	InputSchema() anthropic.ToolInputSchemaParam
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

// InputNormalizer is an optional interface for tools that need to normalize
// their input before permission checking and execution. For example, the bash
// tool strips redundant "cd <workdir> &&" prefixes since commands already run
// in the working directory.
type InputNormalizer interface {
	NormalizeInput(input json.RawMessage) json.RawMessage
}

// Result holds the output from a tool execution.
type Result struct {
	Output  string
	IsError bool
}
