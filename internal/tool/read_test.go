package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadToolExecute(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	content := "line one\nline two\nline three\nline four\nline five\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tool := &ReadTool{}

	tests := []struct {
		name      string
		input     readInput
		wantErr   bool
		isError   bool
		contains  []string
		notContains []string
	}{
		{
			name:     "read entire file",
			input:    readInput{FilePath: testFile},
			contains: []string{"line one", "line two", "line three", "line four", "line five"},
		},
		{
			name:     "read with offset",
			input:    readInput{FilePath: testFile, Offset: 3},
			contains: []string{"line three", "line four"},
			notContains: []string{"line one", "line two"},
		},
		{
			name:     "read with limit",
			input:    readInput{FilePath: testFile, Limit: 2},
			contains: []string{"line one", "line two"},
			notContains: []string{"line three"},
		},
		{
			name:     "read with offset and limit",
			input:    readInput{FilePath: testFile, Offset: 2, Limit: 2},
			contains: []string{"line two", "line three"},
			notContains: []string{"line one", "line four"},
		},
		{
			name:    "nonexistent file",
			input:   readInput{FilePath: filepath.Join(dir, "nope.txt")},
			isError: true,
		},
		{
			name:    "relative path rejected",
			input:   readInput{FilePath: "relative/path.txt"},
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

			for _, s := range tt.contains {
				if !strings.Contains(result.Output, s) {
					t.Errorf("output should contain %q, got: %s", s, result.Output)
				}
			}
			for _, s := range tt.notContains {
				if strings.Contains(result.Output, s) {
					t.Errorf("output should not contain %q, got: %s", s, result.Output)
				}
			}
		})
	}
}
