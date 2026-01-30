package token

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "single character",
			input:    "a",
			expected: 1,
		},
		{
			name:     "four characters",
			input:    "test",
			expected: 1,
		},
		{
			name:     "five characters",
			input:    "hello",
			expected: 2,
		},
		{
			name:     "eight characters",
			input:    "12345678",
			expected: 2,
		},
		{
			name:     "longer text",
			input:    "This is a longer piece of text that should have more tokens.",
			expected: 15, // 60 chars / 4 = 15 tokens
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := Count(tt.input)
			if result != tt.expected {
				t.Errorf("Count(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCountMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  anthropic.MessageParam
		min  int // Minimum expected tokens (exact count varies)
	}{
		{
			name: "simple user message",
			msg:  anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
			min:  3, // 2 for role + at least 1 for text
		},
		{
			name: "assistant message with longer text",
			msg:  anthropic.NewAssistantMessage(anthropic.NewTextBlock("This is a response from the assistant.")),
			min:  10,
		},
		{
			name: "message with multiple blocks",
			msg: anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("First block"),
				anthropic.NewTextBlock("Second block"),
			),
			min: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := CountMessage(tt.msg)
			if result < tt.min {
				t.Errorf("CountMessage() = %d, want at least %d", result, tt.min)
			}
		})
	}
}

func TestCountMessages(t *testing.T) {
	t.Parallel()

	msgs := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi there!")),
		anthropic.NewUserMessage(anthropic.NewTextBlock("How are you?")),
	}

	result := CountMessages(msgs)
	if result < 10 {
		t.Errorf("CountMessages() = %d, want at least 10", result)
	}
}

func TestContextLimits(t *testing.T) {
	t.Parallel()

	t.Run("default limits", func(t *testing.T) {
		t.Parallel()
		limits := DefaultLimits()

		if limits.MaxContextTokens != 200000 {
			t.Errorf("MaxContextTokens = %d, want 200000", limits.MaxContextTokens)
		}
		if limits.ReservedOutputTokens != 8192 {
			t.Errorf("ReservedOutputTokens = %d, want 8192", limits.ReservedOutputTokens)
		}
		if limits.ReservedSystemTokens != 20000 {
			t.Errorf("ReservedSystemTokens = %d, want 20000", limits.ReservedSystemTokens)
		}
		if limits.SummarizationThreshold != 0.8 {
			t.Errorf("SummarizationThreshold = %f, want 0.8", limits.SummarizationThreshold)
		}
	})

	t.Run("available tokens", func(t *testing.T) {
		t.Parallel()
		limits := DefaultLimits()
		expected := 200000 - 8192 - 20000

		if limits.AvailableTokens() != expected {
			t.Errorf("AvailableTokens() = %d, want %d", limits.AvailableTokens(), expected)
		}
	})

	t.Run("summarization trigger", func(t *testing.T) {
		t.Parallel()
		limits := DefaultLimits()
		available := limits.AvailableTokens()
		expected := int(float64(available) * 0.8)

		if limits.SummarizationTrigger() != expected {
			t.Errorf("SummarizationTrigger() = %d, want %d", limits.SummarizationTrigger(), expected)
		}
	})
}
