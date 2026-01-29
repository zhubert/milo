package tool

import (
	"encoding/json"
	"testing"
)

func TestExtractFilePath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid file_path",
			input:    `{"file_path": "/path/to/file.txt"}`,
			expected: "/path/to/file.txt",
		},
		{
			name:     "empty file_path",
			input:    `{"file_path": ""}`,
			expected: "",
		},
		{
			name:     "missing file_path",
			input:    `{"other": "value"}`,
			expected: "",
		},
		{
			name:     "invalid json",
			input:    `not json`,
			expected: "",
		},
		{
			name:     "empty object",
			input:    `{}`,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := ExtractFilePath(json.RawMessage(tc.input))
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestProgressUpdate(t *testing.T) {
	t.Parallel()

	update := ProgressUpdate{
		TotalTasks:     5,
		CompletedTasks: 2,
		InProgress:     []string{"read", "glob"},
	}

	if update.TotalTasks != 5 {
		t.Errorf("expected TotalTasks=5, got %d", update.TotalTasks)
	}
	if update.CompletedTasks != 2 {
		t.Errorf("expected CompletedTasks=2, got %d", update.CompletedTasks)
	}
	if len(update.InProgress) != 2 {
		t.Errorf("expected 2 in-progress tools, got %d", len(update.InProgress))
	}
}

func TestToolCall(t *testing.T) {
	t.Parallel()

	call := ToolCall{
		ID:    "tool-123",
		Name:  "read",
		Input: json.RawMessage(`{"file_path": "/test.txt"}`),
	}

	if call.ID != "tool-123" {
		t.Errorf("expected ID='tool-123', got %q", call.ID)
	}
	if call.Name != "read" {
		t.Errorf("expected Name='read', got %q", call.Name)
	}

	// Verify input can be unmarshaled.
	var data struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(call.Input, &data); err != nil {
		t.Errorf("failed to unmarshal input: %v", err)
	}
	if data.FilePath != "/test.txt" {
		t.Errorf("expected FilePath='/test.txt', got %q", data.FilePath)
	}
}

func TestTaskResult(t *testing.T) {
	t.Parallel()

	result := TaskResult{
		ID:   "task-1",
		Name: "read",
		Result: Result{
			Output:  "file contents",
			IsError: false,
		},
		Err: nil,
	}

	if result.ID != "task-1" {
		t.Errorf("expected ID='task-1', got %q", result.ID)
	}
	if result.Name != "read" {
		t.Errorf("expected Name='read', got %q", result.Name)
	}
	if result.Result.IsError {
		t.Error("expected non-error result")
	}
	if result.Err != nil {
		t.Errorf("expected nil error, got %v", result.Err)
	}
}
