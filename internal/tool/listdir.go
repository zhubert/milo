package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

const maxListDirEntries = 1000

// ListDirTool lists directory contents.
// It implements ParallelSafeTool since it only reads data.
type ListDirTool struct {
	WorkDir string
}

// IsParallelSafe returns true since list operations don't modify state.
func (t *ListDirTool) IsParallelSafe() bool { return true }

type listDirInput struct {
	Path string `json:"path"`
}

func (t *ListDirTool) Name() string { return "list_dir" }

func (t *ListDirTool) Description() string {
	return "List directory contents. Returns files and subdirectories with type indicators " +
		"(/ suffix for directories). Results are sorted alphabetically. " +
		"Skips .git directory contents. Results capped at 1000 entries."
}

func (t *ListDirTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the directory to list. Defaults to working directory if not provided.",
			},
		},
		Required: []string{},
	}
}

func (t *ListDirTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in listDirInput
	// Handle empty input - when no required fields, API may send empty string
	if len(input) == 0 {
		input = []byte("{}")
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing list_dir input: %w", err)
	}

	dir := t.WorkDir
	if in.Path != "" {
		if !filepath.IsAbs(in.Path) {
			return Result{Output: "path must be an absolute path", IsError: true}, nil
		}
		dir = in.Path
	}

	if dir == "" {
		return Result{Output: "no working directory set and no path provided", IsError: true}, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return Result{Output: fmt.Sprintf("error reading directory: %s", err), IsError: true}, nil
	}

	var results []string
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}

		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		results = append(results, name)

		if len(results) >= maxListDirEntries {
			break
		}
	}

	if len(results) == 0 {
		return Result{Output: "(empty directory)"}, nil
	}

	sort.Strings(results)

	var b strings.Builder
	for _, name := range results {
		fmt.Fprintln(&b, name)
	}
	if len(results) >= maxListDirEntries {
		fmt.Fprintf(&b, "\n(results capped at %d entries)\n", maxListDirEntries)
	}

	return Result{Output: b.String()}, nil
}
