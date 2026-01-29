package tool

import (
	"context"
	"sync"
)

// WorkerPool manages a pool of workers for concurrent tool execution.
type WorkerPool struct {
	workers int
	wg      sync.WaitGroup
}

// NewWorkerPool creates a new worker pool with the specified number of workers.
func NewWorkerPool(workers int) *WorkerPool {
	if workers < 1 {
		workers = 1
	}
	return &WorkerPool{
		workers: workers,
	}
}

// ExecuteBatch runs multiple tool tasks concurrently, respecting the worker limit.
// Results are returned in the same order as the input tasks.
// The progressCh receives updates as tasks complete (can be nil to disable).
func (p *WorkerPool) ExecuteBatch(ctx context.Context, tasks []ToolTask, progressCh chan<- ProgressUpdate) []TaskResult {
	if len(tasks) == 0 {
		return nil
	}

	results := make([]TaskResult, len(tasks))
	taskCh := make(chan indexedTask, len(tasks))
	resultCh := make(chan indexedResult, len(tasks))

	// Track progress.
	var progressMu sync.Mutex
	completed := 0
	inProgress := make(map[int]string)

	// Start workers.
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for it := range taskCh {
				// Update in-progress tracking.
				progressMu.Lock()
				inProgress[it.index] = it.task.Call.Name
				if progressCh != nil {
					progressCh <- buildProgress(len(tasks), completed, inProgress)
				}
				progressMu.Unlock()

				// Execute the tool.
				result, err := it.task.Tool.Execute(it.task.Ctx, it.task.Call.Input)

				// Update completion tracking.
				progressMu.Lock()
				delete(inProgress, it.index)
				completed++
				if progressCh != nil {
					progressCh <- buildProgress(len(tasks), completed, inProgress)
				}
				progressMu.Unlock()

				resultCh <- indexedResult{
					index: it.index,
					result: TaskResult{
						ID:     it.task.Call.ID,
						Name:   it.task.Call.Name,
						Result: result,
						Err:    err,
					},
				}
			}
		}()
	}

	// Send tasks to workers.
	go func() {
		for i, task := range tasks {
			// Check context before sending.
			if ctx.Err() != nil {
				// Send cancelled result for remaining tasks.
				resultCh <- indexedResult{
					index: i,
					result: TaskResult{
						ID:   task.Call.ID,
						Name: task.Call.Name,
						Result: Result{
							Output:  "execution cancelled",
							IsError: true,
						},
					},
				}
				continue
			}
			taskCh <- indexedTask{index: i, task: task}
		}
		close(taskCh)
	}()

	// Collect results.
	for i := 0; i < len(tasks); i++ {
		ir := <-resultCh
		results[ir.index] = ir.result
	}

	// Wait for all workers to finish.
	p.wg.Wait()

	return results
}

// indexedTask pairs a task with its original index.
type indexedTask struct {
	index int
	task  ToolTask
}

// indexedResult pairs a result with its original index.
type indexedResult struct {
	index  int
	result TaskResult
}

// buildProgress creates a ProgressUpdate from the current state.
func buildProgress(total, completed int, inProgress map[int]string) ProgressUpdate {
	names := make([]string, 0, len(inProgress))
	for _, name := range inProgress {
		names = append(names, name)
	}
	return ProgressUpdate{
		TotalTasks:     total,
		CompletedTasks: completed,
		InProgress:     names,
	}
}
