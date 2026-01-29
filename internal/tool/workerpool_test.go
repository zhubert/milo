package tool

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// mockTool is a test tool that can be configured for various behaviors.
type mockTool struct {
	name     string
	delay    time.Duration
	result   Result
	execFunc func(ctx context.Context, input json.RawMessage) (Result, error)
}

func (t *mockTool) Name() string { return t.name }

func (t *mockTool) Description() string { return "mock tool for testing" }

func (t *mockTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{},
	}
}

func (t *mockTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.execFunc != nil {
		return t.execFunc(ctx, input)
	}
	if t.delay > 0 {
		select {
		case <-time.After(t.delay):
		case <-ctx.Done():
			return Result{Output: "cancelled", IsError: true}, ctx.Err()
		}
	}
	return t.result, nil
}

func TestWorkerPool_ExecuteBatch_Empty(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(4)
	results := pool.ExecuteBatch(context.Background(), nil, nil)

	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestWorkerPool_ExecuteBatch_SingleTask(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(4)
	tool := &mockTool{
		name:   "test",
		result: Result{Output: "success"},
	}

	tasks := []ToolTask{
		{
			Call: ToolCall{ID: "1", Name: "test", Input: json.RawMessage(`{}`)},
			Tool: tool,
			Ctx:  context.Background(),
		},
	}

	results := pool.ExecuteBatch(context.Background(), tasks, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Result.Output != "success" {
		t.Errorf("expected 'success', got %q", results[0].Result.Output)
	}
}

func TestWorkerPool_ExecuteBatch_Parallel(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(4)

	// Track concurrent executions.
	var maxConcurrent int32
	var current int32

	tool := &mockTool{
		name: "test",
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
	}

	tasks := make([]ToolTask, 8)
	for i := range tasks {
		tasks[i] = ToolTask{
			Call: ToolCall{ID: string(rune('1' + i)), Name: "test", Input: json.RawMessage(`{}`)},
			Tool: tool,
			Ctx:  context.Background(),
		}
	}

	results := pool.ExecuteBatch(context.Background(), tasks, nil)

	if len(results) != 8 {
		t.Fatalf("expected 8 results, got %d", len(results))
	}

	// With 4 workers and 8 tasks, we should see up to 4 concurrent executions.
	if maxConcurrent < 2 {
		t.Errorf("expected concurrent execution, max concurrent was %d", maxConcurrent)
	}
	if maxConcurrent > 4 {
		t.Errorf("exceeded worker limit, max concurrent was %d", maxConcurrent)
	}
}

func TestWorkerPool_ExecuteBatch_PreservesOrder(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(4)

	// Create tools with different delays to encourage out-of-order completion.
	delays := []time.Duration{50, 10, 30, 20}

	tasks := make([]ToolTask, len(delays))
	for i, delay := range delays {
		tool := &mockTool{
			name:   "test",
			delay:  delay * time.Millisecond,
			result: Result{Output: string(rune('A' + i))},
		}
		tasks[i] = ToolTask{
			Call: ToolCall{ID: string(rune('1' + i)), Name: "test", Input: json.RawMessage(`{}`)},
			Tool: tool,
			Ctx:  context.Background(),
		}
	}

	results := pool.ExecuteBatch(context.Background(), tasks, nil)

	// Results should be in original order despite different completion times.
	for i := range results {
		expected := string(rune('A' + i))
		if results[i].Result.Output != expected {
			t.Errorf("result[%d]: expected %q, got %q", i, expected, results[i].Result.Output)
		}
	}
}

func TestWorkerPool_ExecuteBatch_Progress(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(2)
	tool := &mockTool{
		name:   "test",
		delay:  10 * time.Millisecond,
		result: Result{Output: "done"},
	}

	tasks := make([]ToolTask, 4)
	for i := range tasks {
		tasks[i] = ToolTask{
			Call: ToolCall{ID: string(rune('1' + i)), Name: "test", Input: json.RawMessage(`{}`)},
			Tool: tool,
			Ctx:  context.Background(),
		}
	}

	progressCh := make(chan ProgressUpdate, 20)
	pool.ExecuteBatch(context.Background(), tasks, progressCh)
	close(progressCh)

	var updates []ProgressUpdate
	for update := range progressCh {
		updates = append(updates, update)
	}

	if len(updates) == 0 {
		t.Error("expected progress updates, got none")
	}

	// Check that we eventually see completion.
	var sawCompletion bool
	for _, u := range updates {
		if u.CompletedTasks == 4 {
			sawCompletion = true
			break
		}
	}
	if !sawCompletion {
		t.Error("never saw all tasks complete")
	}
}

func TestWorkerPool_ExecuteBatch_Cancellation(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(2)
	tool := &mockTool{
		name:  "test",
		delay: 1 * time.Second,
	}

	tasks := make([]ToolTask, 4)
	ctx, cancel := context.WithCancel(context.Background())

	for i := range tasks {
		tasks[i] = ToolTask{
			Call: ToolCall{ID: string(rune('1' + i)), Name: "test", Input: json.RawMessage(`{}`)},
			Tool: tool,
			Ctx:  ctx,
		}
	}

	// Cancel quickly.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	results := pool.ExecuteBatch(ctx, tasks, nil)
	elapsed := time.Since(start)

	// Should complete much faster than 4 seconds (serial execution).
	if elapsed > 500*time.Millisecond {
		t.Errorf("cancellation took too long: %v", elapsed)
	}

	// All results should be present.
	if len(results) != 4 {
		t.Errorf("expected 4 results, got %d", len(results))
	}
}
