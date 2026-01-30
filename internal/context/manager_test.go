package context

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/zhubert/milo/internal/token"
)

// mockSummarizer is a test implementation of Summarizer.
type mockSummarizer struct {
	summary string
	err     error
	called  bool
}

func (m *mockSummarizer) Summarize(_ context.Context, _ []anthropic.MessageParam) (string, error) {
	m.called = true
	return m.summary, m.err
}

func TestNewManager(t *testing.T) {
	t.Parallel()

	limits := token.DefaultLimits()
	summarizer := &mockSummarizer{}

	mgr := NewManager(limits, summarizer)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.limits != limits {
		t.Error("limits not set correctly")
	}
	if mgr.summarizer != summarizer {
		t.Error("summarizer not set correctly")
	}
}

func TestNewManagerWithDefaults(t *testing.T) {
	t.Parallel()

	summarizer := &mockSummarizer{}
	mgr := NewManagerWithDefaults(summarizer)

	if mgr == nil {
		t.Fatal("NewManagerWithDefaults returned nil")
	}
	if mgr.limits.MaxContextTokens != 200000 {
		t.Error("default limits not applied")
	}
}

func TestNeedsCompaction(t *testing.T) {
	t.Parallel()

	// Use small limits for testing
	limits := token.ContextLimits{
		MaxContextTokens:       1000,
		ReservedOutputTokens:   100,
		ReservedSystemTokens:   100,
		SummarizationThreshold: 0.8,
	}
	mgr := NewManager(limits, nil)

	// Available: 1000 - 100 - 100 = 800
	// Trigger: 800 * 0.8 = 640

	tests := []struct {
		name     string
		messages []anthropic.MessageParam
		expected bool
	}{
		{
			name:     "empty messages",
			messages: nil,
			expected: false,
		},
		{
			name: "small message",
			messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
			},
			expected: false,
		},
		{
			name: "large message exceeds threshold",
			messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(strings.Repeat("x", 3000))),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := mgr.NeedsCompaction(tt.messages)
			if result != tt.expected {
				t.Errorf("NeedsCompaction() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCompact_NoCompactionNeeded(t *testing.T) {
	t.Parallel()

	mgr := NewManagerWithDefaults(nil)
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
	}

	result, err := mgr.Compact(context.Background(), messages)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	if result.SummaryAdded {
		t.Error("expected no summary to be added")
	}
	if len(result.Messages) != len(messages) {
		t.Errorf("expected %d messages, got %d", len(messages), len(result.Messages))
	}
}

func TestCompact_TruncatesToolResults(t *testing.T) {
	t.Parallel()

	// Use small limits to trigger compaction
	limits := token.ContextLimits{
		MaxContextTokens:       500,
		ReservedOutputTokens:   50,
		ReservedSystemTokens:   50,
		SummarizationThreshold: 0.5,
	}
	mgr := NewManager(limits, nil)

	// Create messages with large tool results
	longOutput := strings.Repeat("x", 1000)
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Do something")),
		anthropic.NewAssistantMessage(anthropic.NewToolUseBlock("tool-1", map[string]string{"cmd": "ls"}, "bash")),
		anthropic.NewUserMessage(anthropic.NewToolResultBlock("tool-1", longOutput, false)),
	}

	result, err := mgr.Compact(context.Background(), messages)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	if result.CompactedTokens >= result.OriginalTokens {
		t.Errorf("expected compacted tokens (%d) < original tokens (%d)",
			result.CompactedTokens, result.OriginalTokens)
	}
}

func TestCompact_WithSummarizer(t *testing.T) {
	t.Parallel()

	limits := token.ContextLimits{
		MaxContextTokens:       200,
		ReservedOutputTokens:   20,
		ReservedSystemTokens:   20,
		SummarizationThreshold: 0.3,
	}

	summarizer := &mockSummarizer{
		summary: "User asked for help. Assistant provided assistance.",
	}
	mgr := NewManager(limits, summarizer)

	// Create enough messages to trigger summarization
	messages := make([]anthropic.MessageParam, 20)
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			messages[i] = anthropic.NewUserMessage(anthropic.NewTextBlock("Question " + strings.Repeat("x", 50)))
		} else {
			messages[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock("Answer " + strings.Repeat("y", 50)))
		}
	}

	result, err := mgr.Compact(context.Background(), messages)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	if !summarizer.called {
		t.Error("expected summarizer to be called")
	}
	if !result.SummaryAdded {
		t.Error("expected summary to be added")
	}
	if len(result.Messages) >= len(messages) {
		t.Errorf("expected fewer messages after compaction, got %d (was %d)",
			len(result.Messages), len(messages))
	}
}

func TestTokenCount(t *testing.T) {
	t.Parallel()

	mgr := NewManagerWithDefaults(nil)
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi there!")),
	}

	count := mgr.TokenCount(messages)
	if count < 5 {
		t.Errorf("TokenCount() = %d, expected at least 5", count)
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()

	limits := token.ContextLimits{
		MaxContextTokens:       100000,
		ReservedOutputTokens:   4096,
		ReservedSystemTokens:   10000,
		SummarizationThreshold: 0.75,
	}
	mgr := NewManager(limits, nil)

	result := mgr.Limits()
	if result != limits {
		t.Error("Limits() did not return expected limits")
	}
}

func TestFormatSummary(t *testing.T) {
	t.Parallel()

	summary := "This is a test summary."
	result := formatSummary(summary)

	if !strings.Contains(result, "CONVERSATION SUMMARY") {
		t.Error("expected summary header")
	}
	if !strings.Contains(result, summary) {
		t.Error("expected summary content")
	}
	if !strings.Contains(result, "END SUMMARY") {
		t.Error("expected summary footer")
	}
}
