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

// UndoTool reverts recent file changes made by write and edit tools.
type UndoTool struct {
	// History is the file history to undo from. If nil, uses DefaultFileHistory.
	History *FileHistory
}

type undoInput struct {
	FilePath string `json:"file_path"` // optional: undo changes to a specific file
}

func (t *UndoTool) Name() string { return "undo" }

func (t *UndoTool) Description() string {
	return "Undo the most recent file change. If file_path is provided, undoes the most recent change to that specific file. " +
		"Otherwise, undoes the most recent change across all files. " +
		"Can restore deleted content or remove newly created files."
}

func (t *UndoTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Optional: the absolute path to a specific file to undo changes for. If omitted, undoes the most recent change to any file.",
			},
		},
		Required: []string{},
	}
}

func (t *UndoTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in undoInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing undo input: %w", err)
	}

	// Validate file_path if provided.
	if in.FilePath != "" && !filepath.IsAbs(in.FilePath) {
		return Result{Output: "file_path must be an absolute path", IsError: true}, nil
	}

	history := t.History
	if history == nil {
		history = DefaultFileHistory
	}

	var change *FileChange
	if in.FilePath != "" {
		change = history.PopChange(in.FilePath)
		if change == nil {
			return Result{Output: fmt.Sprintf("no undo history for file: %s", in.FilePath), IsError: true}, nil
		}
	} else {
		change = history.PopMostRecent()
		if change == nil {
			return Result{Output: "no changes to undo", IsError: true}, nil
		}
	}

	// Restore the file to its previous state.
	if change.Existed {
		// File existed before - restore its content.
		dir := filepath.Dir(change.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return Result{Output: fmt.Sprintf("error creating directories: %s", err), IsError: true}, nil
		}
		if err := os.WriteFile(change.FilePath, change.OldContent, 0644); err != nil {
			return Result{Output: fmt.Sprintf("error restoring file: %s", err), IsError: true}, nil
		}
		return Result{Output: fmt.Sprintf("Restored %s to previous state (from %s: %s)",
			change.FilePath, change.ToolName, change.Description)}, nil
	}

	// File didn't exist before - remove it.
	if err := os.Remove(change.FilePath); err != nil {
		if os.IsNotExist(err) {
			return Result{Output: fmt.Sprintf("File already removed: %s", change.FilePath)}, nil
		}
		return Result{Output: fmt.Sprintf("error removing file: %s", err), IsError: true}, nil
	}
	return Result{Output: fmt.Sprintf("Removed %s (was created by %s)", change.FilePath, change.ToolName)}, nil
}

// ListUndoHistory returns a formatted string of all available undo operations.
func ListUndoHistory() string {
	changes := DefaultFileHistory.GetAllChanges()
	if len(changes) == 0 {
		return "No undo history available."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Undo history (%d changes):\n", len(changes)))
	for i, c := range changes {
		status := "modified"
		if !c.Existed {
			status = "created"
		}
		sb.WriteString(fmt.Sprintf("  %d. [%s] %s (%s by %s)\n",
			i+1, c.Timestamp.Format("15:04:05"), c.FilePath, status, c.ToolName))
	}
	return sb.String()
}
