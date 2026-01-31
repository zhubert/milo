package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	// HaikuModel is the model used for summarization.
	HaikuModel = anthropic.ModelClaudeHaiku4_5

	summarizationPrompt = `You are summarizing a conversation between a user and an AI coding assistant.
Your summary will replace the original messages to save context space.

Key requirements:
1. Preserve ALL important information: file paths, code changes, decisions made, errors encountered
2. Maintain the logical flow of what happened
3. Be concise but complete - don't lose critical details
4. Format as a clear narrative, not a list
5. Include specific technical details (function names, file paths, error messages)

Summarize the following conversation segment:`
)

// HaikuSummarizer uses Claude Haiku to summarize conversation segments.
type HaikuSummarizer struct {
	client anthropic.Client
}

// NewHaikuSummarizer creates a new summarizer using the provided client.
func NewHaikuSummarizer(client anthropic.Client) *HaikuSummarizer {
	return &HaikuSummarizer{client: client}
}

// Summarize generates a summary of the given messages using Haiku.
func (s *HaikuSummarizer) Summarize(ctx context.Context, messages []anthropic.MessageParam) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Convert messages to a readable format for summarization
	conversationText := formatMessagesForSummarization(messages)

	// Call Haiku to generate the summary
	resp, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     HaikuModel,
		MaxTokens: 2048,
		System: []anthropic.TextBlockParam{
			{Text: summarizationPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(conversationText),
			),
		},
	})
	if err != nil {
		return "", fmt.Errorf("calling Haiku for summarization: %w", err)
	}

	// Extract the text response
	var summary strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			summary.WriteString(block.Text)
		}
	}

	return summary.String(), nil
}

// formatMessagesForSummarization converts messages to a human-readable format.
func formatMessagesForSummarization(messages []anthropic.MessageParam) string {
	var sb strings.Builder

	for _, msg := range messages {
		role := string(msg.Role)
		sb.WriteString(fmt.Sprintf("[%s]\n", strings.ToUpper(role)))

		for _, block := range msg.Content {
			formatContentBlock(&sb, block)
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatContentBlock writes a content block to the string builder.
func formatContentBlock(sb *strings.Builder, block anthropic.ContentBlockParamUnion) {
	switch {
	case block.OfText != nil:
		sb.WriteString(block.OfText.Text)
		sb.WriteString("\n")
	case block.OfToolUse != nil:
		fmt.Fprintf(sb, "[Tool: %s]\n", block.OfToolUse.Name)
		// Include truncated input for context
		inputStr := fmt.Sprintf("%v", block.OfToolUse.Input)
		if len(inputStr) > 200 {
			inputStr = inputStr[:200] + "..."
		}
		fmt.Fprintf(sb, "Input: %s\n", inputStr)
	case block.OfToolResult != nil:
		formatToolResult(sb, block.OfToolResult)
	}
}

// formatToolResult writes a tool result to the string builder.
func formatToolResult(sb *strings.Builder, tr *anthropic.ToolResultBlockParam) {
	sb.WriteString("[Tool Result")
	if tr.IsError.Valid() && tr.IsError.Value {
		sb.WriteString(" (ERROR)")
	}
	sb.WriteString("]\n")

	for _, c := range tr.Content {
		if c.OfText != nil {
			text := c.OfText.Text
			// Truncate long outputs for the summarization input
			if len(text) > 500 {
				text = text[:500] + "... [truncated]"
			}
			sb.WriteString(text)
			sb.WriteString("\n")
		}
	}
}
