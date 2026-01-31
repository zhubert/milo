package token

import (
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
)

// Approximate tokens per character for Claude models.
// Claude uses a BPE tokenizer where ~4 characters = 1 token for English text.
// This is a conservative estimate; actual token counts may vary.
var charsPerToken = 4

// Count estimates the number of tokens in a string.
func Count(s string) int {
	if len(s) == 0 {
		return 0
	}
	return (len(s) + charsPerToken - 1) / charsPerToken
}

// CountMessage estimates the token count for a single message.
func CountMessage(msg anthropic.MessageParam) int {
	total := 0

	// Count role token overhead (small, but present)
	total += 2

	for _, block := range msg.Content {
		total += countContentBlock(block)
	}

	return total
}

// CountMessages estimates the total token count for a slice of messages.
func CountMessages(msgs []anthropic.MessageParam) int {
	total := 0
	for _, msg := range msgs {
		total += CountMessage(msg)
	}
	return total
}

// countContentBlock estimates tokens for a single content block.
func countContentBlock(block anthropic.ContentBlockParamUnion) int {
	switch {
	case block.OfText != nil:
		return Count(block.OfText.Text)
	case block.OfToolUse != nil:
		// Tool name + input JSON
		inputBytes, err := json.Marshal(block.OfToolUse.Input)
		if err != nil {
			return Count(block.OfToolUse.Name) + 50 // Fallback estimate
		}
		return Count(block.OfToolUse.Name) + Count(string(inputBytes))
	case block.OfToolResult != nil:
		return countToolResult(block.OfToolResult)
	default:
		// Unknown block type, estimate conservatively
		return 50
	}
}

// countToolResult estimates tokens for a tool result block.
func countToolResult(tr *anthropic.ToolResultBlockParam) int {
	total := 10 // Overhead for tool_use_id and structure

	for _, c := range tr.Content {
		if c.OfText != nil {
			total += Count(c.OfText.Text)
		} else {
			total += 50 // Conservative estimate for other types
		}
	}

	return total
}

// ContextLimits defines the token budgets for context management.
type ContextLimits struct {
	// MaxContextTokens is the model's maximum context window.
	MaxContextTokens int
	// ReservedOutputTokens is space reserved for the model's response.
	ReservedOutputTokens int
	// ReservedSystemTokens is space reserved for system prompt and tools.
	ReservedSystemTokens int
	// SummarizationThreshold is when to trigger summarization (percentage of available).
	SummarizationThreshold float64
}

// DefaultLimits returns sensible defaults for Claude Sonnet.
func DefaultLimits() ContextLimits {
	return ContextLimits{
		MaxContextTokens:       200000,
		ReservedOutputTokens:   8192,
		ReservedSystemTokens:   20000,
		SummarizationThreshold: 0.8, // Trigger at 80% of available context
	}
}

// AvailableTokens returns the token budget for conversation history.
func (l ContextLimits) AvailableTokens() int {
	return l.MaxContextTokens - l.ReservedOutputTokens - l.ReservedSystemTokens
}

// SummarizationTrigger returns the token count that triggers summarization.
func (l ContextLimits) SummarizationTrigger() int {
	return int(float64(l.AvailableTokens()) * l.SummarizationThreshold)
}
