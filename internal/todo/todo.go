// Package todo provides task list management for the agent.
package todo

import "sync"

// Status represents the state of a todo item.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
)

// Todo represents a single task item.
type Todo struct {
	Content    string `json:"content"`    // Imperative form: "Run tests"
	ActiveForm string `json:"activeForm"` // Present continuous: "Running tests"
	Status     Status `json:"status"`
}

// Store manages the todo list state.
type Store struct {
	mu    sync.RWMutex
	todos []Todo
}

// NewStore creates a new empty todo store.
func NewStore() *Store {
	return &Store{
		todos: []Todo{},
	}
}

// Set replaces the entire todo list.
func (s *Store) Set(todos []Todo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.todos = make([]Todo, len(todos))
	copy(s.todos, todos)
}

// List returns a copy of the current todo list.
func (s *Store) List() []Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Todo, len(s.todos))
	copy(result, s.todos)
	return result
}

// InProgress returns the todo currently marked as in_progress, if any.
func (s *Store) InProgress() *Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.todos {
		if t.Status == StatusInProgress {
			return &t
		}
	}
	return nil
}

// HasItems returns true if there are any todos.
func (s *Store) HasItems() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.todos) > 0
}

// Stats returns counts of pending, in-progress, and completed todos.
func (s *Store) Stats() (pending, inProgress, completed int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.todos {
		switch t.Status {
		case StatusPending:
			pending++
		case StatusInProgress:
			inProgress++
		case StatusCompleted:
			completed++
		}
	}
	return
}
