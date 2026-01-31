package tool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

// fakeTool is a minimal Tool implementation for testing.
type fakeTool struct {
	name string
}

func (f *fakeTool) Name() string        { return f.name }
func (f *fakeTool) Description() string { return "A fake tool for testing" }
func (f *fakeTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"input": map[string]any{"type": "string"},
		},
	}
}
func (f *fakeTool) Execute(_ context.Context, _ json.RawMessage) (Result, error) {
	return Result{Output: "ok"}, nil
}

func TestRegistryRegisterAndLookup(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	ft := &fakeTool{name: "test_tool"}

	if err := r.Register(ft); err != nil {
		t.Fatalf("unexpected error registering tool: %v", err)
	}

	got := r.Lookup("test_tool")
	if got == nil {
		t.Fatal("expected to find registered tool, got nil")
	}
	if got.Name() != "test_tool" {
		t.Errorf("expected name %q, got %q", "test_tool", got.Name())
	}
}

func TestRegistryLookupNotFound(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	got := r.Lookup("nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent tool, got %v", got)
	}
}

func TestRegistryDuplicateRegistration(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	ft := &fakeTool{name: "dup"}

	if err := r.Register(ft); err != nil {
		t.Fatalf("unexpected error on first register: %v", err)
	}

	err := r.Register(ft)
	if err == nil {
		t.Fatal("expected error on duplicate registration, got nil")
	}
}

func TestRegistryList(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	tools := []*fakeTool{
		{name: "alpha"},
		{name: "beta"},
		{name: "gamma"},
	}
	for _, ft := range tools {
		if err := r.Register(ft); err != nil {
			t.Fatalf("unexpected error registering %s: %v", ft.name, err)
		}
	}

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(list))
	}

	// Verify registration order is preserved.
	for i, ft := range tools {
		if list[i].Name() != ft.name {
			t.Errorf("position %d: expected %q, got %q", i, ft.name, list[i].Name())
		}
	}
}

func TestRegistryToolParams(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	if err := r.Register(&fakeTool{name: "read"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Register(&fakeTool{name: "write"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := r.ToolParams()
	if len(params) != 2 {
		t.Fatalf("expected 2 tool params, got %d", len(params))
	}
	if params[0].OfTool.Name != "read" {
		t.Errorf("expected first tool param name %q, got %q", "read", params[0].OfTool.Name)
	}
	if params[1].OfTool.Name != "write" {
		t.Errorf("expected second tool param name %q, got %q", "write", params[1].OfTool.Name)
	}
}
