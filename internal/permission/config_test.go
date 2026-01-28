package permission

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    Action
		wantErr bool
	}{
		{"allow", Allow, false},
		{"deny", Deny, false},
		{"ask", Ask, false},
		{"invalid", Ask, true},
		{"ALLOW", Ask, true}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := parseAction(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAction(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseAction(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	// Create temp dir and config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "permissions.yaml")

	configContent := `rules:
  - tool: bash
    pattern: "npm *"
    action: allow
  - tool: write
    pattern: "*.tmp"
    action: allow
  - tool: bash
    pattern: "rm -rf*"
    action: deny
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if len(cfg.Rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(cfg.Rules))
	}

	// Verify first rule
	if cfg.Rules[0].Tool != "bash" || cfg.Rules[0].Pattern != "npm *" || cfg.Rules[0].Action != "allow" {
		t.Errorf("first rule mismatch: %+v", cfg.Rules[0])
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	t.Parallel()

	_, err := LoadConfig("/nonexistent/path/permissions.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "permissions.yaml")

	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestCheckerLoadFromFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "permissions.yaml")

	configContent := `rules:
  - tool: bash
    pattern: "npm install"
    action: allow
  - tool: bash
    pattern: "yarn *"
    action: allow
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	c := NewChecker()
	loaded, err := c.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if loaded != 2 {
		t.Errorf("expected 2 rules loaded, got %d", loaded)
	}

	// Verify the rules work
	input := makeInput(map[string]interface{}{"command": "npm install"})
	if got := c.Check("bash", input); got != Allow {
		t.Errorf("npm install should be allowed after config load, got %v", got)
	}

	input2 := makeInput(map[string]interface{}{"command": "yarn add react"})
	if got := c.Check("bash", input2); got != Allow {
		t.Errorf("yarn add should be allowed after config load, got %v", got)
	}
}

func TestCheckerLoadFromFileWithDefaults(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "permissions.yaml")

	// Config with missing pattern and action - should use defaults
	configContent := `rules:
  - tool: bash
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	c := NewChecker()
	loaded, err := c.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if loaded != 1 {
		t.Errorf("expected 1 rule loaded, got %d", loaded)
	}
}

func TestCheckerLoadFromFileInvalidAction(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "permissions.yaml")

	configContent := `rules:
  - tool: bash
    pattern: "*"
    action: invalid_action
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	c := NewChecker()
	_, err := c.LoadFromFile(configPath)
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestCheckerLoadFromFileMissingTool(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "permissions.yaml")

	configContent := `rules:
  - pattern: "*"
    action: allow
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	c := NewChecker()
	_, err := c.LoadFromFile(configPath)
	if err == nil {
		t.Error("expected error for missing tool")
	}
}

func TestCheckerLoadFromDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	miloDir := filepath.Join(tmpDir, ".milo")
	if err := os.MkdirAll(miloDir, 0755); err != nil {
		t.Fatalf("failed to create .milo dir: %v", err)
	}

	configContent := `rules:
  - tool: bash
    pattern: "make test"
    action: allow
`
	configPath := filepath.Join(miloDir, "permissions.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	c := NewChecker()
	loaded, err := c.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if loaded != 1 {
		t.Errorf("expected 1 rule loaded, got %d", loaded)
	}

	input := makeInput(map[string]interface{}{"command": "make test"})
	if got := c.Check("bash", input); got != Allow {
		t.Errorf("make test should be allowed after config load, got %v", got)
	}
}

func TestCheckerLoadFromDirectoryNoConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	c := NewChecker()
	loaded, err := c.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if loaded != 0 {
		t.Errorf("expected 0 rules loaded for missing config, got %d", loaded)
	}
}

func TestNewCheckerWithConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	miloDir := filepath.Join(tmpDir, ".milo")
	if err := os.MkdirAll(miloDir, 0755); err != nil {
		t.Fatalf("failed to create .milo dir: %v", err)
	}

	configContent := `rules:
  - tool: bash
    pattern: "docker *"
    action: allow
`
	configPath := filepath.Join(miloDir, "permissions.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	c, err := NewCheckerWithConfig(tmpDir)
	if err != nil {
		t.Fatalf("NewCheckerWithConfig() error = %v", err)
	}

	// Custom rule should work
	input := makeInput(map[string]interface{}{"command": "docker build ."})
	if got := c.Check("bash", input); got != Allow {
		t.Errorf("docker build should be allowed, got %v", got)
	}

	// Default rules should still work
	input2 := makeInput(map[string]interface{}{"command": "git status"})
	if got := c.Check("bash", input2); got != Allow {
		t.Errorf("git status should still be allowed (default rule), got %v", got)
	}
}

func TestNewCheckerWithConfigNoConfigFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	c, err := NewCheckerWithConfig(tmpDir)
	if err != nil {
		t.Fatalf("NewCheckerWithConfig() error = %v", err)
	}

	// Default rules should work
	input := makeInput(map[string]interface{}{"command": "git status"})
	if got := c.Check("bash", input); got != Allow {
		t.Errorf("git status should be allowed (default rule), got %v", got)
	}
}

func TestConfigRulesPrecedence(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	miloDir := filepath.Join(tmpDir, ".milo")
	if err := os.MkdirAll(miloDir, 0755); err != nil {
		t.Fatalf("failed to create .milo dir: %v", err)
	}

	// Config that allows go build (which defaults to Ask)
	configContent := `rules:
  - tool: bash
    pattern: "go build*"
    action: allow
`
	configPath := filepath.Join(miloDir, "permissions.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	c, err := NewCheckerWithConfig(tmpDir)
	if err != nil {
		t.Fatalf("NewCheckerWithConfig() error = %v", err)
	}

	// Custom rule should take precedence (more specific pattern)
	input := makeInput(map[string]interface{}{"command": "go build ./..."})
	if got := c.Check("bash", input); got != Allow {
		t.Errorf("go build should be allowed by custom rule, got %v", got)
	}
}

func TestSaveToDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	c := NewChecker()
	c.AddRule(Rule{Tool: "bash", Pattern: "npm *", Action: Allow})
	c.AddRule(Rule{Tool: "bash", Pattern: "make*", Action: Allow})

	if err := c.SaveToDirectory(tmpDir); err != nil {
		t.Fatalf("SaveToDirectory() error = %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpDir, ".milo", "permissions.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load it back and verify
	c2 := NewChecker()
	loaded, err := c2.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if loaded != 2 {
		t.Errorf("expected 2 rules loaded, got %d", loaded)
	}

	input := makeInput(map[string]interface{}{"command": "npm install"})
	if got := c2.Check("bash", input); got != Allow {
		t.Errorf("npm install should be allowed after reload, got %v", got)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	c := NewChecker()
	c.SetWorkDir(tmpDir)
	c.AddRule(Rule{Tool: "bash", Pattern: "test*", Action: Allow})

	if err := c.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify .milo directory was created
	miloDir := filepath.Join(tmpDir, ".milo")
	if _, err := os.Stat(miloDir); os.IsNotExist(err) {
		t.Fatal(".milo directory was not created")
	}
}

func TestSaveWithoutWorkDir(t *testing.T) {
	t.Parallel()

	c := NewChecker()
	// Don't set workDir

	err := c.Save()
	if err == nil {
		t.Error("expected error when saving without workDir")
	}
}

func TestAllowAlwaysPersists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	c, err := NewCheckerWithConfig(tmpDir)
	if err != nil {
		t.Fatalf("NewCheckerWithConfig() error = %v", err)
	}

	// AllowAlways should persist
	input := makeInput(map[string]interface{}{"command": "npm install"})
	if err := c.AllowAlways("bash", input); err != nil {
		t.Fatalf("AllowAlways() error = %v", err)
	}

	// Load config again
	c2, err := NewCheckerWithConfig(tmpDir)
	if err != nil {
		t.Fatalf("NewCheckerWithConfig() error = %v", err)
	}

	// Should be allowed due to persisted rule
	if got := c2.Check("bash", input); got != Allow {
		t.Errorf("npm install should be allowed after reload, got %v", got)
	}
}

func TestRulesAreUnique(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// Add the same rule twice
	c.AddRule(Rule{Tool: "bash", Pattern: "npm *", Action: Allow})
	c.AddRule(Rule{Tool: "bash", Pattern: "npm *", Action: Deny})

	// Should only have one rule with that key
	rules := c.CustomRules()
	count := 0
	for _, r := range rules {
		if r.Tool == "bash" && r.Pattern == "npm *" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("expected 1 rule with key bash:npm *, got %d", count)
	}

	// The second (Deny) should have replaced the first
	input := makeInput(map[string]interface{}{"command": "npm install"})
	if got := c.Check("bash", input); got != Deny {
		t.Errorf("expected Deny (second rule), got %v", got)
	}
}

func TestRemoveRule(t *testing.T) {
	t.Parallel()

	c := NewChecker()
	c.AddRule(Rule{Tool: "bash", Pattern: "npm *", Action: Allow})

	// Verify rule exists
	input := makeInput(map[string]interface{}{"command": "npm install"})
	if got := c.Check("bash", input); got != Allow {
		t.Fatalf("npm install should be allowed, got %v", got)
	}

	// Remove it
	if !c.RemoveRule("bash:npm *") {
		t.Error("RemoveRule should return true for existing rule")
	}

	// Verify rule is gone (falls back to default Ask)
	if got := c.Check("bash", input); got != Ask {
		t.Errorf("npm install should ask after removal, got %v", got)
	}

	// Removing again should return false
	if c.RemoveRule("bash:npm *") {
		t.Error("RemoveRule should return false for non-existent rule")
	}
}

func TestCustomRulesReturnsOnlyCustom(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// Initially, CustomRules should be empty (defaults are not custom)
	if len(c.CustomRules()) != 0 {
		t.Errorf("expected 0 custom rules initially, got %d", len(c.CustomRules()))
	}

	// Add a custom rule
	c.AddRule(Rule{Tool: "bash", Pattern: "custom*", Action: Allow})

	if len(c.CustomRules()) != 1 {
		t.Errorf("expected 1 custom rule, got %d", len(c.CustomRules()))
	}

	// Rules() should return both default and custom
	allRules := c.Rules()
	if len(allRules) <= 1 {
		t.Errorf("expected many rules (default + custom), got %d", len(allRules))
	}
}
