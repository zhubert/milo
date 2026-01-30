package agent

import (
	"github.com/anthropics/anthropic-sdk-go"
)

// Conversation manages the message history for an agent session.
type Conversation struct {
	messages []anthropic.MessageParam
}

// NewConversation creates a new empty conversation.
func NewConversation() *Conversation {
	return &Conversation{}
}

// AddUserMessage appends a user text message to the conversation.
func (c *Conversation) AddUserMessage(text string) {
	c.messages = append(c.messages, anthropic.NewUserMessage(
		anthropic.NewTextBlock(text),
	))
}

// AddAssistantMessage appends an assistant message with the given content blocks.
func (c *Conversation) AddAssistantMessage(blocks ...anthropic.ContentBlockParamUnion) {
	c.messages = append(c.messages, anthropic.NewAssistantMessage(blocks...))
}

// AddToolResult appends a user message containing tool result blocks.
// Each tool result is a separate content block within a single user message.
func (c *Conversation) AddToolResult(results ...anthropic.ContentBlockParamUnion) {
	c.messages = append(c.messages, anthropic.NewUserMessage(results...))
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

// SetMessages replaces all messages in the conversation.
// This is used to restore a conversation from a saved session.
func (c *Conversation) SetMessages(messages []anthropic.MessageParam) {
	c.messages = make([]anthropic.MessageParam, len(messages))
	copy(c.messages, messages)
}
