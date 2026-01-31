package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepToolExecute(t *testing.T) {
	t.Parallel()

	// Set up a temp directory tree with searchable content:
	//   root/
	//     main.go    — contains "func main()"
	//     util.go    — contains "func helper()" and "func utility()"
	//     readme.txt — contains "see main for details"
	//     sub/
	//       lib.go   — contains "func libFunc()"
	//     .git/
	//       HEAD     — contains "ref: refs/heads/main"
	//     binary.bin — contains null bytes
	dir := t.TempDir()

	fileContents := map[string]string{
		"main.go":                      "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n",
		"util.go":                      "package main\n\nfunc helper() {}\n\nfunc utility() {}\n",
		"readme.txt":                   "see main for details\nsome other line\n",
		filepath.Join("sub", "lib.go"): "package sub\n\nfunc libFunc() {}\n",
		filepath.Join(".git", "HEAD"):  "ref: refs/heads/main\n",
	}

	for name, content := range fileContents {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("creating dir for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	// Write a binary file.
	binPath := filepath.Join(dir, "binary.bin")
	if err := os.WriteFile(binPath, []byte("hello\x00world"), 0644); err != nil {
		t.Fatalf("writing binary file: %v", err)
	}

	tool := &GrepTool{WorkDir: dir}

	tests := []struct {
		name        string
		input       grepInput
		isError     bool
		contains    []string
		notContains []string
		exact       string
	}{
		{
			name:  "simple regex match",
			input: grepInput{Pattern: "func.*()"},
			contains: []string{
				"main.go:3:func main()",
				"util.go:3:func helper()",
				"util.go:5:func utility()",
				filepath.Join("sub", "lib.go") + ":3:func libFunc()",
			},
		},
		{
			name:  "match with include filter",
			input: grepInput{Pattern: "main", Include: "*.txt"},
			contains: []string{
				"readme.txt:1:see main for details",
			},
			notContains: []string{
				"main.go",
			},
		},
		{
			name:    "invalid regex returns IsError",
			input:   grepInput{Pattern: "[invalid"},
			isError: true,
		},
		{
			name:  "no matches returns message",
			input: grepInput{Pattern: "zzzznotfound"},
			exact: "no matches found",
		},
		{
			name:  "binary files skipped",
			input: grepInput{Pattern: "hello"},
			notContains: []string{
				"binary.bin",
			},
			contains: []string{
				"main.go",
			},
		},
		{
			name:  "output format is file:line:content",
			input: grepInput{Pattern: "^package main$", Include: "*.go"},
			contains: []string{
				"main.go:1:package main",
				"util.go:1:package main",
			},
		},
		{
			name:    "relative path rejected",
			input:   grepInput{Pattern: "func", Path: "relative/path"},
			isError: true,
		},
		{
			name:  "skips .git directory",
			input: grepInput{Pattern: "refs/heads/main"},
			notContains: []string{
				".git",
			},
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
