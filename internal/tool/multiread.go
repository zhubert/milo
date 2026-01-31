package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
)

// MultiReadTool reads multiple files in a single call.
// It implements ParallelSafeTool since it only reads data.
type MultiReadTool struct{}

// IsParallelSafe returns true since read operations don't modify state.
func (t *MultiReadTool) IsParallelSafe() bool { return true }

type fileSpec struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type multiReadInput struct {
	Files []fileSpec `json:"files"`
}

func (t *MultiReadTool) Name() string { return "multi_read" }

func (t *MultiReadTool) Description() string {
	return "Read multiple files in a single call. ALWAYS use this instead of multiple read calls when you need to read 2+ files. " +
		"Each file can have optional offset (1-based line number) and limit (number of lines)."
}

func (t *MultiReadTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"files": map[string]any{
				"type":        "array",
				"description": "Array of files to read",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
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
					"required": []string{"file_path"},
				},
			},
		},
		Required: []string{"files"},
	}
}

func (t *MultiReadTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in multiReadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing multi_read input: %w", err)
	}

	if len(in.Files) == 0 {
		return Result{Output: "no files specified", IsError: true}, nil
	}

	// Read files in parallel
	type fileResult struct {
		index   int
		path    string
		content string
		err     string
	}

	results := make([]fileResult, len(in.Files))
	var wg sync.WaitGroup

	for i, spec := range in.Files {
		wg.Add(1)
		go func(idx int, s fileSpec) {
			defer wg.Done()
			results[idx] = fileResult{index: idx, path: s.FilePath}

			if !filepath.IsAbs(s.FilePath) {
				results[idx].err = "file_path must be an absolute path"
				return
			}

			// Check if path is a directory
			info, err := os.Stat(s.FilePath)
			if err != nil {
				if os.IsNotExist(err) {
					results[idx].err = "file does not exist"
				} else {
					results[idx].err = fmt.Sprintf("error accessing file: %s", err)
				}
				return
			}
			if info.IsDir() {
				results[idx].err = "path is a directory, use list_dir instead"
				return
			}

			data, err := os.ReadFile(s.FilePath)
			if err != nil {
				results[idx].err = fmt.Sprintf("error reading file: %s", err)
				return
			}

			lines := strings.Split(string(data), "\n")

			// Apply offset (1-based)
			start := 0
			if s.Offset > 0 {
				start = s.Offset - 1
			}
			if start > len(lines) {
				start = len(lines)
			}

			end := len(lines)
			if s.Limit > 0 {
				end = start + s.Limit
				if end > len(lines) {
					end = len(lines)
				}
			}

			lines = lines[start:end]

			// Format with line numbers
			var b strings.Builder
			for i, line := range lines {
				lineNum := start + i + 1
				fmt.Fprintf(&b, "%6d\t%s\n", lineNum, line)
			}
			results[idx].content = b.String()
		}(i, spec)
	}

	wg.Wait()

	// Combine results in order
	var output strings.Builder
	hasErrors := false

	for _, r := range results {
		output.WriteString(fmt.Sprintf("=== %s ===\n", r.path))
		if r.err != "" {
			output.WriteString(fmt.Sprintf("ERROR: %s\n", r.err))
			hasErrors = true
		} else {
			output.WriteString(r.content)
		}
		output.WriteString("\n")
	}

	return Result{Output: output.String(), IsError: hasErrors}, nil
}
