package tool

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"
)

// parallelMockTool implements ParallelSafeTool.
type parallelMockTool struct {
	mockTool
	parallelSafe bool
}

func (t *parallelMockTool) IsParallelSafe() bool { return t.parallelSafe }

// fileAccessMockTool implements FileAccessor.
type fileAccessMockTool struct {
	mockTool
	filePath string
	isWrite  bool
}

func (t *fileAccessMockTool) GetFilePath(input json.RawMessage) string { return t.filePath }
func (t *fileAccessMockTool) IsWriteOperation() bool                   { return t.isWrite }

func TestToolExecutor_ExecuteTools_Empty(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	executor := NewToolExecutor(registry, 4)

	results, err := executor.ExecuteTools(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestToolExecutor_ExecuteTools_SingleTool(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	tool := &mockTool{
		name:   "test",
		result: Result{Output: "success"},
	}
	if err := registry.Register(tool); err != nil {
		t.Fatal(err)
	}

	executor := NewToolExecutor(registry, 4)
	calls := []ToolCall{
		{ID: "1", Name: "test", Input: json.RawMessage(`{}`)},
	}

	results, err := executor.ExecuteTools(context.Background(), calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Result.Output != "success" {
		t.Errorf("expected 'success', got %q", results[0].Result.Output)
	}
}

func TestToolExecutor_ExecuteTools_UnknownTool(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	executor := NewToolExecutor(registry, 4)
	calls := []ToolCall{
		{ID: "1", Name: "nonexistent", Input: json.RawMessage(`{}`)},
	}

	results, err := executor.ExecuteTools(context.Background(), calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Result.IsError {
		t.Error("expected error result for unknown tool")
	}
}

func TestToolExecutor_ExecuteTools_ParallelSafeTools(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	var maxConcurrent int32
	var current int32

	for i := 0; i < 4; i++ {
		tool := &parallelMockTool{
			mockTool: mockTool{
				name: "read" + string(rune('1'+i)),
				execFunc: func(ctx context.Context, input json.RawMessage) (Result, error) {
					c := atomic.AddInt32(&current, 1)
					for {
						old := atomic.LoadInt32(&maxConcurrent)
						if c <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, c) {
							break
						}
					}
					time.Sleep(50 * time.Millisecond)
					atomic.AddInt32(&current, -1)
					return Result{Output: "done"}, nil
				},
			},
			parallelSafe: true,
		}
		if err := registry.Register(tool); err != nil {
			t.Fatal(err)
		}
	}

	executor := NewToolExecutor(registry, 4)
	calls := []ToolCall{
		{ID: "1", Name: "read1", Input: json.RawMessage(`{}`)},
		{ID: "2", Name: "read2", Input: json.RawMessage(`{}`)},
		{ID: "3", Name: "read3", Input: json.RawMessage(`{}`)},
		{ID: "4", Name: "read4", Input: json.RawMessage(`{}`)},
	}

	results, err := executor.ExecuteTools(context.Background(), calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// All parallel-safe tools should run concurrently.
	if maxConcurrent < 2 {
		t.Errorf("expected parallel execution, max concurrent was %d", maxConcurrent)
	}
}

func TestToolExecutor_ExecuteTools_ConflictingWrites(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	var execOrder []string

	for i := 0; i < 3; i++ {
		idx := i
		tool := &fileAccessMockTool{
			mockTool: mockTool{
				name: "write" + string(rune('1'+i)),
				execFunc: func(ctx context.Context, input json.RawMessage) (Result, error) {
					execOrder = append(execOrder, "write"+string(rune('1'+idx)))
					time.Sleep(10 * time.Millisecond)
					return Result{Output: "done"}, nil
				},
			},
			filePath: "/same/file.txt",
			isWrite:  true,
		}
		if err := registry.Register(tool); err != nil {
			t.Fatal(err)
		}
	}

	executor := NewToolExecutor(registry, 4)
	calls := []ToolCall{
		{ID: "1", Name: "write1", Input: json.RawMessage(`{}`)},
		{ID: "2", Name: "write2", Input: json.RawMessage(`{}`)},
		{ID: "3", Name: "write3", Input: json.RawMessage(`{}`)},
	}

	results, err := executor.ExecuteTools(context.Background(), calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// All writes to same file should be serialized (though order within groups may vary).
	// We can't guarantee exact order, but we should have all 3 executions.
	if len(execOrder) != 3 {
		t.Errorf("expected 3 executions, got %d", len(execOrder))
	}
}

func TestToolExecutor_ExecuteTools_MixedOperations(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	// Register a parallel-safe read tool.
	readTool := &parallelMockTool{
		mockTool: mockTool{
			name:   "read",
			delay:  10 * time.Millisecond,
			result: Result{Output: "read done"},
		},
		parallelSafe: true,
	}
	if err := registry.Register(readTool); err != nil {
		t.Fatal(err)
	}

	// Register a write tool.
	writeTool := &fileAccessMockTool{
		mockTool: mockTool{
			name:   "write",
			delay:  10 * time.Millisecond,
			result: Result{Output: "write done"},
		},
		filePath: "/test/file.txt",
		isWrite:  true,
	}
	if err := registry.Register(writeTool); err != nil {
		t.Fatal(err)
	}

	executor := NewToolExecutor(registry, 4)
	calls := []ToolCall{
		{ID: "1", Name: "read", Input: json.RawMessage(`{}`)},
		{ID: "2", Name: "read", Input: json.RawMessage(`{}`)},
		{ID: "3", Name: "write", Input: json.RawMessage(`{}`)},
		{ID: "4", Name: "read", Input: json.RawMessage(`{}`)},
	}

	results, err := executor.ExecuteTools(context.Background(), calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Verify all results are correct.
	for i, r := range results {
		if r.Result.IsError {
			t.Errorf("result[%d] had error: %s", i, r.Result.Output)
		}
	}
}

func TestToolExecutor_CanParallelize(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	regularTool := &mockTool{name: "regular"}
	parallelTool := &parallelMockTool{
		mockTool:     mockTool{name: "parallel"},
		parallelSafe: true,
	}

	if err := registry.Register(regularTool); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(parallelTool); err != nil {
		t.Fatal(err)
	}

	executor := NewToolExecutor(registry, 4)

	if executor.CanParallelize(regularTool) {
		t.Error("regular tool should not be parallel-safe")
	}
	if !executor.CanParallelize(parallelTool) {
		t.Error("parallel tool should be parallel-safe")
	}
}

func TestToolExecutor_groupByConflicts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		setupRegistry  func(*Registry)
		calls          []ToolCall
		expectedGroups int
	}{
		{
			name: "all parallel-safe reads",
			setupRegistry: func(r *Registry) {
				for i := 0; i < 3; i++ {
					tool := &parallelMockTool{
						mockTool:     mockTool{name: "read" + string(rune('1'+i))},
						parallelSafe: true,
					}
					_ = r.Register(tool)
				}
			},
			calls: []ToolCall{
				{ID: "1", Name: "read1"},
				{ID: "2", Name: "read2"},
				{ID: "3", Name: "read3"},
			},
			expectedGroups: 1, // All can run in parallel.
		},
		{
			name: "conflicting writes same file",
			setupRegistry: func(r *Registry) {
				for i := 0; i < 3; i++ {
					tool := &fileAccessMockTool{
						mockTool: mockTool{name: "write" + string(rune('1'+i))},
						filePath: "/same/file.txt",
						isWrite:  true,
					}
					_ = r.Register(tool)
				}
			},
			calls: []ToolCall{
				{ID: "1", Name: "write1"},
				{ID: "2", Name: "write2"},
				{ID: "3", Name: "write3"},
			},
			expectedGroups: 3, // All must be serialized.
		},
		{
			name: "writes to different files",
			setupRegistry: func(r *Registry) {
				for i := 0; i < 3; i++ {
					tool := &fileAccessMockTool{
						mockTool: mockTool{name: "write" + string(rune('1'+i))},
						filePath: "/file" + string(rune('1'+i)) + ".txt",
						isWrite:  true,
					}
					_ = r.Register(tool)
				}
			},
			calls: []ToolCall{
				{ID: "1", Name: "write1"},
				{ID: "2", Name: "write2"},
				{ID: "3", Name: "write3"},
			},
			// Non-parallel-safe tools conflict with each other.
			expectedGroups: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			registry := NewRegistry()
			tc.setupRegistry(registry)
			executor := NewToolExecutor(registry, 4)

			groups := executor.groupByConflicts(tc.calls)
			if len(groups) != tc.expectedGroups {
				t.Errorf("expected %d groups, got %d: %v", tc.expectedGroups, len(groups), groups)
			}
		})
	}
}
