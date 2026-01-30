package lsp

import (
	"context"
	"fmt"
	"sync"
)

// Manager manages multiple language server clients.
// It lazily starts servers on demand and reuses existing connections.
type Manager struct {
	registry *Registry

	mu      sync.RWMutex
	clients map[string]*Client // key: "servername:/project/root"
}

// NewManager creates a new language server manager.
func NewManager(registry *Registry) *Manager {
	return &Manager{
		registry: registry,
		clients:  make(map[string]*Client),
	}
}

// GetClient returns a client for the given file path.
// It finds the appropriate server, locates the project root, and returns
// an existing or newly created client.
func (m *Manager) GetClient(ctx context.Context, filePath string) (*Client, error) {
	// Find the appropriate server for this file type
	config, err := m.registry.FindForFile(filePath)
	if err != nil {
		return nil, err
	}

	// Find the project root
	rootDir := FindProjectRoot(filePath, config.RootFiles)

	// Generate a key for this server+root combination
	key := config.Name + ":" + rootDir

	// Check for existing client
	m.mu.RLock()
	client, exists := m.clients[key]
	m.mu.RUnlock()

	if exists && client.IsRunning() {
		return client, nil
	}

	// Need to create a new client
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if client, exists = m.clients[key]; exists && client.IsRunning() {
		return client, nil
	}

	// Clean up old client if it exists but isn't running
	if client != nil {
		_ = client.Close()
		delete(m.clients, key)
	}

	// Create and start new client
	client = NewClient(config, rootDir)
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("starting %s for %s: %w", config.Name, rootDir, err)
	}

	m.clients[key] = client
	return client, nil
}

// CloseAll shuts down all managed language server clients.
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for key, client := range m.clients {
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("closing %s: %w", key, err)
		}
		delete(m.clients, key)
	}

	return firstErr
}

// Registry returns the server registry.
func (m *Manager) Registry() *Registry {
	return m.registry
}

// ActiveClients returns the number of active language server clients.
func (m *Manager) ActiveClients() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, client := range m.clients {
		if client.IsRunning() {
			count++
		}
	}
	return count
}
