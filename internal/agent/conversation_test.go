package agent

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestConversationAddUserMessage(t *testing.T) {
	t.Parallel()

	c := NewConversation()
	c.AddUserMessage("hello")

	msgs := c.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != anthropic.MessageParamRoleUser {
		t.Errorf("expected role %q, got %q", anthropic.MessageParamRoleUser, msgs[0].Role)
	}
}

func TestConversationAddAssistantMessage(t *testing.T) {
	t.Parallel()

	c := NewConversation()
	c.AddAssistantMessage(anthropic.NewTextBlock("I can help with that."))

	msgs := c.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != anthropic.MessageParamRoleAssistant {
		t.Errorf("expected role %q, got %q", anthropic.MessageParamRoleAssistant, msgs[0].Role)
	}
}

func TestConversationAddToolResult(t *testing.T) {
	t.Parallel()

	c := NewConversation()
	c.AddToolResult(anthropic.NewToolResultBlock("tool_123", "output", false))

	msgs := c.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != anthropic.MessageParamRoleUser {
		t.Errorf("expected role %q, got %q", anthropic.MessageParamRoleUser, msgs[0].Role)
	}
}

func TestConversationMultipleMessages(t *testing.T) {
	t.Parallel()

	c := NewConversation()
	c.AddUserMessage("please help")
	c.AddAssistantMessage(
		anthropic.NewTextBlock("Sure."),
		anthropic.NewToolUseBlock("t1", map[string]string{"command": "ls"}, "bash"),
	)
	c.AddToolResult(anthropic.NewToolResultBlock("t1", "file.txt", false))

	if c.Len() != 3 {
		t.Fatalf("expected 3 messages, got %d", c.Len())
	}

	msgs := c.Messages()
	if msgs[0].Role != anthropic.MessageParamRoleUser {
		t.Error("first message should be user")
	}
	if msgs[1].Role != anthropic.MessageParamRoleAssistant {
		t.Error("second message should be assistant")
	}
	if msgs[2].Role != anthropic.MessageParamRoleUser {
		t.Error("third message should be user (tool result)")
	}
}

func TestConversationMessagesReturnsCopy(t *testing.T) {
	t.Parallel()

	c := NewConversation()
	c.AddUserMessage("hello")

	msgs := c.Messages()
	msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock("extra")))

	if c.Len() != 1 {
		t.Error("modifying returned slice should not affect conversation")
	}
}
