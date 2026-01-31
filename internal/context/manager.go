package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/zhubert/milo/internal/token"
)

// Summarizer generates summaries of conversation segments using an LLM.
type Summarizer interface {
	Summarize(ctx context.Context, messages []anthropic.MessageParam) (string, error)
}

// Manager handles context window management for conversations.
type Manager struct {
	limits     token.ContextLimits
	summarizer Summarizer
}

// NewManager creates a new context manager with the given limits and summarizer.
func NewManager(limits token.ContextLimits, summarizer Summarizer) *Manager {
	return &Manager{
		limits:     limits,
		summarizer: summarizer,
	}
}

// NewManagerWithDefaults creates a context manager with default limits.
func NewManagerWithDefaults(summarizer Summarizer) *Manager {
	return NewManager(token.DefaultLimits(), summarizer)
}

// CompactionResult contains the result of a compaction operation.
type CompactionResult struct {
	// Messages is the compacted message list.
	Messages []anthropic.MessageParam
	// OriginalTokens is the token count before compaction.
	OriginalTokens int
	// CompactedTokens is the token count after compaction.
	CompactedTokens int
	// SummaryAdded indicates if a summary message was added.
	SummaryAdded bool
}

// NeedsCompaction checks if the messages exceed the summarization threshold.
func (m *Manager) NeedsCompaction(messages []anthropic.MessageParam) bool {
	tokenCount := token.CountMessages(messages)
	return tokenCount >= m.limits.SummarizationTrigger()
}

// Compact reduces the context size using the smart hybrid strategy:
// 1. Aggressively truncate tool results (keep only last N)
// 2. Summarize older conversation turns with Haiku
// 3. Preserve recent messages intact
func (m *Manager) Compact(ctx context.Context, messages []anthropic.MessageParam) (*CompactionResult, error) {
	originalTokens := token.CountMessages(messages)

	if originalTokens < m.limits.SummarizationTrigger() {
		return &CompactionResult{
			Messages:        messages,
			OriginalTokens:  originalTokens,
			CompactedTokens: originalTokens,
			SummaryAdded:    false,
		}, nil
	}

	// Step 1: Truncate tool results aggressively
	truncated := m.truncateToolResults(messages)
	truncatedTokens := token.CountMessages(truncated)

	// If truncation alone is enough, return early
	if truncatedTokens < m.limits.SummarizationTrigger() {
		return &CompactionResult{
			Messages:        truncated,
			OriginalTokens:  originalTokens,
			CompactedTokens: truncatedTokens,
			SummaryAdded:    false,
		}, nil
	}

	// Step 2: Summarize older messages
	compacted, err := m.summarizeOldMessages(ctx, truncated)
	if err != nil {
		// If summarization fails, fall back to simple truncation
		compacted = m.simpleTruncate(truncated)
	}

	compactedTokens := token.CountMessages(compacted)

	return &CompactionResult{
		Messages:        compacted,
		OriginalTokens:  originalTokens,
		CompactedTokens: compactedTokens,
		SummaryAdded:    err == nil,
	}, nil
}

// truncateToolResults replaces verbose tool outputs with shortened versions.
func (m *Manager) truncateToolResults(messages []anthropic.MessageParam) []anthropic.MessageParam {
	result := make([]anthropic.MessageParam, 0, len(messages))

	// Keep last N messages with full tool results
	const preserveRecent = 6

	for i, msg := range messages {
		isRecent := i >= len(messages)-preserveRecent

		if isRecent {
			result = append(result, msg)
			continue
		}

		// For older messages, truncate tool results
		truncatedMsg := m.truncateMessageToolResults(msg)
		result = append(result, truncatedMsg)
	}

	return result
}

// truncateMessageToolResults truncates tool results in a single message.
func (m *Manager) truncateMessageToolResults(msg anthropic.MessageParam) anthropic.MessageParam {
	newContent := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Content))

	for _, block := range msg.Content {
		if block.OfToolResult != nil {
			// Truncate the tool result content
			truncated := m.truncateToolResultContent(block.OfToolResult)
			newContent = append(newContent, anthropic.ContentBlockParamUnion{OfToolResult: truncated})
		} else {
			newContent = append(newContent, block)
		}
	}

	return anthropic.MessageParam{
		Role:    msg.Role,
		Content: newContent,
	}
}

// truncateToolResultContent shortens a tool result's content.
func (m *Manager) truncateToolResultContent(tr *anthropic.ToolResultBlockParam) *anthropic.ToolResultBlockParam {
	const maxChars = 500
	const truncationMsg = "\n[... output truncated for context management ...]"

	newContent := make([]anthropic.ToolResultBlockParamContentUnion, 0, len(tr.Content))

	for _, c := range tr.Content {
		if c.OfText != nil {
			text := c.OfText.Text
			if len(text) > maxChars {
				text = text[:maxChars] + truncationMsg
			}
			newContent = append(newContent, anthropic.ToolResultBlockParamContentUnion{
				OfText: &anthropic.TextBlockParam{
					Text: text,
				},
			})
		} else {
			newContent = append(newContent, c)
		}
	}

	return &anthropic.ToolResultBlockParam{
		ToolUseID: tr.ToolUseID,
		Content:   newContent,
		IsError:   tr.IsError,
	}
}

// summarizeOldMessages uses the summarizer to condense older conversation turns.
func (m *Manager) summarizeOldMessages(ctx context.Context, messages []anthropic.MessageParam) ([]anthropic.MessageParam, error) {
	if m.summarizer == nil {
		return m.simpleTruncate(messages), nil
	}

	// Keep recent messages intact (last 8 messages or so)
	const preserveRecent = 8
	if len(messages) <= preserveRecent {
		return messages, nil
	}

	oldMessages := messages[:len(messages)-preserveRecent]
	recentMessages := messages[len(messages)-preserveRecent:]

	// Summarize the old messages
	summary, err := m.summarizer.Summarize(ctx, oldMessages)
	if err != nil {
		return nil, fmt.Errorf("summarizing old messages: %w", err)
	}

	// Create a new message list with the summary at the start
	result := make([]anthropic.MessageParam, 0, len(recentMessages)+1)

	// Add summary as a user message (Claude expects alternating user/assistant)
	summaryMsg := anthropic.NewUserMessage(
		anthropic.NewTextBlock(formatSummary(summary)),
	)
	result = append(result, summaryMsg)

	// Add recent messages
	result = append(result, recentMessages...)

	return result, nil
}

// simpleTruncate removes the oldest messages when summarization isn't available.
func (m *Manager) simpleTruncate(messages []anthropic.MessageParam) []anthropic.MessageParam {
	targetTokens := m.limits.SummarizationTrigger() / 3 // Target 50% of trigger threshold

	// Remove messages from the front until under budget
	for len(messages) > 2 && token.CountMessages(messages) > targetTokens {
		// Always keep at least the first user message for context
		// Remove from position 1 (second oldest)
		if len(messages) > 2 {
			messages = append(messages[:1], messages[2:]...)
		} else {
			break
		}
	}

	return messages
}

// formatSummary wraps the summary text in a clear format.
func formatSummary(summary string) string {
	var sb strings.Builder
	sb.WriteString("[CONVERSATION SUMMARY - Earlier messages have been condensed]\n\n")
	sb.WriteString(summary)
	sb.WriteString("\n\n[END SUMMARY - Recent conversation continues below]")
	return sb.String()
}

// TokenCount returns the current token count for the given messages.
func (m *Manager) TokenCount(messages []anthropic.MessageParam) int {
	return token.CountMessages(messages)
}

// Limits returns the context limits configuration.
func (m *Manager) Limits() token.ContextLimits {
	return m.limits
}
