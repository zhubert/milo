package tool

import (
	"context"
	"encoding/json"
)

// ParallelSafeTool indicates a tool can safely run concurrently with other
// parallel-safe tools. Tools that only read data (e.g., read, glob, grep)
// should implement this interface.
type ParallelSafeTool interface {
	Tool
	IsParallelSafe() bool
}

// FileAccessor is implemented by tools that access files, enabling conflict
// detection for parallel execution. Write operations to the same file must
// be serialized.
type FileAccessor interface {
	// GetFilePath extracts the target file path from the tool input.
	// Returns empty string if the tool doesn't target a specific file.
	GetFilePath(input json.RawMessage) string

	// IsWriteOperation returns true if this tool modifies the file system.
	IsWriteOperation() bool
}

// ToolCall represents a single tool invocation request.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// TaskResult holds the outcome of a tool execution.
type TaskResult struct {
	ID     string
	Name   string
	Result Result
	Err    error
}

// ProgressUpdate reports progress during parallel tool execution.
type ProgressUpdate struct {
	TotalTasks     int
	CompletedTasks int
	InProgress     []string // Tool names currently executing
}

// ToolTask is an internal structure for the worker pool.
type ToolTask struct {
	Call     ToolCall
	Tool     Tool
	Ctx      context.Context
	ResultCh chan<- TaskResult
}
