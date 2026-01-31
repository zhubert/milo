package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/zhubert/milo/internal/todo"
)

// TodoTool manages the task list for tracking progress.
type TodoTool struct {
	Store *todo.Store
}

// IsParallelSafe returns false since todo updates should be sequential.
func (t *TodoTool) IsParallelSafe() bool { return false }

type todoInput struct {
	Todos []todoItem `json:"todos"`
}

type todoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

func (t *TodoTool) Name() string { return "todo" }

func (t *TodoTool) Description() string {
	return `Manage a task list to track progress on multi-step work.

Use this tool to:
- Break down complex tasks into smaller steps
- Track progress by marking items as pending, in_progress, or completed
- Give the user visibility into what you're working on

When to use:
- Complex multi-step tasks (3+ steps)
- User provides multiple tasks to do
- Non-trivial tasks requiring planning

When NOT to use:
- Single, trivial tasks
- Purely informational requests
- Tasks completable in 1-2 quick steps

Task states:
- pending: Not yet started
- in_progress: Currently working on (only ONE at a time)
- completed: Finished

Always provide both content (imperative: "Run tests") and activeForm (continuous: "Running tests").
Mark tasks completed IMMEDIATELY after finishing - don't batch completions.`
}

func (t *TodoTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"todos": map[string]any{
				"type":        "array",
				"description": "The complete updated todo list",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content": map[string]any{
							"type":        "string",
							"description": "Task description in imperative form (e.g., 'Run tests')",
						},
						"status": map[string]any{
							"type":        "string",
							"enum":        []string{"pending", "in_progress", "completed"},
							"description": "Current task status",
						},
						"activeForm": map[string]any{
							"type":        "string",
							"description": "Task description in present continuous form (e.g., 'Running tests')",
						},
					},
					"required": []string{"content", "status", "activeForm"},
				},
			},
		},
		Required: []string{"todos"},
	}
}

func (t *TodoTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in todoInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing todo input: %w", err)
	}

	// Convert input items to todo.Todo
	todos := make([]todo.Todo, len(in.Todos))
	for i, item := range in.Todos {
		status := todo.Status(item.Status)
		switch status {
		case todo.StatusPending, todo.StatusInProgress, todo.StatusCompleted:
			// valid
		default:
			return Result{
				Output:  fmt.Sprintf("invalid status %q for todo %d", item.Status, i),
				IsError: true,
			}, nil
		}

		todos[i] = todo.Todo{
			Content:    item.Content,
			ActiveForm: item.ActiveForm,
			Status:     status,
		}
	}

	t.Store.Set(todos)

	// Return a summary
	pending, inProgress, completed := t.Store.Stats()
	return Result{
		Output: fmt.Sprintf("Todo list updated: %d pending, %d in progress, %d completed",
			pending, inProgress, completed),
	}, nil
}
