package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEditToolExecute(t *testing.T) {
	t.Parallel()

	editTool := &EditTool{}

	t.Run("successful replace", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fp := filepath.Join(dir, "file.txt")
		if err := os.WriteFile(fp, []byte("hello world"), 0644); err != nil {
			t.Fatalf("creating file: %v", err)
		}

		input, err := json.Marshal(editInput{
			FilePath:  fp,
			OldString: "hello",
			NewString: "goodbye",
		})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := editTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}

		got, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(got) != "goodbye world" {
			t.Errorf("file content: got %q, want %q", string(got), "goodbye world")
		}
	})

	t.Run("old_string not found", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fp := filepath.Join(dir, "file.txt")
		if err := os.WriteFile(fp, []byte("hello world"), 0644); err != nil {
			t.Fatalf("creating file: %v", err)
		}

		input, err := json.Marshal(editInput{
			FilePath:  fp,
			OldString: "nonexistent",
			NewString: "replacement",
		})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := editTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError when old_string not found")
		}
	})

	t.Run("old_string not unique without replace_all", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fp := filepath.Join(dir, "file.txt")
		if err := os.WriteFile(fp, []byte("aaa bbb aaa"), 0644); err != nil {
			t.Fatalf("creating file: %v", err)
		}

		input, err := json.Marshal(editInput{
			FilePath:  fp,
			OldString: "aaa",
			NewString: "ccc",
		})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := editTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError when old_string is not unique")
		}
	})

	t.Run("old_string not unique with replace_all", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fp := filepath.Join(dir, "file.txt")
		if err := os.WriteFile(fp, []byte("aaa bbb aaa"), 0644); err != nil {
			t.Fatalf("creating file: %v", err)
		}

		input, err := json.Marshal(editInput{
			FilePath:   fp,
			OldString:  "aaa",
			NewString:  "ccc",
			ReplaceAll: true,
		})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := editTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}

		got, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(got) != "ccc bbb ccc" {
			t.Errorf("file content: got %q, want %q", string(got), "ccc bbb ccc")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fp := filepath.Join(dir, "nope.txt")

		input, err := json.Marshal(editInput{
			FilePath:  fp,
			OldString: "x",
			NewString: "y",
		})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := editTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError for nonexistent file")
		}
	})
}
