package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/anthropic-sdk-go"
)

// MoveTool moves or renames files and directories.
type MoveTool struct{}

type moveInput struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

func (t *MoveTool) Name() string { return "move" }

func (t *MoveTool) Description() string {
	return "Move or rename files and directories. Creates parent directories for the destination if they don't exist. " +
		"Can be used to rename files/directories (same parent directory) or move them to a different location."
}

func (t *MoveTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"source": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file or directory to move/rename",
			},
			"destination": map[string]any{
				"type":        "string",
				"description": "The absolute path for the new location/name",
			},
		},
		Required: []string{"source", "destination"},
	}
}

func (t *MoveTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in moveInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing move input: %w", err)
	}

	if !filepath.IsAbs(in.Source) {
		return Result{Output: "source must be an absolute path", IsError: true}, nil
	}

	if !filepath.IsAbs(in.Destination) {
		return Result{Output: "destination must be an absolute path", IsError: true}, nil
	}

	// Check if source exists
	sourceInfo, err := os.Stat(in.Source)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Output: fmt.Sprintf("source does not exist: %s", in.Source), IsError: true}, nil
		}
		return Result{Output: fmt.Sprintf("error accessing source: %s", err), IsError: true}, nil
	}

	// Create parent directory for destination if it doesn't exist
	destDir := filepath.Dir(in.Destination)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return Result{Output: fmt.Sprintf("error creating destination directory: %s", err), IsError: true}, nil
	}

	// Check if destination already exists
	if _, err := os.Stat(in.Destination); err == nil {
		return Result{Output: fmt.Sprintf("destination already exists: %s", in.Destination), IsError: true}, nil
	}

	// Perform the move
	if err := os.Rename(in.Source, in.Destination); err != nil {
		return Result{Output: fmt.Sprintf("error moving/renaming: %s", err), IsError: true}, nil
	}

	var itemType string
	if sourceInfo.IsDir() {
		itemType = "directory"
	} else {
		itemType = "file"
	}

	return Result{Output: fmt.Sprintf("Successfully moved %s from %s to %s", itemType, in.Source, in.Destination)}, nil
}
