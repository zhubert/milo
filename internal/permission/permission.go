package permission

import "sync"

// Action represents the permission decision for a tool.
type Action int

const (
	// Allow permits the tool to execute without prompting.
	Allow Action = iota
	// Deny blocks the tool from executing.
	Deny
	// Ask requires user confirmation before executing.
	Ask
)

// Checker evaluates whether a tool is permitted to run.
type Checker struct {
	mu            sync.RWMutex
	rules         map[string]Action
	sessionAlways map[string]bool
}

// NewChecker creates a permission checker with default rules.
// Default: read → allow, write → ask, edit → ask, bash → ask.
func NewChecker() *Checker {
	return &Checker{
		rules: map[string]Action{
			"read":  Allow,
			"write": Ask,
			"edit":  Ask,
			"bash":  Ask,
		},
		sessionAlways: make(map[string]bool),
	}
}

// Check returns the permission action for the given tool.
// If the tool has been allowed-always for this session, returns Allow.
// If no rule exists, defaults to Ask.
func (c *Checker) Check(toolName string) Action {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.sessionAlways[toolName] {
		return Allow
	}

	action, ok := c.rules[toolName]
	if !ok {
		return Ask
	}
	return action
}

// AllowAlways marks a tool as allowed for the remainder of this session.
func (c *Checker) AllowAlways(toolName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionAlways[toolName] = true
}

// SetRule sets the permission rule for a specific tool.
func (c *Checker) SetRule(toolName string, action Action) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules[toolName] = action
}
