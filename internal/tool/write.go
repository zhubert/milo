package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/anthropic-sdk-go"
)

// WriteTool writes content to a file, creating parent directories as needed.
// It implements FileAccessor to enable conflict detection.
type WriteTool struct {
	// History is the file history to record changes to. If nil, uses DefaultFileHistory.
	History *FileHistory
}

// GetFilePath extracts the target file path from the input.
func (t *WriteTool) GetFilePath(input json.RawMessage) string {
	return ExtractFilePath(input)
}

// IsWriteOperation returns true since this tool modifies files.
func (t *WriteTool) IsWriteOperation() bool { return true }

type writeInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (t *WriteTool) Name() string { return "write" }

func (t *WriteTool) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist, or overwrites it if it does. " +
		"Parent directories are created automatically."
}

func (t *WriteTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to write",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		Required: []string{"file_path", "content"},
	}
}

func (t *WriteTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in writeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing write input: %w", err)
	}

	if !filepath.IsAbs(in.FilePath) {
		return Result{Output: "file_path must be an absolute path", IsError: true}, nil
	}

	dir := filepath.Dir(in.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return Result{Output: fmt.Sprintf("error creating directories: %s", err), IsError: true}, nil
	}

	// Record file state before modification for undo support.
	history := t.History
	if history == nil {
		history = DefaultFileHistory
	}
	if err := history.RecordChange(in.FilePath, "write", "write file"); err != nil {
		return Result{Output: fmt.Sprintf("error recording file history: %s", err), IsError: true}, nil
	}

	if err := os.WriteFile(in.FilePath, []byte(in.Content), 0644); err != nil {
		return Result{Output: fmt.Sprintf("error writing file: %s", err), IsError: true}, nil
	}

	return Result{Output: fmt.Sprintf("Successfully wrote %d bytes to %s", len(in.Content), in.FilePath)}, nil
}
