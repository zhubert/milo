package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveToolExecute(t *testing.T) {
	t.Parallel()

	moveTool := &MoveTool{}

	t.Run("move file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		source := filepath.Join(dir, "source.txt")
		dest := filepath.Join(dir, "dest.txt")

		// Create source file
		if err := os.WriteFile(source, []byte("test content"), 0644); err != nil {
			t.Fatalf("creating source file: %v", err)
		}

		input, err := json.Marshal(moveInput{Source: source, Destination: dest})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := moveTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}

		// Check source no longer exists
		if _, err := os.Stat(source); !os.IsNotExist(err) {
			t.Error("source file still exists after move")
		}

		// Check destination exists with correct content
		got, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("reading destination file: %v", err)
		}
		if string(got) != "test content" {
			t.Errorf("file content: got %q, want %q", string(got), "test content")
		}
	})

	t.Run("move directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		sourceDir := filepath.Join(dir, "source_dir")
		destDir := filepath.Join(dir, "dest_dir")
		testFile := filepath.Join(sourceDir, "test.txt")

		// Create source directory with a file
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("creating source directory: %v", err)
		}
		if err := os.WriteFile(testFile, []byte("directory content"), 0644); err != nil {
			t.Fatalf("creating test file in directory: %v", err)
		}

		input, err := json.Marshal(moveInput{Source: sourceDir, Destination: destDir})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := moveTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}

		// Check source directory no longer exists
		if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
			t.Error("source directory still exists after move")
		}

		// Check destination directory exists with correct content
		destFile := filepath.Join(destDir, "test.txt")
		got, err := os.ReadFile(destFile)
		if err != nil {
			t.Fatalf("reading file in destination directory: %v", err)
		}
		if string(got) != "directory content" {
			t.Errorf("file content: got %q, want %q", string(got), "directory content")
		}
	})

	t.Run("rename file in same directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		source := filepath.Join(dir, "old_name.txt")
		dest := filepath.Join(dir, "new_name.txt")

		// Create source file
		if err := os.WriteFile(source, []byte("rename test"), 0644); err != nil {
			t.Fatalf("creating source file: %v", err)
		}

		input, err := json.Marshal(moveInput{Source: source, Destination: dest})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := moveTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}

		// Check source no longer exists
		if _, err := os.Stat(source); !os.IsNotExist(err) {
			t.Error("source file still exists after rename")
		}

		// Check destination exists with correct content
		got, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("reading renamed file: %v", err)
		}
		if string(got) != "rename test" {
			t.Errorf("file content: got %q, want %q", string(got), "rename test")
		}
	})

	t.Run("create destination directories", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		source := filepath.Join(dir, "source.txt")
		dest := filepath.Join(dir, "a", "b", "c", "dest.txt")

		// Create source file
		if err := os.WriteFile(source, []byte("deep move"), 0644); err != nil {
			t.Fatalf("creating source file: %v", err)
		}

		input, err := json.Marshal(moveInput{Source: source, Destination: dest})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := moveTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}

		// Check destination exists with correct content
		got, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("reading destination file: %v", err)
		}
		if string(got) != "deep move" {
			t.Errorf("file content: got %q, want %q", string(got), "deep move")
		}
	})

	t.Run("source does not exist", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		source := filepath.Join(dir, "nonexistent.txt")
		dest := filepath.Join(dir, "dest.txt")

		input, err := json.Marshal(moveInput{Source: source, Destination: dest})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := moveTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError for nonexistent source")
		}
	})

	t.Run("destination already exists", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		source := filepath.Join(dir, "source.txt")
		dest := filepath.Join(dir, "dest.txt")

		// Create both source and destination files
		if err := os.WriteFile(source, []byte("source content"), 0644); err != nil {
			t.Fatalf("creating source file: %v", err)
		}
		if err := os.WriteFile(dest, []byte("dest content"), 0644); err != nil {
			t.Fatalf("creating destination file: %v", err)
		}

		input, err := json.Marshal(moveInput{Source: source, Destination: dest})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := moveTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError for existing destination")
		}
	})

	t.Run("relative source path rejected", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dest := filepath.Join(dir, "dest.txt")

		input, err := json.Marshal(moveInput{Source: "relative/path.txt", Destination: dest})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := moveTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError for relative source path")
		}
	})

	t.Run("relative destination path rejected", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		source := filepath.Join(dir, "source.txt")

		input, err := json.Marshal(moveInput{Source: source, Destination: "relative/dest.txt"})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := moveTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError for relative destination path")
		}
	})
}