package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffToolExecute(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create test files.
	file1 := filepath.Join(dir, "original.txt")
	file2 := filepath.Join(dir, "modified.txt")
	file3 := filepath.Join(dir, "identical.txt")

	original := "line one\nline two\nline three\nline four\nline five\n"
	modified := "line one\nline TWO\nline three\nline four\nline five\nnew line\n"

	if err := os.WriteFile(file1, []byte(original), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(modified), 0644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}
	if err := os.WriteFile(file3, []byte(original), 0644); err != nil {
		t.Fatalf("writing file3: %v", err)
	}

	tool := &DiffTool{}

	tests := []struct {
		name        string
		input       diffInput
		wantErr     bool
		isError     bool
		contains    []string
		notContains []string
		exact       string
	}{
		{
			name:     "compare two different files",
			input:    diffInput{FilePath1: file1, FilePath2: file2},
			contains: []string{"---", "+++", "-line two", "+line TWO", "+new line"},
		},
		{
			name:  "compare file with content",
			input: diffInput{FilePath1: file1, Content: modified},
			contains: []string{
				"--- " + file1,
				"+++ (provided content)",
				"-line two",
				"+line TWO",
			},
		},
		{
			name:  "identical files",
			input: diffInput{FilePath1: file1, FilePath2: file3},
			exact: "Files are identical",
		},
		{
			name:  "identical file and content",
			input: diffInput{FilePath1: file1, Content: original},
			exact: "Files are identical",
		},
		{
			name:    "relative path rejected for file_path_1",
			input:   diffInput{FilePath1: "relative/path.txt", FilePath2: file2},
			isError: true,
		},
		{
			name:    "relative path rejected for file_path_2",
			input:   diffInput{FilePath1: file1, FilePath2: "relative/path.txt"},
			isError: true,
		},
		{
			name:    "missing both file_path_2 and content",
			input:   diffInput{FilePath1: file1},
			isError: true,
		},
		{
			name:    "both file_path_2 and content provided",
			input:   diffInput{FilePath1: file1, FilePath2: file2, Content: "some content"},
			isError: true,
		},
		{
			name:    "nonexistent file_path_1",
			input:   diffInput{FilePath1: filepath.Join(dir, "nope.txt"), FilePath2: file2},
			isError: true,
		},
		{
			name:    "nonexistent file_path_2",
			input:   diffInput{FilePath1: file1, FilePath2: filepath.Join(dir, "nope.txt")},
			isError: true,
		},
		{
			name:  "custom context lines",
			input: diffInput{FilePath1: file1, FilePath2: file2, Context: 1},
			contains: []string{
				"@@ -",
				"-line two",
				"+line TWO",
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
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError != tt.isError {
				t.Errorf("IsError: got %v, want %v (output: %s)", result.IsError, tt.isError, result.Output)
			}

			if tt.exact != "" && result.Output != tt.exact {
				t.Errorf("output: got %q, want %q", result.Output, tt.exact)
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

func TestDiffToolIsParallelSafe(t *testing.T) {
	t.Parallel()

	tool := &DiffTool{}
	if !tool.IsParallelSafe() {
		t.Error("DiffTool should be parallel safe")
	}
}

func TestUnifiedDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text1    string
		text2    string
		context  int
		contains []string
	}{
		{
			name:     "single line change",
			text1:    "a\nb\nc\n",
			text2:    "a\nB\nc\n",
			context:  3,
			contains: []string{"-b", "+B"},
		},
		{
			name:     "addition at end",
			text1:    "a\nb\n",
			text2:    "a\nb\nc\n",
			context:  3,
			contains: []string{"+c"},
		},
		{
			name:     "deletion",
			text1:    "a\nb\nc\n",
			text2:    "a\nc\n",
			context:  3,
			contains: []string{"-b"},
		},
		{
			name:    "identical",
			text1:   "a\nb\nc\n",
			text2:   "a\nb\nc\n",
			context: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := unifiedDiff("file1", "file2", tt.text1, tt.text2, tt.context)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("diff should contain %q, got:\n%s", s, result)
				}
			}

			if len(tt.contains) == 0 && result != "" {
				t.Errorf("expected empty diff for identical files, got:\n%s", result)
			}
		})
	}
}
