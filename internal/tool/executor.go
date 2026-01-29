package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ToolExecutor orchestrates tool execution, running parallel-safe tools
// concurrently while serializing conflicting operations.
type ToolExecutor struct {
	registry *Registry
	pool     *WorkerPool
}

// NewToolExecutor creates a new executor with the specified number of workers.
func NewToolExecutor(registry *Registry, workers int) *ToolExecutor {
	return &ToolExecutor{
		registry: registry,
		pool:     NewWorkerPool(workers),
	}
}

// ExecuteTools runs the given tool calls, parallelizing where safe.
// Tools are grouped by conflict detection: parallel-safe read operations run
// concurrently, while write operations to the same file are serialized.
// Results are returned in the same order as the input calls.
func (e *ToolExecutor) ExecuteTools(ctx context.Context, calls []ToolCall, progressCh chan<- ProgressUpdate) ([]TaskResult, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	results := make([]TaskResult, len(calls))

	// Categorize tools into groups that can run in parallel.
	groups := e.groupByConflicts(calls)

	// Execute each group. Within a group, tools run in parallel.
	// Groups themselves run sequentially to respect dependencies.
	processedCount := 0
	for _, group := range groups {
		if ctx.Err() != nil {
			// Fill remaining results with cancellation.
			for _, idx := range group {
				results[idx] = TaskResult{
					ID:   calls[idx].ID,
					Name: calls[idx].Name,
					Result: Result{
						Output:  "execution cancelled",
						IsError: true,
					},
				}
			}
			continue
		}

		// Build tasks for this group.
		tasks := make([]ToolTask, len(group))
		for i, idx := range group {
			call := calls[idx]
			t := e.registry.Lookup(call.Name)
			if t == nil {
				// Unknown tool - will be handled as error result.
				results[idx] = TaskResult{
					ID:   call.ID,
					Name: call.Name,
					Result: Result{
						Output:  fmt.Sprintf("unknown tool: %s", call.Name),
						IsError: true,
					},
				}
				// Remove from group by marking as processed.
				tasks[i] = ToolTask{} // Empty task, will skip.
				continue
			}
			tasks[i] = ToolTask{
				Call: call,
				Tool: t,
				Ctx:  ctx,
			}
		}

		// Filter out empty tasks (unknown tools).
		validTasks := make([]ToolTask, 0, len(tasks))
		validIndices := make([]int, 0, len(tasks))
		for i, task := range tasks {
			if task.Tool != nil {
				validTasks = append(validTasks, task)
				validIndices = append(validIndices, group[i])
			}
		}

		if len(validTasks) == 0 {
			continue
		}

		// Create a progress channel adapter if needed.
		var groupProgressCh chan ProgressUpdate
		var wg sync.WaitGroup
		if progressCh != nil {
			groupProgressCh = make(chan ProgressUpdate, len(validTasks)*2)
			wg.Add(1)
			go func(processed int) {
				defer wg.Done()
				for update := range groupProgressCh {
					// Adjust counts to reflect overall progress.
					progressCh <- ProgressUpdate{
						TotalTasks:     len(calls),
						CompletedTasks: processed + update.CompletedTasks,
						InProgress:     update.InProgress,
					}
				}
			}(processedCount)
		}

		// Execute the group.
		groupResults := e.pool.ExecuteBatch(ctx, validTasks, groupProgressCh)

		if groupProgressCh != nil {
			close(groupProgressCh)
			wg.Wait() // Wait for forwarding goroutine to complete before continuing.
		}

		// Map results back to original indices.
		for i, result := range groupResults {
			results[validIndices[i]] = result
		}

		processedCount += len(validTasks)
	}

	return results, nil
}

// CanParallelize checks if a tool is safe for parallel execution.
func (e *ToolExecutor) CanParallelize(t Tool) bool {
	if pt, ok := t.(ParallelSafeTool); ok {
		return pt.IsParallelSafe()
	}
	return false
}

// callInfo tracks metadata about a tool call for conflict detection.
type callInfo struct {
	index      int
	isWrite    bool
	filePath   string
	isParallel bool
}

// groupByConflicts partitions tool calls into groups that can safely run in parallel.
// Returns a slice of groups, where each group contains indices into the calls slice.
// Groups are ordered such that conflicting operations are in different groups.
func (e *ToolExecutor) groupByConflicts(calls []ToolCall) [][]int {
	if len(calls) == 0 {
		return nil
	}

	infos := make([]callInfo, len(calls))
	for i, call := range calls {
		t := e.registry.Lookup(call.Name)
		info := callInfo{index: i}

		if t != nil {
			info.isParallel = e.CanParallelize(t)

			if fa, ok := t.(FileAccessor); ok {
				info.isWrite = fa.IsWriteOperation()
				info.filePath = fa.GetFilePath(call.Input)
			}
		}

		infos[i] = info
	}

	// Build conflict graph - which calls conflict with which.
	conflicts := make(map[int]map[int]bool)
	for i := range calls {
		conflicts[i] = make(map[int]bool)
	}

	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			if e.hasConflict(infos[i], infos[j]) {
				conflicts[i][j] = true
				conflicts[j][i] = true
			}
		}
	}

	// Greedy grouping: assign each call to the first group it doesn't conflict with.
	var groups [][]int
	assigned := make([]int, len(calls)) // group index for each call, -1 if unassigned
	for i := range assigned {
		assigned[i] = -1
	}

	for i := range calls {
		// Find first group this call doesn't conflict with.
		foundGroup := -1
		for g, group := range groups {
			conflictsWithGroup := false
			for _, idx := range group {
				if conflicts[i][idx] {
					conflictsWithGroup = true
					break
				}
			}
			if !conflictsWithGroup {
				foundGroup = g
				break
			}
		}

		if foundGroup >= 0 {
			groups[foundGroup] = append(groups[foundGroup], i)
			assigned[i] = foundGroup
		} else {
			// Create new group.
			groups = append(groups, []int{i})
			assigned[i] = len(groups) - 1
		}
	}

	return groups
}

// hasConflict determines if two tool calls conflict and cannot run in parallel.
func (e *ToolExecutor) hasConflict(a, b callInfo) bool {
	// If both are parallel-safe reads, no conflict.
	if a.isParallel && b.isParallel && !a.isWrite && !b.isWrite {
		return false
	}

	// If either is a write and they target the same file, conflict.
	if a.filePath != "" && b.filePath != "" && a.filePath == b.filePath {
		if a.isWrite || b.isWrite {
			return true
		}
	}

	// If neither is marked parallel-safe, be conservative and conflict.
	if !a.isParallel && !b.isParallel {
		return true
	}

	// If one is a write (even to different files), conflict with non-parallel tools.
	if (a.isWrite && !b.isParallel) || (b.isWrite && !a.isParallel) {
		return true
	}

	return false
}

// ExtractFilePath is a helper to extract file_path from common tool inputs.
func ExtractFilePath(input json.RawMessage) string {
	var data struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(input, &data); err != nil {
		return ""
	}
	return data.FilePath
}
