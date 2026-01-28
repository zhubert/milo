package permission

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
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

// Key returns a unique identifier for this rule (tool:pattern).
func (r *Rule) Key() string {
	return r.Tool + ":" + r.Pattern
}

// String returns the compact format: Tool(pattern) or Tool(pattern):action
// For allow (the default), the action suffix is omitted.
// Tool names are capitalized (Bash, Read, Write).
func (r *Rule) String() string {
	// Capitalize tool name
	tool := strings.Title(r.Tool)
	if r.Action == Allow {
		return fmt.Sprintf("%s(%s)", tool, r.Pattern)
	}
	return fmt.Sprintf("%s(%s):%s", tool, r.Pattern, r.Action.String())
}

// ParseRule parses a compact rule string like "Bash(npm *)" or "Bash(rm -rf *):deny"
// into a Rule. If no action is specified, it defaults to Allow.
func ParseRule(s string) (Rule, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Rule{}, fmt.Errorf("empty rule string")
	}

	// Find the opening parenthesis
	openParen := strings.Index(s, "(")
	if openParen == -1 {
		return Rule{}, fmt.Errorf("invalid rule format %q: missing '('", s)
	}

	// Find the closing parenthesis
	closeParen := strings.LastIndex(s, ")")
	if closeParen == -1 || closeParen < openParen {
		return Rule{}, fmt.Errorf("invalid rule format %q: missing ')'", s)
	}

	tool := strings.TrimSpace(s[:openParen])
	if tool == "" {
		return Rule{}, fmt.Errorf("invalid rule format %q: missing tool name", s)
	}

	pattern := s[openParen+1 : closeParen]
	if pattern == "" {
		pattern = "*"
	}

	// Check for action suffix after the closing paren
	action := Allow // default
	remainder := strings.TrimSpace(s[closeParen+1:])
	if remainder != "" {
		if !strings.HasPrefix(remainder, ":") {
			return Rule{}, fmt.Errorf("invalid rule format %q: expected ':action' after ')'", s)
		}
		actionStr := strings.TrimPrefix(remainder, ":")
		switch strings.ToLower(actionStr) {
		case "allow":
			action = Allow
		case "deny":
			action = Deny
		case "ask":
			action = Ask
		default:
			return Rule{}, fmt.Errorf("invalid action %q in rule %q", actionStr, s)
		}
	}

	return Rule{
		Tool:    strings.ToLower(tool),
		Pattern: pattern,
		Action:  action,
	}, nil
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

	// For command patterns with colon syntax: "git:*" means "starts with git"
	if toolName == "bash" && strings.HasSuffix(r.Pattern, ":*") {
		prefix := strings.TrimSuffix(r.Pattern, ":*")
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
	defaultRules  []Rule            // Built-in rules (read-only)
	customRules   map[string]Rule   // User-defined rules keyed by tool:pattern
	sessionAlways map[string]bool   // Session-level always-allow
	defaultAction Action
	workDir       string            // Working directory for saving config
}

// NewChecker creates a permission checker with default rules.
func NewChecker() *Checker {
	c := &Checker{
		defaultRules:  make([]Rule, 0),
		customRules:   make(map[string]Rule),
		sessionAlways: make(map[string]bool),
		defaultAction: Ask,
	}

	// Add default rules for safe operations
	c.addDefaultRules()

	return c
}

func (c *Checker) addDefaultRules() {
	// Read-only tools are safe by default
	c.defaultRules = append(c.defaultRules,
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
		c.defaultRules = append(c.defaultRules, Rule{Tool: "bash", Pattern: cmd, Action: Allow})
	}

	// Dangerous patterns that should always be denied
	// Use :* suffix for prefix matching
	dangerousPatterns := []string{
		"rm -rf /:*",
		"rm -rf .:*",
		"chmod -R 777:*",
		":(){ :|:& };::*", // fork bomb
		"> /dev/sd:*",
		"mkfs:*",
	}

	for _, pattern := range dangerousPatterns {
		c.defaultRules = append(c.defaultRules, Rule{Tool: "bash", Pattern: pattern, Action: Deny})
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
		c.defaultRules = append(c.defaultRules,
			Rule{Tool: "write", Pattern: pattern, Action: Ask},
			Rule{Tool: "edit", Pattern: pattern, Action: Ask},
			Rule{Tool: "read", Pattern: pattern, Action: Ask},
		)
	}

	// Default ask for write operations
	c.defaultRules = append(c.defaultRules,
		Rule{Tool: "write", Pattern: "*", Action: Ask},
		Rule{Tool: "edit", Pattern: "*", Action: Ask},
		Rule{Tool: "bash", Pattern: "*", Action: Ask},
	)
}

// SetWorkDir sets the working directory for config file operations.
func (c *Checker) SetWorkDir(dir string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workDir = dir
}

// WorkDir returns the current working directory.
func (c *Checker) WorkDir() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.workDir
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

	// Combine all rules for evaluation
	allRules := c.allRules()

	// Find the most specific matching rule
	var bestMatch *Rule
	bestScore := -1

	for i := range allRules {
		rule := &allRules[i]
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

// allRules returns all rules (default + custom) for evaluation.
// Must be called with at least a read lock held.
func (c *Checker) allRules() []Rule {
	result := make([]Rule, 0, len(c.defaultRules)+len(c.customRules))
	result = append(result, c.defaultRules...)
	for _, rule := range c.customRules {
		result = append(result, rule)
	}
	return result
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

// AllowAlways marks a specific tool+input combination as allowed for this session
// and persists it to the config file.
func (c *Checker) AllowAlways(toolName string, toolInput json.RawMessage) error {
	input := extractInputString(toolName, toolInput)

	c.mu.Lock()
	// Session-level allow
	if input != "" {
		c.sessionAlways[toolName+":"+input] = true
	}
	c.sessionAlways[toolName] = true

	// Add as a custom rule for persistence
	pattern := input
	if pattern == "" {
		pattern = "*"
	}
	rule := Rule{Tool: toolName, Pattern: pattern, Action: Allow}
	c.customRules[rule.Key()] = rule
	workDir := c.workDir
	c.mu.Unlock()

	// Persist to config file
	if workDir != "" {
		return c.SaveToDirectory(workDir)
	}
	return nil
}

// AllowToolAlways marks a tool as allowed for all inputs for this session.
func (c *Checker) AllowToolAlways(toolName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionAlways[toolName] = true
}

// AddRule adds a custom rule to the checker.
// If a rule with the same tool:pattern already exists, it is replaced.
func (c *Checker) AddRule(rule Rule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.customRules[rule.Key()] = rule
}

// RemoveRule removes a custom rule by its key (tool:pattern).
// Returns true if a rule was removed.
func (c *Checker) RemoveRule(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.customRules[key]; exists {
		delete(c.customRules, key)
		return true
	}
	return false
}

// SetDefaultAction sets the default action for unmatched tools.
func (c *Checker) SetDefaultAction(action Action) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultAction = action
}

// Rules returns a copy of all rules (default + custom).
func (c *Checker) Rules() []Rule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.allRules()
}

// CustomRules returns a copy of only the custom (user-defined) rules.
func (c *Checker) CustomRules() []Rule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Rule, 0, len(c.customRules))
	for _, rule := range c.customRules {
		result = append(result, rule)
	}
	// Sort for consistent output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key() < result[j].Key()
	})
	return result
}
