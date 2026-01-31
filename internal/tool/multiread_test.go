package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMultiReadToolExecute(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create test files
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")

	content1 := "line1\nline2\nline3\n"
	content2 := "alpha\nbeta\ngamma\n"

	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	tool := &MultiReadTool{}

	t.Run("read multiple files successfully", func(t *testing.T) {
		t.Parallel()

		input := multiReadInput{
			Files: []fileSpec{
				{FilePath: file1},
				{FilePath: file2},
			},
		}
		inputJSON, _ := json.Marshal(input)

		result, err := tool.Execute(context.Background(), inputJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Errorf("unexpected IsError: %s", result.Output)
		}

		// Verify both files are in output
		if !strings.Contains(result.Output, "=== "+file1+" ===") {
			t.Errorf("output should contain file1 header")
		}
		if !strings.Contains(result.Output, "=== "+file2+" ===") {
			t.Errorf("output should contain file2 header")
		}
		if !strings.Contains(result.Output, "line1") {
			t.Errorf("output should contain file1 content")
		}
		if !strings.Contains(result.Output, "alpha") {
			t.Errorf("output should contain file2 content")
		}
	})

	t.Run("preserves file order", func(t *testing.T) {
		t.Parallel()

		input := multiReadInput{
			Files: []fileSpec{
				{FilePath: file2},
				{FilePath: file1},
			},
		}
		inputJSON, _ := json.Marshal(input)

		result, err := tool.Execute(context.Background(), inputJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// file2 should appear before file1 in output
		idx1 := strings.Index(result.Output, "=== "+file1+" ===")
		idx2 := strings.Index(result.Output, "=== "+file2+" ===")
		if idx2 >= idx1 {
			t.Errorf("expected file2 before file1 in output")
		}
	})
}

func TestMultiReadWithOffsetLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tool := &MultiReadTool{}

	tests := []struct {
		name        string
		offset      int
		limit       int
		contains    []string
		notContains []string
	}{
		{
			name:     "no offset or limit",
			offset:   0,
			limit:    0,
			contains: []string{"line1", "line2", "line3", "line4", "line5"},
		},
		{
			name:        "with offset",
			offset:      3,
			limit:       0,
			contains:    []string{"line3", "line4", "line5"},
			notContains: []string{"line1", "line2"},
		},
		{
			name:        "with limit",
			offset:      0,
			limit:       2,
			contains:    []string{"line1", "line2"},
			notContains: []string{"line3", "line4"},
		},
		{
			name:        "with offset and limit",
			offset:      2,
			limit:       2,
			contains:    []string{"line2", "line3"},
			notContains: []string{"line1", "line4", "line5"},
		},
		{
			name:     "offset beyond file",
			offset:   100,
			limit:    0,
			contains: []string{}, // Output should just have the header
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := multiReadInput{
				Files: []fileSpec{
					{FilePath: testFile, Offset: tt.offset, Limit: tt.limit},
				},
			}
			inputJSON, _ := json.Marshal(input)

			result, err := tool.Execute(context.Background(), inputJSON)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, s := range tt.contains {
				if !strings.Contains(result.Output, s) {
					t.Errorf("output should contain %q", s)
				}
			}
			for _, s := range tt.notContains {
				if strings.Contains(result.Output, s) {
					t.Errorf("output should not contain %q", s)
				}
			}
		})
	}
}

func TestMultiReadErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a directory to test directory path error
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	tool := &MultiReadTool{}

	tests := []struct {
		name     string
		files    []fileSpec
		errMsg   string
		isError  bool
	}{
		{
			name: "nonexistent file",
			files: []fileSpec{
				{FilePath: filepath.Join(dir, "nonexistent.txt")},
			},
			errMsg:  "file does not exist",
			isError: true,
		},
		{
			name: "directory path",
			files: []fileSpec{
				{FilePath: subDir},
			},
			errMsg:  "path is a directory",
			isError: true,
		},
		{
			name: "relative path",
			files: []fileSpec{
				{FilePath: "relative/path.txt"},
			},
			errMsg:  "must be an absolute path",
			isError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := multiReadInput{Files: tt.files}
			inputJSON, _ := json.Marshal(input)

			result, err := tool.Execute(context.Background(), inputJSON)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError != tt.isError {
				t.Errorf("IsError: got %v, want %v", result.IsError, tt.isError)
			}
			if !strings.Contains(result.Output, tt.errMsg) {
				t.Errorf("output should contain %q, got: %s", tt.errMsg, result.Output)
			}
		})
	}
}

func TestMultiReadEmptyInput(t *testing.T) {
	t.Parallel()

	tool := &MultiReadTool{}

	input := multiReadInput{Files: []fileSpec{}}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError for empty input")
	}
	if !strings.Contains(result.Output, "no files specified") {
		t.Errorf("expected 'no files specified' error, got: %s", result.Output)
	}
}

func TestMultiReadPartialErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existingFile := filepath.Join(dir, "exists.txt")
	content := "this file exists\n"
	if err := os.WriteFile(existingFile, []byte(content), 0644); err != nil {
		t.Fatalf("writing existing file: %v", err)
	}

	tool := &MultiReadTool{}

	// Mix of existing and non-existing files
	input := multiReadInput{
		Files: []fileSpec{
			{FilePath: existingFile},
			{FilePath: filepath.Join(dir, "missing.txt")},
		},
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have IsError true because at least one file failed
	if !result.IsError {
		t.Error("expected IsError when some files fail")
	}

	// But should still have content from the successful file
	if !strings.Contains(result.Output, "this file exists") {
		t.Error("output should contain content from successful file")
	}
	if !strings.Contains(result.Output, "file does not exist") {
		t.Error("output should contain error for missing file")
	}
}

func TestMultiReadInvalidJSON(t *testing.T) {
	t.Parallel()

	tool := &MultiReadTool{}

	result, err := tool.Execute(context.Background(), []byte("invalid json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parsing multi_read input") {
		t.Errorf("expected parsing error, got: %v", err)
	}
	_ = result // Result is not important when error is returned
}

func TestMultiReadIsParallelSafe(t *testing.T) {
	t.Parallel()

	tool := &MultiReadTool{}
	if !tool.IsParallelSafe() {
		t.Error("MultiReadTool should be parallel safe")
	}
}

func TestMultiReadMetadata(t *testing.T) {
	t.Parallel()

	tool := &MultiReadTool{}

	if tool.Name() != "multi_read" {
		t.Errorf("Name: got %q, want %q", tool.Name(), "multi_read")
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	schema := tool.InputSchema()
	if schema.Properties == nil {
		t.Error("InputSchema should have properties")
	}
	if schema.Required == nil || len(schema.Required) == 0 {
		t.Error("InputSchema should have required fields")
	}
}

func TestMultiReadLineNumbers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	content := "first\nsecond\nthird\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tool := &MultiReadTool{}

	input := multiReadInput{
		Files: []fileSpec{
			{FilePath: testFile, Offset: 2, Limit: 1},
		},
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With offset 2 (1-based), we start at line 2
	// Output should show line number 2
	if !strings.Contains(result.Output, "2\t") {
		t.Errorf("output should show line number 2, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "second") {
		t.Errorf("output should contain 'second', got: %s", result.Output)
	}
}
