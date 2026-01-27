package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobToolExecute(t *testing.T) {
	t.Parallel()

	// Set up a temp directory tree:
	//   root/
	//     a.txt
	//     b.go
	//     sub/
	//       c.txt
	//       deep/
	//         d.txt
	//     .git/
	//       config
	dir := t.TempDir()

	files := []string{
		"a.txt",
		"b.go",
		filepath.Join("sub", "c.txt"),
		filepath.Join("sub", "deep", "d.txt"),
		filepath.Join(".git", "config"),
	}
	for _, f := range files {
		full := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("creating dir for %s: %v", f, err)
		}
		if err := os.WriteFile(full, []byte("content"), 0644); err != nil {
			t.Fatalf("writing %s: %v", f, err)
		}
	}

	tool := &GlobTool{WorkDir: dir}

	tests := []struct {
		name        string
		input       globInput
		isError     bool
		contains    []string
		notContains []string
		exact       string // if non-empty, output must equal this exactly
	}{
		{
			name:     "match *.txt in flat directory",
			input:    globInput{Pattern: "*.txt"},
			contains: []string{"a.txt"},
			notContains: []string{
				"b.go",
				filepath.Join("sub", "c.txt"),
			},
		},
		{
			name:  "match **/*.txt recursively",
			input: globInput{Pattern: "**/*.txt"},
			contains: []string{
				"a.txt",
				filepath.Join("sub", "c.txt"),
				filepath.Join("sub", "deep", "d.txt"),
			},
			notContains: []string{"b.go"},
		},
		{
			name:  "match with explicit path",
			input: globInput{Pattern: "*.txt", Path: filepath.Join(dir, "sub")},
			contains: []string{
				"c.txt",
			},
			notContains: []string{"a.txt", "d.txt"},
		},
		{
			name:  "no matches returns message",
			input: globInput{Pattern: "*.rs"},
			exact: "no files matched",
		},
		{
			name:  "** matches all files",
			input: globInput{Pattern: "**"},
			contains: []string{
				"a.txt",
				"b.go",
				filepath.Join("sub", "c.txt"),
				filepath.Join("sub", "deep", "d.txt"),
			},
		},
		{
			name:  "skips .git directory",
			input: globInput{Pattern: "**"},
			notContains: []string{
				filepath.Join(".git", "config"),
			},
		},
		{
			name:    "relative path rejected",
			input:   globInput{Pattern: "*.go", Path: "relative/path"},
			isError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputJSON, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("marshaling input: %v", err)
			}

			result, err := tool.Execute(context.Background(), inputJSON)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError != tt.isError {
				t.Errorf("IsError: got %v, want %v (output: %s)", result.IsError, tt.isError, result.Output)
			}
			if tt.isError {
				return
			}

			if tt.exact != "" {
				if result.Output != tt.exact {
					t.Errorf("output: got %q, want %q", result.Output, tt.exact)
				}
				return
			}

			for _, s := range tt.contains {
				if !strings.Contains(result.Output, s) {
					t.Errorf("output should contain %q, got:\n%s", s, result.Output)
				}
			}
			for _, s := range tt.notContains {
				if strings.Contains(result.Output, s) {
					t.Errorf("output should not contain %q, got:\n%s", s, result.Output)
				}
			}
		})
	}
}
