package agent

import (
	"strings"
	"testing"

	"github.com/zhubert/looper/internal/tool"
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
