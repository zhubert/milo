package agent

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/zhubert/milo/internal/token"
)

// Conversation manages the message history for an agent session.
type Conversation struct {
	messages   []anthropic.MessageParam
	tokenCount int
}

// NewConversation creates a new empty conversation.
func NewConversation() *Conversation {
	return &Conversation{}
}

// TokenCount returns the estimated token count of the conversation.
func (c *Conversation) TokenCount() int {
	return c.tokenCount
}

// updateTokenCount recalculates the token count for the conversation.
func (c *Conversation) updateTokenCount() {
	c.tokenCount = token.CountMessages(c.messages)
}

// SetMessages replaces all messages in the conversation.
// This is used to restore a conversation from a saved session or after compaction.
func (c *Conversation) SetMessages(messages []anthropic.MessageParam) {
	c.messages = make([]anthropic.MessageParam, len(messages))
	copy(c.messages, messages)
	c.updateTokenCount()
}

// AddUserMessage appends a user text message to the conversation.
func (c *Conversation) AddUserMessage(text string) {
	msg := anthropic.NewUserMessage(anthropic.NewTextBlock(text))
	c.messages = append(c.messages, msg)
	c.tokenCount += token.CountMessage(msg)
}

// AddAssistantMessage appends an assistant message with the given content blocks.
func (c *Conversation) AddAssistantMessage(blocks ...anthropic.ContentBlockParamUnion) {
	msg := anthropic.NewAssistantMessage(blocks...)
	c.messages = append(c.messages, msg)
	c.tokenCount += token.CountMessage(msg)
}

// AddToolResult appends a user message containing tool result blocks.
// Each tool result is a separate content block within a single user message.
func (c *Conversation) AddToolResult(results ...anthropic.ContentBlockParamUnion) {
	msg := anthropic.NewUserMessage(results...)
	c.messages = append(c.messages, msg)
	c.tokenCount += token.CountMessage(msg)
}

// Messages returns the conversation history as API-ready params.
func (c *Conversation) Messages() []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, len(c.messages))
	copy(out, c.messages)
	return out
}

// Len returns the number of messages in the conversation.
func (c *Conversation) Len() int {
	return len(c.messages)
}
