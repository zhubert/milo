package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// ReadTool reads file contents with optional offset and limit.
// It implements ParallelSafeTool since it only reads data.
type ReadTool struct{}

// IsParallelSafe returns true since read operations don't modify state.
func (t *ReadTool) IsParallelSafe() bool { return true }

type readInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

func (t *ReadTool) Name() string { return "read" }

func (t *ReadTool) Description() string {
	return "Read the contents of a file. Returns the file content with line numbers. " +
		"Supports optional offset (1-based line number to start from) and limit (number of lines to read)."
}

func (t *ReadTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to read",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "The 1-based line number to start reading from",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "The number of lines to read",
			},
		},
		Required: []string{"file_path"},
	}
}

func (t *ReadTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in readInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing read input: %w", err)
	}

	if !filepath.IsAbs(in.FilePath) {
		return Result{Output: "file_path must be an absolute path", IsError: true}, nil
	}

	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return Result{Output: fmt.Sprintf("error reading file: %s", err), IsError: true}, nil
	}

	lines := strings.Split(string(data), "\n")

	// Apply offset (1-based).
	start := 0
	if in.Offset > 0 {
		start = in.Offset - 1
	}
	if start >= len(lines) {
		start = len(lines)
	}

	end := len(lines)
	if in.Limit > 0 {
		end = start + in.Limit
		if end > len(lines) {
			end = len(lines)
		}
	}

	lines = lines[start:end]

	// Format with line numbers (cat -n style).
	var b strings.Builder
	for i, line := range lines {
		lineNum := start + i + 1
		fmt.Fprintf(&b, "%6d\t%s\n", lineNum, line)
	}

	return Result{Output: b.String()}, nil
}
