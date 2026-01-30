// Package session provides persistence for agent conversations.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// Session represents a saved conversation session.
type Session struct {
	ID        string                   `json:"id"`
	Title     string                   `json:"title"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
	Messages  []anthropic.MessageParam `json:"messages"`
}

// NewSession creates a new session with a generated ID.
func NewSession() (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Session{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  nil,
	}, nil
}

// generateID creates a random 8-character hex ID.
func generateID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetTitle sets the session title.
func (s *Session) SetTitle(title string) {
	s.Title = title
	s.UpdatedAt = time.Now()
}

// SetMessages replaces the session's messages.
func (s *Session) SetMessages(messages []anthropic.MessageParam) {
	s.Messages = messages
	s.UpdatedAt = time.Now()
}

// MessageCount returns the number of messages in the session.
func (s *Session) MessageCount() int {
	return len(s.Messages)
}

// Summary contains metadata about a session for listing purposes.
type Summary struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

// Summary returns a Summary of the session without the full messages.
func (s *Session) Summary() Summary {
	return Summary{
		ID:           s.ID,
		Title:        s.Title,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		MessageCount: len(s.Messages),
	}
}
