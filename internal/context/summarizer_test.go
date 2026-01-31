package context

import (
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestFormatMessagesForSummarization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []anthropic.MessageParam
		contains []string
	}{
		{
			name:     "empty messages",
			messages: []anthropic.MessageParam{},
			contains: []string{},
		},
		{
			name: "user text message",
			messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock("Hello, world!")),
			},
			contains: []string{"[USER]", "Hello, world!"},
		},
		{
			name: "assistant text message",
			messages: []anthropic.MessageParam{
				anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi there!")),
			},
			contains: []string{"[ASSISTANT]", "Hi there!"},
		},
		{
			name: "multiple messages",
			messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock("Question one")),
				anthropic.NewAssistantMessage(anthropic.NewTextBlock("Answer one")),
				anthropic.NewUserMessage(anthropic.NewTextBlock("Question two")),
			},
			contains: []string{"[USER]", "[ASSISTANT]", "Question one", "Answer one", "Question two"},
		},
		{
			name: "tool use block",
			messages: []anthropic.MessageParam{
				anthropic.NewAssistantMessage(
					anthropic.NewToolUseBlock("tool-123", map[string]string{"cmd": "ls -la"}, "bash"),
				),
			},
			contains: []string{"[ASSISTANT]", "[Tool: bash]", "Input:"},
		},
		{
			name: "tool result block",
			messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(
					anthropic.NewToolResultBlock("tool-123", "command output here", false),
				),
			},
			contains: []string{"[USER]", "[Tool Result]", "command output here"},
		},
		{
			name: "tool result with error",
			messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(
					anthropic.NewToolResultBlock("tool-123", "error message", true),
				),
			},
			contains: []string{"[Tool Result (ERROR)]", "error message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := formatMessagesForSummarization(tt.messages)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("result should contain %q, got: %s", s, result)
				}
			}
		})
	}
}

func TestFormatContentBlock(t *testing.T) {
	t.Parallel()

	t.Run("text block", func(t *testing.T) {
		t.Parallel()

		var sb strings.Builder
		block := anthropic.NewTextBlock("Test content")
		formatContentBlock(&sb, block)

		result := sb.String()
		if !strings.Contains(result, "Test content") {
			t.Errorf("should contain text content, got: %s", result)
		}
	})

	t.Run("tool use block", func(t *testing.T) {
		t.Parallel()

		var sb strings.Builder
		block := anthropic.NewToolUseBlock("id", map[string]string{"key": "value"}, "read")
		formatContentBlock(&sb, block)

		result := sb.String()
		if !strings.Contains(result, "[Tool: read]") {
			t.Errorf("should contain tool name, got: %s", result)
		}
		if !strings.Contains(result, "Input:") {
			t.Errorf("should contain input label, got: %s", result)
		}
	})

	t.Run("tool use block with long input truncated", func(t *testing.T) {
		t.Parallel()

		var sb strings.Builder
		longInput := map[string]string{"data": strings.Repeat("x", 300)}
		block := anthropic.NewToolUseBlock("id", longInput, "write")
		formatContentBlock(&sb, block)

		result := sb.String()
		// Long inputs should be truncated at 200 chars
		if !strings.Contains(result, "...") {
			t.Errorf("long input should be truncated with '...', got: %s", result)
		}
	})

	t.Run("tool result block", func(t *testing.T) {
		t.Parallel()

		var sb strings.Builder
		block := anthropic.NewToolResultBlock("id", "Result text", false)
		formatContentBlock(&sb, block)

		result := sb.String()
		if !strings.Contains(result, "[Tool Result]") {
			t.Errorf("should contain tool result header, got: %s", result)
		}
		if !strings.Contains(result, "Result text") {
			t.Errorf("should contain result text, got: %s", result)
		}
	})
}

func TestFormatToolResult(t *testing.T) {
	t.Parallel()

	t.Run("without error", func(t *testing.T) {
		t.Parallel()

		var sb strings.Builder
		tr := anthropic.NewToolResultBlock("id", "Success output", false)
		formatToolResult(&sb, tr.OfToolResult)

		result := sb.String()
		if !strings.Contains(result, "[Tool Result]") {
			t.Errorf("should have tool result header, got: %s", result)
		}
		if strings.Contains(result, "(ERROR)") {
			t.Errorf("should not have ERROR flag, got: %s", result)
		}
		if !strings.Contains(result, "Success output") {
			t.Errorf("should contain output text, got: %s", result)
		}
	})

	t.Run("with error flag", func(t *testing.T) {
		t.Parallel()

		var sb strings.Builder
		tr := anthropic.NewToolResultBlock("id", "Error message", true)
		formatToolResult(&sb, tr.OfToolResult)

		result := sb.String()
		if !strings.Contains(result, "[Tool Result (ERROR)]") {
			t.Errorf("should have ERROR flag, got: %s", result)
		}
		if !strings.Contains(result, "Error message") {
			t.Errorf("should contain error message, got: %s", result)
		}
	})

	t.Run("truncates long output", func(t *testing.T) {
		t.Parallel()

		var sb strings.Builder
		longOutput := strings.Repeat("x", 600)
		tr := anthropic.NewToolResultBlock("id", longOutput, false)
		formatToolResult(&sb, tr.OfToolResult)

		result := sb.String()
		// Output should be truncated at 500 chars
		if !strings.Contains(result, "... [truncated]") {
			t.Errorf("long output should be truncated, got: %s", result)
		}
		// Should not contain the full output
		if strings.Contains(result, strings.Repeat("x", 600)) {
			t.Errorf("should not contain full output")
		}
	})
}

func TestNewHaikuSummarizer(t *testing.T) {
	t.Parallel()

	// We can't easily test with a real client, but we can verify creation doesn't panic
	// and the struct is properly initialized
	var client anthropic.Client
	summarizer := NewHaikuSummarizer(client)

	if summarizer == nil {
		t.Fatal("NewHaikuSummarizer returned nil")
	}
}

func TestHaikuSummarizer_Summarize_EmptyMessages(t *testing.T) {
	t.Parallel()

	// Create summarizer with zero client (will fail on actual API call,
	// but empty messages should return early)
	var client anthropic.Client
	summarizer := NewHaikuSummarizer(client)

	result, err := summarizer.Summarize(t.Context(), []anthropic.MessageParam{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for empty messages, got: %q", result)
	}
}

func TestFormatMessagesOrder(t *testing.T) {
	t.Parallel()

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("First")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Second")),
		anthropic.NewUserMessage(anthropic.NewTextBlock("Third")),
	}

	result := formatMessagesForSummarization(messages)

	firstIdx := strings.Index(result, "First")
	secondIdx := strings.Index(result, "Second")
	thirdIdx := strings.Index(result, "Third")

	if firstIdx == -1 || secondIdx == -1 || thirdIdx == -1 {
		t.Fatal("not all messages found in output")
	}

	if firstIdx >= secondIdx || secondIdx >= thirdIdx {
		t.Error("messages should appear in order")
	}
}

func TestFormatContentBlock_MultipleContentInMessage(t *testing.T) {
	t.Parallel()

	// Test message with multiple content blocks
	messages := []anthropic.MessageParam{
		anthropic.NewAssistantMessage(
			anthropic.NewTextBlock("Some text"),
			anthropic.NewToolUseBlock("tool-1", map[string]string{"a": "b"}, "bash"),
		),
	}

	result := formatMessagesForSummarization(messages)

	if !strings.Contains(result, "Some text") {
		t.Error("should contain text block content")
	}
	if !strings.Contains(result, "[Tool: bash]") {
		t.Error("should contain tool use block")
	}
}

func TestHaikuModelConstant(t *testing.T) {
	t.Parallel()

	// Verify the model constant is set correctly
	if HaikuModel != anthropic.ModelClaudeHaiku4_5 {
		t.Errorf("HaikuModel should be ClaudeHaiku4_5, got: %v", HaikuModel)
	}
}
