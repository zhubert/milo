package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUndoToolExecute(t *testing.T) {
	t.Parallel()

	t.Run("undo write to existing file", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(10)
		dir := t.TempDir()
		fp := filepath.Join(dir, "test.txt")

		// Create initial file.
		originalContent := "original content"
		if err := os.WriteFile(fp, []byte(originalContent), 0644); err != nil {
			t.Fatalf("creating test file: %v", err)
		}

		// Use WriteTool to modify it (this records history).
		writeTool := &WriteTool{History: history}
		writeInputBytes, err := json.Marshal(writeInput{FilePath: fp, Content: "new content"})
		if err != nil {
			t.Fatalf("marshaling write input: %v", err)
		}
		result, err := writeTool.Execute(context.Background(), writeInputBytes)
		if err != nil {
			t.Fatalf("executing write: %v", err)
		}
		if result.IsError {
			t.Fatalf("write failed: %s", result.Output)
		}

		// Verify file was modified.
		data, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading modified file: %v", err)
		}
		if string(data) != "new content" {
			t.Fatalf("expected 'new content', got %q", string(data))
		}

		// Undo the change.
		undoTool := &UndoTool{History: history}
		undoInputBytes, err := json.Marshal(undoInput{FilePath: fp})
		if err != nil {
			t.Fatalf("marshaling undo input: %v", err)
		}
		result, err = undoTool.Execute(context.Background(), undoInputBytes)
		if err != nil {
			t.Fatalf("executing undo: %v", err)
		}
		if result.IsError {
			t.Fatalf("undo failed: %s", result.Output)
		}

		// Verify file was restored.
		data, err = os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading restored file: %v", err)
		}
		if string(data) != originalContent {
			t.Fatalf("expected %q, got %q", originalContent, string(data))
		}
	})

	t.Run("undo write to new file removes it", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(10)
		dir := t.TempDir()
		fp := filepath.Join(dir, "newfile.txt")

		// Use WriteTool to create a new file.
		writeTool := &WriteTool{History: history}
		writeInputBytes, err := json.Marshal(writeInput{FilePath: fp, Content: "new file content"})
		if err != nil {
			t.Fatalf("marshaling write input: %v", err)
		}
		result, err := writeTool.Execute(context.Background(), writeInputBytes)
		if err != nil {
			t.Fatalf("executing write: %v", err)
		}
		if result.IsError {
			t.Fatalf("write failed: %s", result.Output)
		}

		// Verify file exists.
		if _, err := os.Stat(fp); err != nil {
			t.Fatalf("file should exist: %v", err)
		}

		// Undo the change.
		undoTool := &UndoTool{History: history}
		undoInputBytes, err := json.Marshal(undoInput{})
		if err != nil {
			t.Fatalf("marshaling undo input: %v", err)
		}
		result, err = undoTool.Execute(context.Background(), undoInputBytes)
		if err != nil {
			t.Fatalf("executing undo: %v", err)
		}
		if result.IsError {
			t.Fatalf("undo failed: %s", result.Output)
		}

		// Verify file was removed.
		if _, err := os.Stat(fp); !os.IsNotExist(err) {
			t.Fatal("file should have been removed")
		}
	})

	t.Run("undo edit restores original content", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(10)
		dir := t.TempDir()
		fp := filepath.Join(dir, "edit.txt")

		// Create initial file.
		originalContent := "hello world"
		if err := os.WriteFile(fp, []byte(originalContent), 0644); err != nil {
			t.Fatalf("creating test file: %v", err)
		}

		// Use EditTool to modify it.
		editTool := &EditTool{History: history}
		editInputBytes, err := json.Marshal(editInput{
			FilePath:  fp,
			OldString: "world",
			NewString: "universe",
		})
		if err != nil {
			t.Fatalf("marshaling edit input: %v", err)
		}
		result, err := editTool.Execute(context.Background(), editInputBytes)
		if err != nil {
			t.Fatalf("executing edit: %v", err)
		}
		if result.IsError {
			t.Fatalf("edit failed: %s", result.Output)
		}

		// Verify file was edited.
		data, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading edited file: %v", err)
		}
		if string(data) != "hello universe" {
			t.Fatalf("expected 'hello universe', got %q", string(data))
		}

		// Undo the change.
		undoTool := &UndoTool{History: history}
		undoInputBytes, err := json.Marshal(undoInput{})
		if err != nil {
			t.Fatalf("marshaling undo input: %v", err)
		}
		result, err = undoTool.Execute(context.Background(), undoInputBytes)
		if err != nil {
			t.Fatalf("executing undo: %v", err)
		}
		if result.IsError {
			t.Fatalf("undo failed: %s", result.Output)
		}

		// Verify file was restored.
		data, err = os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading restored file: %v", err)
		}
		if string(data) != originalContent {
			t.Fatalf("expected %q, got %q", originalContent, string(data))
		}
	})

	t.Run("undo with no history returns error", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(10)
		undoTool := &UndoTool{History: history}
		undoInputBytes, err := json.Marshal(undoInput{})
		if err != nil {
			t.Fatalf("marshaling undo input: %v", err)
		}
		result, err := undoTool.Execute(context.Background(), undoInputBytes)
		if err != nil {
			t.Fatalf("executing undo: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected error when no history exists")
		}
		if result.Output != "no changes to undo" {
			t.Fatalf("unexpected error message: %s", result.Output)
		}
	})

	t.Run("undo specific file with no history for that file", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(10)
		undoTool := &UndoTool{History: history}
		undoInputBytes, err := json.Marshal(undoInput{FilePath: "/nonexistent/file.txt"})
		if err != nil {
			t.Fatalf("marshaling undo input: %v", err)
		}
		result, err := undoTool.Execute(context.Background(), undoInputBytes)
		if err != nil {
			t.Fatalf("executing undo: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected error when no history exists for file")
		}
	})

	t.Run("relative path returns error", func(t *testing.T) {
		t.Parallel()

		undoTool := &UndoTool{}
		undoInputBytes, err := json.Marshal(undoInput{FilePath: "relative/path.txt"})
		if err != nil {
			t.Fatalf("marshaling undo input: %v", err)
		}
		result, err := undoTool.Execute(context.Background(), undoInputBytes)
		if err != nil {
			t.Fatalf("executing undo: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected error for relative path")
		}
	})

	t.Run("multiple undos in sequence", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(10)
		dir := t.TempDir()
		fp := filepath.Join(dir, "multi.txt")

		// Create initial file.
		if err := os.WriteFile(fp, []byte("v1"), 0644); err != nil {
			t.Fatalf("creating test file: %v", err)
		}

		writeTool := &WriteTool{History: history}

		// Make several writes.
		for _, content := range []string{"v2", "v3", "v4"} {
			input, err := json.Marshal(writeInput{FilePath: fp, Content: content})
			if err != nil {
				t.Fatalf("marshaling write input: %v", err)
			}
			result, err := writeTool.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("executing write: %v", err)
			}
			if result.IsError {
				t.Fatalf("write failed: %s", result.Output)
			}
		}

		// Verify current content.
		data, err := os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(data) != "v4" {
			t.Fatalf("expected 'v4', got %q", string(data))
		}

		undoTool := &UndoTool{History: history}

		// Undo back to v3.
		undoInputBytes, err := json.Marshal(undoInput{})
		if err != nil {
			t.Fatalf("marshaling undo input: %v", err)
		}
		result, err := undoTool.Execute(context.Background(), undoInputBytes)
		if err != nil {
			t.Fatalf("executing undo: %v", err)
		}
		if result.IsError {
			t.Fatalf("undo failed: %s", result.Output)
		}
		data, err = os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(data) != "v3" {
			t.Fatalf("expected 'v3', got %q", string(data))
		}

		// Undo back to v2.
		result, err = undoTool.Execute(context.Background(), undoInputBytes)
		if err != nil {
			t.Fatalf("executing undo: %v", err)
		}
		if result.IsError {
			t.Fatalf("undo failed: %s", result.Output)
		}
		data, err = os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(data) != "v2" {
			t.Fatalf("expected 'v2', got %q", string(data))
		}

		// Undo back to v1.
		result, err = undoTool.Execute(context.Background(), undoInputBytes)
		if err != nil {
			t.Fatalf("executing undo: %v", err)
		}
		if result.IsError {
			t.Fatalf("undo failed: %s", result.Output)
		}
		data, err = os.ReadFile(fp)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(data) != "v1" {
			t.Fatalf("expected 'v1', got %q", string(data))
		}
	})
}

func TestFileHistory(t *testing.T) {
	t.Parallel()

	t.Run("respects max size", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(3)
		dir := t.TempDir()

		for i := 0; i < 5; i++ {
			fp := filepath.Join(dir, "test.txt")
			if err := os.WriteFile(fp, []byte("content"), 0644); err != nil {
				t.Fatalf("writing file: %v", err)
			}
			if err := history.RecordChange(fp, "test", "test change"); err != nil {
				t.Fatalf("recording change: %v", err)
			}
		}

		if history.Len() != 3 {
			t.Fatalf("expected 3 changes, got %d", history.Len())
		}
	})

	t.Run("pop removes from history", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(10)
		dir := t.TempDir()
		fp := filepath.Join(dir, "test.txt")

		if err := os.WriteFile(fp, []byte("content"), 0644); err != nil {
			t.Fatalf("writing file: %v", err)
		}
		if err := history.RecordChange(fp, "test", "test change"); err != nil {
			t.Fatalf("recording change: %v", err)
		}

		if history.Len() != 1 {
			t.Fatalf("expected 1 change, got %d", history.Len())
		}

		change := history.PopMostRecent()
		if change == nil {
			t.Fatal("expected change, got nil")
		}

		if history.Len() != 0 {
			t.Fatalf("expected 0 changes after pop, got %d", history.Len())
		}
	})

	t.Run("clear removes all history", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(10)
		dir := t.TempDir()
		fp := filepath.Join(dir, "test.txt")

		if err := os.WriteFile(fp, []byte("content"), 0644); err != nil {
			t.Fatalf("writing file: %v", err)
		}

		for i := 0; i < 3; i++ {
			if err := history.RecordChange(fp, "test", "test change"); err != nil {
				t.Fatalf("recording change: %v", err)
			}
		}

		history.Clear()
		if history.Len() != 0 {
			t.Fatalf("expected 0 changes after clear, got %d", history.Len())
		}
	})

	t.Run("records non-existent file correctly", func(t *testing.T) {
		t.Parallel()

		history := NewFileHistory(10)
		fp := "/nonexistent/path/file.txt"

		if err := history.RecordChange(fp, "test", "test change"); err != nil {
			t.Fatalf("recording change: %v", err)
		}

		changes := history.GetChanges(fp)
		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}
		if changes[0].Existed {
			t.Fatal("expected Existed to be false")
		}
		if changes[0].OldContent != nil {
			t.Fatal("expected OldContent to be nil")
		}
	})
}
