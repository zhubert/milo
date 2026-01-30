package tool

import (
	"os"
	"sync"
	"time"
)

// FileChange represents a single file modification that can be undone.
type FileChange struct {
	FilePath    string
	OldContent  []byte // nil means file didn't exist before
	Existed     bool   // whether the file existed before the change
	Timestamp   time.Time
	ToolName    string // which tool made the change
	Description string // human-readable description of what changed
}

// FileHistory tracks file changes for undo operations.
// It maintains a stack of changes per file, allowing multiple undos.
type FileHistory struct {
	mu      sync.RWMutex
	changes []FileChange // stack of changes, most recent last
	maxSize int          // maximum number of changes to retain
}

// DefaultFileHistory is the global history instance used by write tools.
var DefaultFileHistory = NewFileHistory(100)

// NewFileHistory creates a new FileHistory with the given max size.
func NewFileHistory(maxSize int) *FileHistory {
	return &FileHistory{
		changes: make([]FileChange, 0),
		maxSize: maxSize,
	}
}

// RecordChange saves the current state of a file before it is modified.
// If the file doesn't exist, it records that fact so undo can delete it.
func (h *FileHistory) RecordChange(filePath, toolName, description string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	change := FileChange{
		FilePath:    filePath,
		Timestamp:   time.Now(),
		ToolName:    toolName,
		Description: description,
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			change.Existed = false
			change.OldContent = nil
		} else {
			return err
		}
	} else {
		change.Existed = true
		change.OldContent = data
	}

	h.changes = append(h.changes, change)

	// Trim if we exceed max size.
	if len(h.changes) > h.maxSize {
		h.changes = h.changes[len(h.changes)-h.maxSize:]
	}

	return nil
}

// GetChanges returns the change history for a specific file, most recent first.
func (h *FileHistory) GetChanges(filePath string) []FileChange {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []FileChange
	for i := len(h.changes) - 1; i >= 0; i-- {
		if h.changes[i].FilePath == filePath {
			result = append(result, h.changes[i])
		}
	}
	return result
}

// GetAllChanges returns all recorded changes, most recent first.
func (h *FileHistory) GetAllChanges() []FileChange {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]FileChange, len(h.changes))
	for i, j := 0, len(h.changes)-1; j >= 0; i, j = i+1, j-1 {
		result[i] = h.changes[j]
	}
	return result
}

// PopChange removes and returns the most recent change for a file.
// Returns nil if no changes exist for the file.
func (h *FileHistory) PopChange(filePath string) *FileChange {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find the most recent change for this file.
	for i := len(h.changes) - 1; i >= 0; i-- {
		if h.changes[i].FilePath == filePath {
			change := h.changes[i]
			h.changes = append(h.changes[:i], h.changes[i+1:]...)
			return &change
		}
	}
	return nil
}

// PopMostRecent removes and returns the most recent change across all files.
// Returns nil if no changes exist.
func (h *FileHistory) PopMostRecent() *FileChange {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.changes) == 0 {
		return nil
	}

	change := h.changes[len(h.changes)-1]
	h.changes = h.changes[:len(h.changes)-1]
	return &change
}

// Clear removes all recorded changes.
func (h *FileHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.changes = h.changes[:0]
}

// Len returns the number of recorded changes.
func (h *FileHistory) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.changes)
}
