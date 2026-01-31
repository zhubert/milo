package tool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	maxGrepResults = 1000
	maxFileSize    = 1 << 20 // 1 MB
	binaryCheckLen = 512
)

// GrepTool searches file contents for a regular expression.
// It implements ParallelSafeTool since it only reads file contents.
type GrepTool struct {
	WorkDir string
}

// IsParallelSafe returns true since grep operations don't modify state.
func (t *GrepTool) IsParallelSafe() bool { return true }

type grepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Include string `json:"include"`
}

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Description() string {
	return "Search file contents using a regular expression. " +
		"Returns matches in path:line:content format. " +
		"Skips .git directories, binary files, and files over 1MB. " +
		"Results capped at 1000 matching lines."
}

func (t *GrepTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression to search for (Go regexp syntax)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute directory to search in. Defaults to working directory if not provided.",
			},
			"include": map[string]any{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g., *.go, *.ts). Uses filepath.Match against filename.",
			},
		},
		Required: []string{"pattern"},
	}
}

func (t *GrepTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing grep input: %w", err)
	}

	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return Result{Output: fmt.Sprintf("invalid regex: %s", err), IsError: true}, nil
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

	var b strings.Builder
	matchCount := 0

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		if in.Include != "" {
			matched, matchErr := filepath.Match(in.Include, d.Name())
			if matchErr != nil {
				return nil
			}
			if !matched {
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > maxFileSize {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		if isBinary(path) {
			return nil
		}

		count, scanErr := searchFile(path, rel, re, &b, maxGrepResults-matchCount)
		if scanErr != nil {
			return nil
		}
		matchCount += count

		if matchCount >= maxGrepResults {
			return filepath.SkipAll
		}

		return nil
	})
	if walkErr != nil {
		return Result{Output: fmt.Sprintf("error walking directory: %s", walkErr), IsError: true}, nil
	}

	if matchCount == 0 {
		return Result{Output: "no matches found"}, nil
	}

	output := b.String()
	if matchCount >= maxGrepResults {
		output += fmt.Sprintf("\n(results capped at %d matches)\n", maxGrepResults)
	}

	return Result{Output: output}, nil
}

// searchFile scans a file line by line, appending matches to b. It returns
// the number of matches written and stops after limit matches.
func searchFile(path, rel string, re *regexp.Regexp, b *strings.Builder, limit int) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("opening file: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	count := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			fmt.Fprintf(b, "%s:%d:%s\n", rel, lineNum, line)
			count++
			if count >= limit {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("scanning file: %w", err)
	}

	return count, nil
}

// isBinary checks whether a file appears to be binary by looking for null
// bytes in the first 512 bytes.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, binaryCheckLen)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}

	return bytes.ContainsRune(buf[:n], 0)
}
