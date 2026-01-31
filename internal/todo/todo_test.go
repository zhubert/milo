package todo

import (
	"sync"
	"testing"
)

func TestNewStore(t *testing.T) {
	t.Parallel()

	store := NewStore()

	if store == nil {
		t.Fatal("NewStore returned nil")
	}
	if store.todos == nil {
		t.Error("todos slice should be initialized")
	}
	if len(store.todos) != 0 {
		t.Errorf("expected empty store, got %d items", len(store.todos))
	}
}

func TestStoreSet(t *testing.T) {
	t.Parallel()

	store := NewStore()
	todos := []Todo{
		{Content: "Task 1", ActiveForm: "Doing task 1", Status: StatusPending},
		{Content: "Task 2", ActiveForm: "Doing task 2", Status: StatusInProgress},
	}

	store.Set(todos)

	// Verify items were stored
	list := store.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}

	// Verify copy semantics - modifying original shouldn't affect store
	todos[0].Content = "Modified"
	list = store.List()
	if list[0].Content == "Modified" {
		t.Error("Set should copy items, not store references")
	}
}

func TestStoreSet_EmptySlice(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.Set([]Todo{
		{Content: "Initial", Status: StatusPending},
	})

	// Setting empty slice should clear the store
	store.Set([]Todo{})

	if store.HasItems() {
		t.Error("expected store to be empty after setting empty slice")
	}
}

func TestStoreList(t *testing.T) {
	t.Parallel()

	store := NewStore()
	original := []Todo{
		{Content: "Task 1", Status: StatusPending},
		{Content: "Task 2", Status: StatusCompleted},
	}
	store.Set(original)

	// Get the list
	list := store.List()

	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}

	// Verify it's a copy - modifying returned slice shouldn't affect store
	list[0].Content = "Modified"
	newList := store.List()
	if newList[0].Content == "Modified" {
		t.Error("List should return a copy, not the original slice")
	}
}

func TestStoreList_Empty(t *testing.T) {
	t.Parallel()

	store := NewStore()
	list := store.List()

	if list == nil {
		t.Error("List should return non-nil slice even when empty")
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestInProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		todos    []Todo
		wantNil  bool
		wantTask string
	}{
		{
			name:    "empty store",
			todos:   []Todo{},
			wantNil: true,
		},
		{
			name: "no in_progress items",
			todos: []Todo{
				{Content: "Task 1", Status: StatusPending},
				{Content: "Task 2", Status: StatusCompleted},
			},
			wantNil: true,
		},
		{
			name: "one in_progress item",
			todos: []Todo{
				{Content: "Task 1", Status: StatusPending},
				{Content: "Task 2", Status: StatusInProgress},
				{Content: "Task 3", Status: StatusCompleted},
			},
			wantNil:  false,
			wantTask: "Task 2",
		},
		{
			name: "first in_progress returned when multiple",
			todos: []Todo{
				{Content: "Task 1", Status: StatusInProgress},
				{Content: "Task 2", Status: StatusInProgress},
			},
			wantNil:  false,
			wantTask: "Task 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := NewStore()
			store.Set(tt.todos)

			result := store.InProgress()

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
			} else {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if result.Content != tt.wantTask {
					t.Errorf("expected Content %q, got %q", tt.wantTask, result.Content)
				}
			}
		})
	}
}

func TestHasItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		todos  []Todo
		expect bool
	}{
		{
			name:   "empty store",
			todos:  []Todo{},
			expect: false,
		},
		{
			name: "one item",
			todos: []Todo{
				{Content: "Task", Status: StatusPending},
			},
			expect: true,
		},
		{
			name: "multiple items",
			todos: []Todo{
				{Content: "Task 1", Status: StatusPending},
				{Content: "Task 2", Status: StatusCompleted},
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := NewStore()
			store.Set(tt.todos)

			if got := store.HasItems(); got != tt.expect {
				t.Errorf("HasItems() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		todos          []Todo
		wantPending    int
		wantInProgress int
		wantCompleted  int
	}{
		{
			name:           "empty store",
			todos:          []Todo{},
			wantPending:    0,
			wantInProgress: 0,
			wantCompleted:  0,
		},
		{
			name: "all pending",
			todos: []Todo{
				{Status: StatusPending},
				{Status: StatusPending},
				{Status: StatusPending},
			},
			wantPending:    3,
			wantInProgress: 0,
			wantCompleted:  0,
		},
		{
			name: "all completed",
			todos: []Todo{
				{Status: StatusCompleted},
				{Status: StatusCompleted},
			},
			wantPending:    0,
			wantInProgress: 0,
			wantCompleted:  2,
		},
		{
			name: "all in_progress",
			todos: []Todo{
				{Status: StatusInProgress},
			},
			wantPending:    0,
			wantInProgress: 1,
			wantCompleted:  0,
		},
		{
			name: "mixed statuses",
			todos: []Todo{
				{Status: StatusPending},
				{Status: StatusInProgress},
				{Status: StatusCompleted},
				{Status: StatusPending},
				{Status: StatusCompleted},
			},
			wantPending:    2,
			wantInProgress: 1,
			wantCompleted:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := NewStore()
			store.Set(tt.todos)

			pending, inProgress, completed := store.Stats()

			if pending != tt.wantPending {
				t.Errorf("pending: got %d, want %d", pending, tt.wantPending)
			}
			if inProgress != tt.wantInProgress {
				t.Errorf("inProgress: got %d, want %d", inProgress, tt.wantInProgress)
			}
			if completed != tt.wantCompleted {
				t.Errorf("completed: got %d, want %d", completed, tt.wantCompleted)
			}
		})
	}
}

func TestStoreConcurrency(t *testing.T) {
	t.Parallel()

	store := NewStore()
	var wg sync.WaitGroup

	// Run concurrent operations
	for i := 0; i < 100; i++ {
		wg.Add(4)

		go func() {
			defer wg.Done()
			store.Set([]Todo{{Content: "Task", Status: StatusPending}})
		}()

		go func() {
			defer wg.Done()
			_ = store.List()
		}()

		go func() {
			defer wg.Done()
			_ = store.InProgress()
		}()

		go func() {
			defer wg.Done()
			_ = store.HasItems()
		}()
	}

	wg.Wait()
	// Test passes if no race conditions or deadlocks occur
}
