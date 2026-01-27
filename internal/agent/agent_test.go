package agent

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/zhubert/looper/internal/tool"
)

func TestNewAgent(t *testing.T) {
	t.Parallel()

	client := anthropic.NewClient()
	registry := tool.NewRegistry()
	if err := registry.Register(&tool.ReadTool{}); err != nil {
		t.Fatalf("registering tool: %v", err)
	}

	ag := New(client, registry, "/tmp/test")
	if ag == nil {
		t.Fatal("expected non-nil agent")
	}
	if ag.conv.Len() != 0 {
		t.Errorf("expected empty conversation, got %d messages", ag.conv.Len())
	}
}

func TestAgentSystemPromptIncludesWorkDir(t *testing.T) {
	t.Parallel()

	registry := tool.NewRegistry()
	if err := registry.Register(&tool.ReadTool{}); err != nil {
		t.Fatalf("registering tool: %v", err)
	}
	if err := registry.Register(&tool.WriteTool{}); err != nil {
		t.Fatalf("registering tool: %v", err)
	}
	if err := registry.Register(&tool.EditTool{}); err != nil {
		t.Fatalf("registering tool: %v", err)
	}
	if err := registry.Register(&tool.BashTool{}); err != nil {
		t.Fatalf("registering tool: %v", err)
	}

	prompt := BuildSystemPrompt("/home/user/project", registry)

	// Verify all tool names appear.
	for _, name := range []string{"read", "write", "edit", "bash"} {
		if !containsString(prompt, name) {
			t.Errorf("system prompt should mention tool %q", name)
		}
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
