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

// Result holds the output from a tool execution.
type Result struct {
	Output  string
	IsError bool
}
