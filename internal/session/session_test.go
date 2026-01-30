package session

import (
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestNewSession(t *testing.T) {
	t.Parallel()

	sess, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	if sess.ID == "" {
		t.Error("expected non-empty ID")
	}
	if len(sess.ID) != 8 {
		t.Errorf("expected ID length 8, got %d", len(sess.ID))
	}
	if sess.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if sess.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if sess.Messages != nil {
		t.Error("expected nil Messages")
	}
}

func TestSession_SetTitle(t *testing.T) {
	t.Parallel()

	sess, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	originalUpdatedAt := sess.UpdatedAt
	time.Sleep(time.Millisecond) // Ensure time difference

	sess.SetTitle("Test Title")

	if sess.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %q", sess.Title)
	}
	if !sess.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestSession_SetMessages(t *testing.T) {
	t.Parallel()

	sess, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi there")),
	}

	originalUpdatedAt := sess.UpdatedAt
	time.Sleep(time.Millisecond)

	sess.SetMessages(messages)

	if sess.MessageCount() != 2 {
		t.Errorf("expected 2 messages, got %d", sess.MessageCount())
	}
	if !sess.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestSession_Summary(t *testing.T) {
	t.Parallel()

	sess, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	sess.SetTitle("My Session")
	sess.SetMessages([]anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("test")),
	})

	summary := sess.Summary()

	if summary.ID != sess.ID {
		t.Errorf("summary ID mismatch: got %s, want %s", summary.ID, sess.ID)
	}
	if summary.Title != "My Session" {
		t.Errorf("summary Title mismatch: got %s, want %s", summary.Title, "My Session")
	}
	if summary.MessageCount != 1 {
		t.Errorf("summary MessageCount mismatch: got %d, want %d", summary.MessageCount, 1)
	}
}

func TestGenerateID_Unique(t *testing.T) {
	t.Parallel()

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generateID()
		if err != nil {
			t.Fatalf("generateID() error: %v", err)
		}
		if ids[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}
