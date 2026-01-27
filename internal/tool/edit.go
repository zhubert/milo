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

// EditTool performs string-replacement edits on files.
type EditTool struct{}

type editInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Description() string {
	return "Perform a string replacement edit on a file. Finds old_string in the file and replaces it with new_string. " +
		"Fails if old_string is not found or appears more than once (unless replace_all is true)."
}

func (t *EditTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to edit",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "The exact string to find and replace",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "The string to replace old_string with",
			},
			"replace_all": map[string]any{
				"type":        "boolean",
				"description": "If true, replace all occurrences. If false (default), fail when old_string appears more than once.",
			},
		},
		Required: []string{"file_path", "old_string", "new_string"},
	}
}

func (t *EditTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in editInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing edit input: %w", err)
	}

	if !filepath.IsAbs(in.FilePath) {
		return Result{Output: "file_path must be an absolute path", IsError: true}, nil
	}

	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return Result{Output: fmt.Sprintf("error reading file: %s", err), IsError: true}, nil
	}

	content := string(data)
	count := strings.Count(content, in.OldString)

	if count == 0 {
		return Result{Output: "old_string not found in file", IsError: true}, nil
	}

	if count > 1 && !in.ReplaceAll {
		return Result{
			Output:  fmt.Sprintf("old_string found %d times — use replace_all to replace all occurrences", count),
			IsError: true,
		}, nil
	}

	var newContent string
	if in.ReplaceAll {
		newContent = strings.ReplaceAll(content, in.OldString, in.NewString)
	} else {
		newContent = strings.Replace(content, in.OldString, in.NewString, 1)
	}

	if err := os.WriteFile(in.FilePath, []byte(newContent), 0644); err != nil {
		return Result{Output: fmt.Sprintf("error writing file: %s", err), IsError: true}, nil
	}

	return Result{Output: fmt.Sprintf("Successfully replaced %d occurrence(s) in %s", count, in.FilePath)}, nil
}
