package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDirToolExecute(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatalf("creating test subdir: %v", err)
	}

	tool := &ListDirTool{WorkDir: dir}

	tests := []struct {
		name     string
		input    json.RawMessage
		wantErr  bool
		isError  bool
		contains []string
	}{
		{
			name:     "list with explicit path",
			input:    mustJSON(t, listDirInput{Path: dir}),
			contains: []string{"file1.txt", "subdir/"},
		},
		{
			name:     "list with empty input object",
			input:    []byte("{}"),
			contains: []string{"file1.txt", "subdir/"},
		},
		{
			name:     "list with empty input",
			input:    []byte(""),
			contains: []string{"file1.txt", "subdir/"},
		},
		{
			name:    "list nonexistent directory",
			input:   mustJSON(t, listDirInput{Path: filepath.Join(dir, "nope")}),
			isError: true,
		},
		{
			name:    "relative path rejected",
			input:   mustJSON(t, listDirInput{Path: "relative/path"}),
			isError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := tool.Execute(context.Background(), tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if result.IsError != tt.isError {
				t.Errorf("Execute() IsError = %v, want %v, output: %s", result.IsError, tt.isError, result.Output)
			}
			for _, s := range tt.contains {
				if !strings.Contains(result.Output, s) {
					t.Errorf("Execute() output missing %q, got: %s", s, result.Output)
				}
			}
		})
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling JSON: %v", err)
	}
	return b
}
