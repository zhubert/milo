package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

const maxGlobResults = 1000

// GlobTool searches for files matching a glob pattern.
// It implements ParallelSafeTool since it only reads directory listings.
type GlobTool struct {
	WorkDir string
}

// IsParallelSafe returns true since glob operations don't modify state.
func (t *GlobTool) IsParallelSafe() bool { return true }

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern. Supports ** for recursive directory matching. " +
		"Returns matching file paths sorted alphabetically, one per line. " +
		"Skips .git directories. Results capped at 1000 files."
}

func (t *GlobTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern supporting ** for recursive matching (e.g., **/*.go, src/**/*.ts)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute directory to search in. Defaults to working directory if not provided.",
			},
		},
		Required: []string{"pattern"},
	}
}

func (t *GlobTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in globInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing glob input: %w", err)
	}

	root := t.WorkDir
	if in.Path != "" {
		if !filepath.IsAbs(in.Path) {
			return Result{Output: "path must be an absolute path", IsError: true}, nil
		}
		root = in.Path
	}

	if root == "" {
		return Result{Output: "no working directory set and no path provided", IsError: true}, nil
	}

	segments := splitPattern(in.Pattern)

	var matches []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip files/dirs we can't access
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		if matchGlob(segments, strings.Split(rel, string(filepath.Separator))) {
			matches = append(matches, rel)
			if len(matches) >= maxGlobResults {
				return filepath.SkipAll
			}
		}

		return nil
	})
	if walkErr != nil {
		return Result{Output: fmt.Sprintf("error walking directory: %s", walkErr), IsError: true}, nil
	}

	if len(matches) == 0 {
		return Result{Output: "no files matched"}, nil
	}

	sort.Strings(matches)

	var b strings.Builder
	for _, m := range matches {
		fmt.Fprintln(&b, m)
	}
	if len(matches) >= maxGlobResults {
		fmt.Fprintf(&b, "\n(results capped at %d files)\n", maxGlobResults)
	}

	return Result{Output: b.String()}, nil
}

// splitPattern splits a glob pattern into path segments.
func splitPattern(pattern string) []string {
	// Normalize to forward slashes for splitting, then work with OS separator.
	pattern = filepath.ToSlash(pattern)
	return strings.Split(pattern, "/")
}

// matchGlob matches path segments against pattern segments, supporting ** for
// zero or more directory levels.
func matchGlob(pattern, path []string) bool {
	return matchSegments(pattern, path, 0, 0)
}

func matchSegments(pattern, path []string, pi, si int) bool {
	for pi < len(pattern) && si < len(path) {
		if pattern[pi] == "**" {
			// ** matches zero or more path segments.
			// Try matching the rest of the pattern against every possible
			// suffix of the remaining path.
			for s := si; s <= len(path); s++ {
				if matchSegments(pattern, path, pi+1, s) {
					return true
				}
			}
			return false
		}

		matched, err := filepath.Match(pattern[pi], path[si])
		if err != nil || !matched {
			return false
		}
		pi++
		si++
	}

	// Consume trailing ** segments (they can match zero directories).
	for pi < len(pattern) && pattern[pi] == "**" {
		pi++
	}

	return pi == len(pattern) && si == len(path)
}
