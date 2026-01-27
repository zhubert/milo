package tool

import (
	"fmt"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
)

// Registry manages the set of tools available to the agent.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string
}

// NewRegistry creates a new empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry. Returns an error if a tool
// with the same name is already registered.
func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}
	r.tools[name] = t
	r.order = append(r.order, name)
	return nil
}

// Lookup returns the tool with the given name, or nil if not found.
func (r *Registry) Lookup(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all registered tools in registration order.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.tools[name])
	}
	return result
}

// ToolParams returns the Anthropic API tool parameter definitions
// for all registered tools.
func (r *Registry) ToolParams() []anthropic.ToolUnionParam {
	r.mu.RLock()
	defer r.mu.RUnlock()

	params := make([]anthropic.ToolUnionParam, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		param := anthropic.ToolUnionParamOfTool(t.InputSchema(), t.Name())
		param.OfTool.Description = anthropic.String(t.Description())
		params = append(params, param)
	}
	return params
}
