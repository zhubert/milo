package agent

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/zhubert/milo/internal/permission"
	"github.com/zhubert/milo/internal/todo"
	"github.com/zhubert/milo/internal/tool"
)

func TestNewAgent(t *testing.T) {
	t.Parallel()

	client := anthropic.NewClient()
	registry := tool.NewRegistry()
	if err := registry.Register(&tool.ReadTool{}); err != nil {
		t.Fatalf("registering tool: %v", err)
	}

	perms := permission.NewChecker()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	todoStore := todo.NewStore()
	ag := New(client, registry, perms, "/tmp/test", logger, DefaultModel, todoStore)
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
	for _, tl := range []tool.Tool{
		&tool.ReadTool{},
		&tool.WriteTool{},
		&tool.EditTool{},
		&tool.BashTool{},
	} {
		if err := registry.Register(tl); err != nil {
			t.Fatalf("registering tool: %v", err)
		}
	}

	prompt := BuildSystemPrompt("/home/user/project", registry)

	for _, name := range []string{"read", "write", "edit", "bash", "/home/user/project"} {
		if !strings.Contains(prompt, name) {
			t.Errorf("system prompt should contain %q", name)
		}
	}
}
