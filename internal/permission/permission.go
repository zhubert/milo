package permission

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
)

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

// String returns a human-readable representation of the action.
func (a Action) String() string {
	switch a {
	case Allow:
		return "allow"
	case Deny:
		return "deny"
	case Ask:
		return "ask"
	default:
		return "unknown"
	}
}

// Rule defines a permission rule that matches tool invocations.
// A rule matches when both the tool name matches and the pattern
// matches the relevant input field (command for bash, path for file tools).
type Rule struct {
	// Tool is the name of the tool this rule applies to.
	// Use "*" to match any tool.
	Tool string

	// Pattern is a glob pattern to match against the tool's input.
	// For bash: matches against the command string.
	// For file tools (read, write, edit, glob, grep): matches against the path.
	// Use "*" to match any input.
	Pattern string

	// Action is the permission action to take when this rule matches.
	Action Action
}

// Matches checks if this rule matches the given tool name and input.
func (r *Rule) Matches(toolName, input string) bool {
	// Check tool name match
	if r.Tool != "*" && r.Tool != toolName {
		return false
	}

	// Check pattern match
	if r.Pattern == "*" {
		return true
	}

	// Use filepath.Match for glob matching
	matched, err := filepath.Match(r.Pattern, input)
	if err != nil {
		// Invalid pattern, try prefix match as fallback
		return strings.HasPrefix(input, strings.TrimSuffix(r.Pattern, "*"))
	}
	if matched {
		return true
	}

	// For path patterns, also try matching just the base name
	if isFilePathTool(toolName) {
		baseName := filepath.Base(input)
		matched, err = filepath.Match(r.Pattern, baseName)
		if err == nil && matched {
			return true
		}
	}

	// For command patterns, try prefix matching (e.g., "git *" matches "git status")
	if toolName == "bash" && strings.HasSuffix(r.Pattern, "*") {
		prefix := strings.TrimSuffix(r.Pattern, "*")
		if strings.HasPrefix(input, prefix) {
			return true
		}
	}

	return false
}

// Specificity returns a score indicating how specific this rule is.
// Higher scores mean more specific rules.
func (r *Rule) Specificity() int {
	score := 0

	// Specific tool name is more specific than wildcard
	if r.Tool != "*" {
		score += 100
	}

	// Specific pattern is more specific than wildcard
	if r.Pattern != "*" {
		score += 50
		// Longer patterns are generally more specific
		score += len(r.Pattern)
	}

	return score
}

func isFilePathTool(toolName string) bool {
	switch toolName {
	case "read", "write", "edit", "glob", "grep":
		return true
	default:
		return false
	}
}

// Checker evaluates whether a tool is permitted to run.
type Checker struct {
	mu            sync.RWMutex
	rules         []Rule
	sessionAlways map[string]bool
	defaultAction Action
}

// NewChecker creates a permission checker with default rules.
func NewChecker() *Checker {
	c := &Checker{
		rules:         make([]Rule, 0),
		sessionAlways: make(map[string]bool),
		defaultAction: Ask,
	}

	// Add default rules for safe operations
	c.addDefaultRules()

	return c
}

func (c *Checker) addDefaultRules() {
	// Read-only tools are safe by default
	c.rules = append(c.rules,
		Rule{Tool: "read", Pattern: "*", Action: Allow},
		Rule{Tool: "glob", Pattern: "*", Action: Allow},
		Rule{Tool: "grep", Pattern: "*", Action: Allow},
	)

	// Safe bash commands that don't modify state
	safeCommands := []string{
		"git status*",
		"git log*",
		"git diff*",
		"git branch*",
		"git show*",
		"git remote*",
		"ls*",
		"pwd*",
		"cat *",
		"head *",
		"tail *",
		"wc *",
		"which *",
		"whereis *",
		"echo *",
		"date*",
		"whoami*",
		"hostname*",
		"uname*",
		"env*",
		"printenv*",
		"go version*",
		"go list*",
		"go mod graph*",
		"npm list*",
		"npm --version*",
		"node --version*",
		"python --version*",
		"python3 --version*",
	}

	for _, cmd := range safeCommands {
		c.rules = append(c.rules, Rule{Tool: "bash", Pattern: cmd, Action: Allow})
	}

	// Dangerous patterns that should always be denied
	dangerousPatterns := []string{
		"rm -rf /*",
		"rm -rf .*",
		"chmod -R 777*",
		":(){ :|:& };:*", // fork bomb
		"> /dev/sd*",
		"dd if=*of=/dev/*",
		"mkfs*",
		"wget * | sh*",
		"curl * | sh*",
		"wget * | bash*",
		"curl * | bash*",
	}

	for _, pattern := range dangerousPatterns {
		c.rules = append(c.rules, Rule{Tool: "bash", Pattern: pattern, Action: Deny})
	}

	// Sensitive file patterns that should require confirmation
	sensitiveFiles := []string{
		"*.env",
		"*/.env",
		"*.pem",
		"*.key",
		"*id_rsa*",
		"*id_ed25519*",
		"*.secret*",
		"*credentials*",
		"*password*",
	}

	for _, pattern := range sensitiveFiles {
		c.rules = append(c.rules,
			Rule{Tool: "write", Pattern: pattern, Action: Ask},
			Rule{Tool: "edit", Pattern: pattern, Action: Ask},
			Rule{Tool: "read", Pattern: pattern, Action: Ask},
		)
	}

	// Default ask for write operations
	c.rules = append(c.rules,
		Rule{Tool: "write", Pattern: "*", Action: Ask},
		Rule{Tool: "edit", Pattern: "*", Action: Ask},
		Rule{Tool: "bash", Pattern: "*", Action: Ask},
	)
}

// Check returns the permission action for the given tool and input.
// It evaluates rules in order of specificity, returning the action
// of the most specific matching rule.
func (c *Checker) Check(toolName string, toolInput json.RawMessage) Action {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Extract the relevant input string based on tool type
	input := extractInputString(toolName, toolInput)

	// Check session-level always-allow first
	sessionKey := toolName + ":" + input
	if c.sessionAlways[sessionKey] {
		return Allow
	}
	// Also check tool-level always-allow for backwards compatibility
	if c.sessionAlways[toolName] {
		return Allow
	}

	// Find the most specific matching rule
	var bestMatch *Rule
	bestScore := -1

	for i := range c.rules {
		rule := &c.rules[i]
		if rule.Matches(toolName, input) {
			score := rule.Specificity()
			if score > bestScore {
				bestScore = score
				bestMatch = rule
			}
		}
	}

	if bestMatch != nil {
		return bestMatch.Action
	}

	return c.defaultAction
}

// extractInputString extracts the relevant string from tool input for matching.
func extractInputString(toolName string, toolInput json.RawMessage) string {
	if len(toolInput) == 0 {
		return ""
	}

	var data map[string]interface{}
	if err := json.Unmarshal(toolInput, &data); err != nil {
		return ""
	}

	switch toolName {
	case "bash":
		if cmd, ok := data["command"].(string); ok {
			return cmd
		}
	case "read", "write":
		if path, ok := data["file_path"].(string); ok {
			return path
		}
	case "edit":
		if path, ok := data["file_path"].(string); ok {
			return path
		}
	case "glob":
		if path, ok := data["path"].(string); ok {
			return path
		}
		if pattern, ok := data["pattern"].(string); ok {
			return pattern
		}
	case "grep":
		if path, ok := data["path"].(string); ok {
			return path
		}
	}

	return ""
}

// AllowAlways marks a specific tool+input combination as allowed for this session.
func (c *Checker) AllowAlways(toolName string, toolInput json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	input := extractInputString(toolName, toolInput)
	if input != "" {
		c.sessionAlways[toolName+":"+input] = true
	}
	// Also allow the tool generally for backwards compatibility
	c.sessionAlways[toolName] = true
}

// AllowToolAlways marks a tool as allowed for all inputs for this session.
func (c *Checker) AllowToolAlways(toolName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionAlways[toolName] = true
}

// AddRule adds a custom rule to the checker.
// Rules are evaluated by specificity, with more specific rules taking precedence.
func (c *Checker) AddRule(rule Rule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules = append(c.rules, rule)
}

// SetDefaultAction sets the default action for unmatched tools.
func (c *Checker) SetDefaultAction(action Action) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultAction = action
}

// Rules returns a copy of the current rules (for inspection/debugging).
func (c *Checker) Rules() []Rule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Rule, len(c.rules))
	copy(result, c.rules)
	return result
}
