package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Store manages session persistence to the filesystem.
type Store struct {
	dir string
	mu  sync.RWMutex
}

// NewStore creates a new session store at the given directory.
// The directory will be created if it doesn't exist.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating session directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// StoreForWorkDir returns a Store using .milo/sessions in the given work directory.
func StoreForWorkDir(workDir string) (*Store, error) {
	return NewStore(filepath.Join(workDir, ".milo", "sessions"))
}

// Save persists a session to disk.
func (s *Store) Save(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	path := s.sessionPath(session.ID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}

	return nil
}

// Load retrieves a session by ID.
func (s *Store) Load(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %q not found", id)
		}
		return nil, fmt.Errorf("reading session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshaling session: %w", err)
	}

	return &session, nil
}

// Delete removes a session from disk.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.sessionPath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already gone, that's fine
		}
		return fmt.Errorf("deleting session file: %w", err)
	}

	return nil
}

// List returns summaries of all sessions, sorted by update time (newest first).
func (s *Store) List() ([]Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session directory: %w", err)
	}

	var summaries []Summary
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Strip .json
		session, err := s.loadUnlocked(id)
		if err != nil {
			continue // Skip corrupted sessions
		}

		summaries = append(summaries, session.Summary())
	}

	// Sort by UpdatedAt descending (most recent first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})

	return summaries, nil
}

// loadUnlocked loads a session without acquiring the lock.
// Caller must hold at least a read lock.
func (s *Store) loadUnlocked(id string) (*Session, error) {
	path := s.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// sessionPath returns the file path for a session ID.
func (s *Store) sessionPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// MostRecent returns the most recently updated session, or nil if no sessions exist.
func (s *Store) MostRecent() (*Session, error) {
	summaries, err := s.List()
	if err != nil {
		return nil, err
	}

	if len(summaries) == 0 {
		return nil, nil
	}

	// Already sorted by UpdatedAt descending
	return s.Load(summaries[0].ID)
}
