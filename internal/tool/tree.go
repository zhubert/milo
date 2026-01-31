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

const (
	defaultTreeDepth = 4
	maxTreeDepth     = 10
	maxTreeEntries   = 500
)

// TreeTool displays directory structure as a tree.
// It implements ParallelSafeTool since it only reads data.
type TreeTool struct {
	WorkDir string
}

// IsParallelSafe returns true since tree operations don't modify state.
func (t *TreeTool) IsParallelSafe() bool { return true }

type treeInput struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
}

func (t *TreeTool) Name() string { return "tree" }

func (t *TreeTool) Description() string {
	return "Display directory structure as a tree. Shows files and directories recursively " +
		"with visual hierarchy. Useful for understanding project layout. " +
		"Skips .git, node_modules, and other common non-essential directories. " +
		"Default depth is 4, max is 10. Results capped at 500 entries."
}

func (t *TreeTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the root directory. Defaults to working directory if not provided.",
			},
			"depth": map[string]any{
				"type":        "integer",
				"description": "Maximum depth to traverse (default 4, max 10).",
			},
		},
		Required: []string{},
	}
}

func (t *TreeTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in treeInput
	if len(input) == 0 {
		input = []byte("{}")
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing tree input: %w", err)
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

	depth := in.Depth
	if depth <= 0 {
		depth = defaultTreeDepth
	}
	if depth > maxTreeDepth {
		depth = maxTreeDepth
	}

	info, err := os.Stat(dir)
	if err != nil {
		return Result{Output: fmt.Sprintf("error accessing path: %s", err), IsError: true}, nil
	}
	if !info.IsDir() {
		return Result{Output: "path is not a directory", IsError: true}, nil
	}

	var b strings.Builder
	count := 0
	truncated := false

	// Print root directory name
	fmt.Fprintln(&b, filepath.Base(dir)+"/")

	// Build tree recursively
	truncated = t.buildTree(&b, dir, "", depth, &count)

	if truncated {
		fmt.Fprintf(&b, "\n(output truncated at %d entries)\n", maxTreeEntries)
	}

	return Result{Output: b.String()}, nil
}

// shouldSkip returns true for directories that should be skipped.
func shouldSkip(name string) bool {
	skip := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"__pycache__":  true,
		".venv":        true,
		"venv":         true,
		".idea":        true,
		".vscode":      true,
		"dist":         true,
		"build":        true,
		".next":        true,
		".nuxt":        true,
		"target":       true, // Rust/Java build dir
	}
	return skip[name]
}

// buildTree recursively builds the tree structure.
// Returns true if output was truncated.
func (t *TreeTool) buildTree(b *strings.Builder, dir, prefix string, depth int, count *int) bool {
	if depth <= 0 {
		return false
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	// Filter and sort entries
	var filtered []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files (except some useful ones) and skipped dirs
		if strings.HasPrefix(name, ".") && name != ".github" && name != ".gitignore" {
			continue
		}
		if entry.IsDir() && shouldSkip(name) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Sort: directories first, then files, alphabetically within each group
	sort.Slice(filtered, func(i, j int) bool {
		iDir := filtered[i].IsDir()
		jDir := filtered[j].IsDir()
		if iDir != jDir {
			return iDir // directories first
		}
		return filtered[i].Name() < filtered[j].Name()
	})

	for i, entry := range filtered {
		if *count >= maxTreeEntries {
			return true
		}
		*count++

		isLast := i == len(filtered)-1
		name := entry.Name()

		// Choose the right connector
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		if entry.IsDir() {
			fmt.Fprintf(b, "%s%s%s/\n", prefix, connector, name)

			// Recurse with updated prefix
			newPrefix := prefix + "│   "
			if isLast {
				newPrefix = prefix + "    "
			}
			if t.buildTree(b, filepath.Join(dir, name), newPrefix, depth-1, count) {
				return true
			}
		} else {
			fmt.Fprintf(b, "%s%s%s\n", prefix, connector, name)
		}
	}

	return false
}
