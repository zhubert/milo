package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhubert/milo/internal/tool"
)

func TestBuildSystemPrompt(t *testing.T) {
	t.Parallel()

	registry := tool.NewRegistry()
	if err := registry.Register(&tool.ReadTool{}); err != nil {
		t.Fatalf("registering read tool: %v", err)
	}
	if err := registry.Register(&tool.BashTool{}); err != nil {
		t.Fatalf("registering bash tool: %v", err)
	}

	prompt := BuildSystemPrompt("/home/user/project", registry)

	checks := []string{
		"/home/user/project",
		"read",
		"bash",
		"Available Tools",
		"Environment",
	}
	for _, s := range checks {
		if !strings.Contains(prompt, s) {
			t.Errorf("system prompt should contain %q", s)
		}
	}
}

func TestBuildSystemPromptWithAgentsFile(t *testing.T) {
	t.Parallel()

	// Create a temporary directory with AGENTS.md
	tempDir := t.TempDir()
	agentsContent := `# Project Guidelines

This is a Go project with specific guidelines.

## Testing
- Use table-driven tests
- Always check errors`

	agentsPath := filepath.Join(tempDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(agentsContent), 0644); err != nil {
		t.Fatalf("writing AGENTS.md: %v", err)
	}

	registry := tool.NewRegistry()
	if err := registry.Register(&tool.ReadTool{}); err != nil {
		t.Fatalf("registering read tool: %v", err)
	}

	prompt := BuildSystemPrompt(tempDir, registry)

	// Should contain the AGENTS.md content
	if !strings.Contains(prompt, "Project Guidelines") {
		t.Error("system prompt should contain content from AGENTS.md")
	}
	if !strings.Contains(prompt, "table-driven tests") {
		t.Error("system prompt should contain specific guidelines from AGENTS.md")
	}
	if !strings.Contains(prompt, "Project Configuration") {
		t.Error("system prompt should have Project Configuration section when agent file is present")
	}
}

func TestBuildSystemPromptWithClaudeFile(t *testing.T) {
	t.Parallel()

	// Create a temporary directory with CLAUDE.md (no AGENTS.md)
	tempDir := t.TempDir()
	claudeContent := `# CLAUDE.md

## Language
This is a Go project.

## Error Handling
Every error must be explicitly handled.`

	claudePath := filepath.Join(tempDir, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte(claudeContent), 0644); err != nil {
		t.Fatalf("writing CLAUDE.md: %v", err)
	}

	registry := tool.NewRegistry()
	if err := registry.Register(&tool.ReadTool{}); err != nil {
		t.Fatalf("registering read tool: %v", err)
	}

	prompt := BuildSystemPrompt(tempDir, registry)

	// Should contain the CLAUDE.md content
	if !strings.Contains(prompt, "This is a Go project") {
		t.Error("system prompt should contain content from CLAUDE.md")
	}
	if !strings.Contains(prompt, "Error Handling") {
		t.Error("system prompt should contain specific sections from CLAUDE.md")
	}
	if !strings.Contains(prompt, "Project Configuration") {
		t.Error("system prompt should have Project Configuration section when agent file is present")
	}
}

func TestBuildSystemPromptPrefersAgentsOverClaude(t *testing.T) {
	t.Parallel()

	// Create a temporary directory with both AGENTS.md and CLAUDE.md
	tempDir := t.TempDir()

	agentsContent := `# AGENTS.md Content
This should be preferred.`
	claudeContent := `# CLAUDE.md Content
This should be ignored when AGENTS.md exists.`

	agentsPath := filepath.Join(tempDir, "AGENTS.md")
	claudePath := filepath.Join(tempDir, "CLAUDE.md")

	if err := os.WriteFile(agentsPath, []byte(agentsContent), 0644); err != nil {
		t.Fatalf("writing AGENTS.md: %v", err)
	}
	if err := os.WriteFile(claudePath, []byte(claudeContent), 0644); err != nil {
		t.Fatalf("writing CLAUDE.md: %v", err)
	}

	registry := tool.NewRegistry()
	if err := registry.Register(&tool.ReadTool{}); err != nil {
		t.Fatalf("registering read tool: %v", err)
	}

	prompt := BuildSystemPrompt(tempDir, registry)

	// Should contain AGENTS.md content, not CLAUDE.md content
	if !strings.Contains(prompt, "AGENTS.md Content") {
		t.Error("system prompt should contain content from AGENTS.md")
	}
	if strings.Contains(prompt, "CLAUDE.md Content") {
		t.Error("system prompt should not contain content from CLAUDE.md when AGENTS.md exists")
	}
}

func TestReadAgentConfig(t *testing.T) {
	t.Parallel()

	t.Run("no files exist", func(t *testing.T) {
		tempDir := t.TempDir()
		content := readAgentConfig(tempDir)
		if content != "" {
			t.Error("expected empty string when no agent config files exist")
		}
	})

	t.Run("AGENTS.md exists", func(t *testing.T) {
		tempDir := t.TempDir()
		expected := "# Test content"
		agentsPath := filepath.Join(tempDir, "AGENTS.md")
		if err := os.WriteFile(agentsPath, []byte(expected), 0644); err != nil {
			t.Fatalf("writing AGENTS.md: %v", err)
		}

		content := readAgentConfig(tempDir)
		if content != expected {
			t.Errorf("expected %q, got %q", expected, content)
		}
	})

	t.Run("CLAUDE.md exists", func(t *testing.T) {
		tempDir := t.TempDir()
		expected := "# Claude content"
		claudePath := filepath.Join(tempDir, "CLAUDE.md")
		if err := os.WriteFile(claudePath, []byte(expected), 0644); err != nil {
			t.Fatalf("writing CLAUDE.md: %v", err)
		}

		content := readAgentConfig(tempDir)
		if content != expected {
			t.Errorf("expected %q, got %q", expected, content)
		}
	})
}
