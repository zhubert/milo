package permission

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the structure of a permissions config file.
type Config struct {
	Rules []string `yaml:"rules"`
}

// parseAction converts a string action to an Action type.
func parseAction(s string) (Action, error) {
	switch s {
	case "allow":
		return Allow, nil
	case "deny":
		return Deny, nil
	case "ask":
		return Ask, nil
	default:
		return Ask, fmt.Errorf("unknown action %q, must be allow, deny, or ask", s)
	}
}

// LoadConfig loads a permissions config from the given path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

// LoadFromFile loads rules from a YAML config file and adds them to the checker.
// Returns the number of rules loaded.
func (c *Checker) LoadFromFile(path string) (int, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return 0, err
	}

	return c.loadFromConfig(cfg)
}

func (c *Checker) loadFromConfig(cfg *Config) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	loaded := 0
	for i, ruleStr := range cfg.Rules {
		rule, err := ParseRule(ruleStr)
		if err != nil {
			return loaded, fmt.Errorf("rule %d: %w", i, err)
		}
		c.customRules[rule.Key()] = rule
		loaded++
	}

	return loaded, nil
}

// LoadFromDirectory loads permissions from .milo/permissions.yaml in the given directory.
// If the file doesn't exist, it returns (0, nil) - no error.
func (c *Checker) LoadFromDirectory(dir string) (int, error) {
	path := filepath.Join(dir, ".milo", "permissions.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	}
	return c.LoadFromFile(path)
}

// LoadGlobal loads permissions from ~/.milo/permissions.yaml.
// If the file doesn't exist, it returns (0, nil) - no error.
func (c *Checker) LoadGlobal() (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, nil // Can't find home dir, skip global config
	}

	path := filepath.Join(home, ".milo", "permissions.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	}
	return c.LoadFromFile(path)
}

// SaveToDirectory saves custom rules to .milo/permissions.yaml in the given directory.
func (c *Checker) SaveToDirectory(dir string) error {
	c.mu.RLock()
	rules := make([]string, 0, len(c.customRules))
	for _, rule := range c.customRules {
		rules = append(rules, rule.String())
	}
	c.mu.RUnlock()

	// Build config
	cfg := Config{
		Rules: rules,
	}

	// Create directory if needed
	miloDir := filepath.Join(dir, ".milo")
	if err := os.MkdirAll(miloDir, 0755); err != nil {
		return fmt.Errorf("creating .milo directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Write file
	path := filepath.Join(miloDir, "permissions.yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Save saves custom rules to the config file in the current working directory.
func (c *Checker) Save() error {
	workDir := c.WorkDir()
	if workDir == "" {
		return fmt.Errorf("no working directory set")
	}
	return c.SaveToDirectory(workDir)
}

// NewCheckerWithConfig creates a new Checker and loads rules from config files.
// It loads in order: global config, then repo config (repo rules take precedence).
func NewCheckerWithConfig(workDir string) (*Checker, error) {
	c := NewChecker()
	c.workDir = workDir

	// Load global config first (lower precedence)
	if _, err := c.LoadGlobal(); err != nil {
		return nil, fmt.Errorf("loading global config: %w", err)
	}

	// Load repo config (higher precedence due to specificity)
	if _, err := c.LoadFromDirectory(workDir); err != nil {
		return nil, fmt.Errorf("loading repo config: %w", err)
	}

	return c, nil
}
