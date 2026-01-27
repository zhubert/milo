package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteToolExecute(t *testing.T) {
	t.Parallel()

	writeTool := &WriteTool{}

	t.Run("write new file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fp := filepath.Join(dir, "new.txt")

		input, err := json.Marshal(writeInput{FilePath: fp, Content: "hello world"})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := writeTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}

		got, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading written file: %v", err)
		}
		if string(got) != "hello world" {
			t.Errorf("file content: got %q, want %q", string(got), "hello world")
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fp := filepath.Join(dir, "exist.txt")

		if err := os.WriteFile(fp, []byte("old"), 0644); err != nil {
			t.Fatalf("creating file: %v", err)
		}

		input, err := json.Marshal(writeInput{FilePath: fp, Content: "new"})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := writeTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}

		got, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading written file: %v", err)
		}
		if string(got) != "new" {
			t.Errorf("file content: got %q, want %q", string(got), "new")
		}
	})

	t.Run("create nested directories", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fp := filepath.Join(dir, "a", "b", "c", "deep.txt")

		input, err := json.Marshal(writeInput{FilePath: fp, Content: "deep"})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := writeTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}

		got, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading written file: %v", err)
		}
		if string(got) != "deep" {
			t.Errorf("file content: got %q, want %q", string(got), "deep")
		}
	})

	t.Run("relative path rejected", func(t *testing.T) {
		t.Parallel()

		input, err := json.Marshal(writeInput{FilePath: "relative/path.txt", Content: "test"})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := writeTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError for relative path")
		}
	})
}
