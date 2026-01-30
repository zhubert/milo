package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestStore_SaveAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	sess, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	sess.SetTitle("Test Session")
	sess.SetMessages([]anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi there")),
	})

	// Save the session.
	if err := store.Save(sess); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(dir, sess.ID+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("session file was not created")
	}

	// Load the session.
	loaded, err := store.Load(sess.ID)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.ID != sess.ID {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID, sess.ID)
	}
	if loaded.Title != sess.Title {
		t.Errorf("Title mismatch: got %s, want %s", loaded.Title, sess.Title)
	}
	if loaded.MessageCount() != sess.MessageCount() {
		t.Errorf("MessageCount mismatch: got %d, want %d", loaded.MessageCount(), sess.MessageCount())
	}
}

func TestStore_Load_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	_, err = store.Load("nonexistent")
	if err == nil {
		t.Error("expected error loading nonexistent session")
	}
}

func TestStore_Delete(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	sess, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if err := store.Delete(sess.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify file is gone.
	path := filepath.Join(dir, sess.ID+".json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("session file was not deleted")
	}

	// Delete again should not error.
	if err := store.Delete(sess.ID); err != nil {
		t.Errorf("Delete() of nonexistent session should not error: %v", err)
	}
}

func TestStore_List(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	// Create multiple sessions.
	sess1, _ := NewSession()
	sess1.SetTitle("Session 1")
	sess2, _ := NewSession()
	sess2.SetTitle("Session 2")

	if err := store.Save(sess1); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if err := store.Save(sess2); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	summaries, err := store.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(summaries) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(summaries))
	}
}


func TestStore_MostRecent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	// Create sessions.
	sess1, _ := NewSession()
	sess1.SetTitle("First")
	if err := store.Save(sess1); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	sess2, _ := NewSession()
	sess2.SetTitle("Second")
	if err := store.Save(sess2); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Most recent should be sess2 (created later).
	recent, err := store.MostRecent()
	if err != nil {
		t.Fatalf("MostRecent() error: %v", err)
	}

	if recent.ID != sess2.ID {
		t.Errorf("expected most recent to be sess2 (%s), got %s", sess2.ID, recent.ID)
	}
}

func TestStore_MostRecent_NoSessions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	recent, err := store.MostRecent()
	if err != nil {
		t.Fatalf("MostRecent() error: %v", err)
	}

	if recent != nil {
		t.Error("expected nil when no sessions exist")
	}
}

func TestStore_MessageSerialization(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	sess, _ := NewSession()
	sess.SetMessages([]anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("What is 2+2?")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("The answer is 4.")),
		anthropic.NewUserMessage(anthropic.NewTextBlock("Thanks!")),
	})

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := store.Load(sess.ID)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(loaded.Messages))
	}

	// Verify message content survived serialization.
	if string(loaded.Messages[0].Role) != "user" {
		t.Errorf("expected first message role 'user', got %s", loaded.Messages[0].Role)
	}
	if string(loaded.Messages[1].Role) != "assistant" {
		t.Errorf("expected second message role 'assistant', got %s", loaded.Messages[1].Role)
	}
}
