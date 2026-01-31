package tool

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewFileHistory(t *testing.T) {
	t.Parallel()

	h := NewFileHistory(50)

	if h == nil {
		t.Fatal("NewFileHistory returned nil")
	}
	if h.maxSize != 50 {
		t.Errorf("maxSize: got %d, want 50", h.maxSize)
	}
	if h.changes == nil {
		t.Error("changes slice should be initialized")
	}
	if len(h.changes) != 0 {
		t.Errorf("expected empty changes, got %d", len(h.changes))
	}
}

func TestRecordChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	t.Run("existing file", func(t *testing.T) {
		t.Parallel()

		testFile := filepath.Join(dir, "existing.txt")
		content := "original content"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		h := NewFileHistory(10)
		err := h.RecordChange(testFile, "edit", "test edit")
		if err != nil {
			t.Fatalf("RecordChange failed: %v", err)
		}

		if h.Len() != 1 {
			t.Fatalf("expected 1 change, got %d", h.Len())
		}

		changes := h.GetChanges(testFile)
		if len(changes) != 1 {
			t.Fatalf("expected 1 change for file, got %d", len(changes))
		}

		change := changes[0]
		if !change.Existed {
			t.Error("Existed should be true for existing file")
		}
		if string(change.OldContent) != content {
			t.Errorf("OldContent: got %q, want %q", string(change.OldContent), content)
		}
		if change.ToolName != "edit" {
			t.Errorf("ToolName: got %q, want %q", change.ToolName, "edit")
		}
		if change.Description != "test edit" {
			t.Errorf("Description: got %q, want %q", change.Description, "test edit")
		}
	})

	t.Run("non-existing file", func(t *testing.T) {
		t.Parallel()

		nonExistent := filepath.Join(dir, "does_not_exist.txt")

		h := NewFileHistory(10)
		err := h.RecordChange(nonExistent, "write", "new file")
		if err != nil {
			t.Fatalf("RecordChange failed: %v", err)
		}

		changes := h.GetChanges(nonExistent)
		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}

		change := changes[0]
		if change.Existed {
			t.Error("Existed should be false for non-existing file")
		}
		if change.OldContent != nil {
			t.Errorf("OldContent should be nil, got %v", change.OldContent)
		}
	})

	t.Run("unreadable file returns error", func(t *testing.T) {
		t.Parallel()

		// Create a directory and try to read it as a file
		unreadableDir := filepath.Join(dir, "unreadable_dir")
		if err := os.Mkdir(unreadableDir, 0755); err != nil {
			t.Fatalf("creating directory: %v", err)
		}

		h := NewFileHistory(10)
		err := h.RecordChange(unreadableDir, "edit", "test")

		// Trying to read a directory should return an error
		if err == nil {
			t.Error("expected error when recording change for directory")
		}
	})
}

func TestGetChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	h := NewFileHistory(10)

	// Record multiple changes
	_ = h.RecordChange(file1, "edit", "first edit")
	_ = h.RecordChange(file2, "edit", "second edit")
	_ = h.RecordChange(file1, "edit", "third edit")

	// Get changes for file1 only
	changes := h.GetChanges(file1)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes for file1, got %d", len(changes))
	}

	// Verify most recent first order
	if changes[0].Description != "third edit" {
		t.Errorf("expected most recent change first, got %q", changes[0].Description)
	}
	if changes[1].Description != "first edit" {
		t.Errorf("expected oldest change last, got %q", changes[1].Description)
	}

	// Verify file2 has only one change
	changes2 := h.GetChanges(file2)
	if len(changes2) != 1 {
		t.Fatalf("expected 1 change for file2, got %d", len(changes2))
	}

	// Non-existent file should return empty slice
	changes3 := h.GetChanges(filepath.Join(dir, "nonexistent.txt"))
	if len(changes3) != 0 {
		t.Errorf("expected 0 changes for nonexistent file, got %d", len(changes3))
	}
}

func TestGetAllChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")

	if err := os.WriteFile(file1, []byte("a"), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("b"), 0644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	h := NewFileHistory(10)

	_ = h.RecordChange(file1, "edit", "first")
	_ = h.RecordChange(file2, "edit", "second")
	_ = h.RecordChange(file1, "edit", "third")

	changes := h.GetAllChanges()
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}

	// Verify reverse chronological order
	if changes[0].Description != "third" {
		t.Errorf("expected most recent first, got %q", changes[0].Description)
	}
	if changes[2].Description != "first" {
		t.Errorf("expected oldest last, got %q", changes[2].Description)
	}
}

func TestPopChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")

	if err := os.WriteFile(file1, []byte("a"), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("b"), 0644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	h := NewFileHistory(10)

	_ = h.RecordChange(file1, "edit", "first")
	_ = h.RecordChange(file2, "edit", "second")
	_ = h.RecordChange(file1, "edit", "third")

	// Pop most recent change for file1
	change := h.PopChange(file1)
	if change == nil {
		t.Fatal("expected non-nil change")
	}
	if change.Description != "third" {
		t.Errorf("expected 'third', got %q", change.Description)
	}

	// Verify it was removed
	if h.Len() != 2 {
		t.Errorf("expected 2 changes remaining, got %d", h.Len())
	}

	// Pop again should get the first change for file1
	change = h.PopChange(file1)
	if change == nil {
		t.Fatal("expected non-nil change")
	}
	if change.Description != "first" {
		t.Errorf("expected 'first', got %q", change.Description)
	}

	// Pop again should return nil
	change = h.PopChange(file1)
	if change != nil {
		t.Errorf("expected nil, got %+v", change)
	}
}

func TestPopChange_EmptyHistory(t *testing.T) {
	t.Parallel()

	h := NewFileHistory(10)
	change := h.PopChange("/some/path.txt")

	if change != nil {
		t.Errorf("expected nil for empty history, got %+v", change)
	}
}

func TestPopMostRecent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")

	if err := os.WriteFile(file1, []byte("a"), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("b"), 0644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	h := NewFileHistory(10)

	_ = h.RecordChange(file1, "edit", "first")
	_ = h.RecordChange(file2, "edit", "second")
	_ = h.RecordChange(file1, "edit", "third")

	// Pop should return most recent across all files
	change := h.PopMostRecent()
	if change == nil {
		t.Fatal("expected non-nil change")
	}
	if change.Description != "third" {
		t.Errorf("expected 'third', got %q", change.Description)
	}
	if change.FilePath != file1 {
		t.Errorf("expected file1 path, got %q", change.FilePath)
	}

	// Next pop should return second
	change = h.PopMostRecent()
	if change == nil {
		t.Fatal("expected non-nil change")
	}
	if change.Description != "second" {
		t.Errorf("expected 'second', got %q", change.Description)
	}

	// And last
	change = h.PopMostRecent()
	if change.Description != "first" {
		t.Errorf("expected 'first', got %q", change.Description)
	}

	// Now should be empty
	change = h.PopMostRecent()
	if change != nil {
		t.Errorf("expected nil, got %+v", change)
	}
}

func TestPopMostRecent_Empty(t *testing.T) {
	t.Parallel()

	h := NewFileHistory(10)
	change := h.PopMostRecent()

	if change != nil {
		t.Errorf("expected nil for empty history, got %+v", change)
	}
}

func TestClear(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	h := NewFileHistory(10)
	_ = h.RecordChange(testFile, "edit", "first")
	_ = h.RecordChange(testFile, "edit", "second")
	_ = h.RecordChange(testFile, "edit", "third")

	if h.Len() != 3 {
		t.Fatalf("expected 3 changes before clear, got %d", h.Len())
	}

	h.Clear()

	if h.Len() != 0 {
		t.Errorf("expected 0 changes after clear, got %d", h.Len())
	}

	// Verify we can still add after clear
	_ = h.RecordChange(testFile, "edit", "fourth")
	if h.Len() != 1 {
		t.Errorf("expected 1 change after re-adding, got %d", h.Len())
	}
}

func TestLen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	h := NewFileHistory(10)

	if h.Len() != 0 {
		t.Errorf("expected 0, got %d", h.Len())
	}

	_ = h.RecordChange(testFile, "edit", "first")
	if h.Len() != 1 {
		t.Errorf("expected 1, got %d", h.Len())
	}

	_ = h.RecordChange(testFile, "edit", "second")
	if h.Len() != 2 {
		t.Errorf("expected 2, got %d", h.Len())
	}

	_ = h.PopMostRecent()
	if h.Len() != 1 {
		t.Errorf("expected 1 after pop, got %d", h.Len())
	}
}

func TestMaxSizeTrimming(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// Create history with max size of 3
	h := NewFileHistory(3)

	// Add 5 changes
	for i := 1; i <= 5; i++ {
		// Update file content to have different snapshots
		if err := os.WriteFile(testFile, []byte("content"+string(rune('0'+i))), 0644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}
		_ = h.RecordChange(testFile, "edit", "change"+string(rune('0'+i)))
	}

	// Should only have 3 changes (the most recent)
	if h.Len() != 3 {
		t.Errorf("expected 3 changes (max size), got %d", h.Len())
	}

	// Verify oldest changes were trimmed
	changes := h.GetAllChanges()
	if changes[0].Description != "change5" {
		t.Errorf("expected most recent to be 'change5', got %q", changes[0].Description)
	}
	if changes[2].Description != "change3" {
		t.Errorf("expected oldest remaining to be 'change3', got %q", changes[2].Description)
	}
}

func TestFileHistoryConcurrency(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	h := NewFileHistory(100)
	var wg sync.WaitGroup

	// Run concurrent operations
	for i := 0; i < 50; i++ {
		wg.Add(5)

		go func() {
			defer wg.Done()
			_ = h.RecordChange(testFile, "edit", "concurrent")
		}()

		go func() {
			defer wg.Done()
			_ = h.GetChanges(testFile)
		}()

		go func() {
			defer wg.Done()
			_ = h.GetAllChanges()
		}()

		go func() {
			defer wg.Done()
			_ = h.Len()
		}()

		go func() {
			defer wg.Done()
			_ = h.PopMostRecent()
		}()
	}

	wg.Wait()
	// Test passes if no race conditions or deadlocks occur
}
