package lsp

import (
	"context"
	"testing"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	manager := NewManager(registry)

	if manager.Registry() != registry {
		t.Error("manager should return the registry it was created with")
	}

	if manager.ActiveClients() != 0 {
		t.Errorf("new manager should have 0 active clients, got %d", manager.ActiveClients())
	}
}

func TestManagerCloseAll(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	manager := NewManager(registry)

	// CloseAll on empty manager should not error
	if err := manager.CloseAll(); err != nil {
		t.Errorf("CloseAll on empty manager failed: %v", err)
	}
}

func TestManagerGetClientNoServer(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	ctx := context.Background()
	registry.DetectAvailable(ctx)

	manager := NewManager(registry)

	// Try to get client for unsupported file type
	_, err := manager.GetClient(ctx, "/path/to/file.xyz")
	if err == nil {
		t.Error("expected error for unsupported file type")
	}

	if _, ok := err.(*NoServerError); !ok {
		t.Errorf("expected NoServerError, got %T: %v", err, err)
	}
}

func TestManagerGetClientServerNotInstalled(t *testing.T) {
	t.Parallel()

	// Create a registry with a fake server that's not installed
	registry := &Registry{
		servers: []*ServerConfig{
			{
				Name:        "fake-server",
				Command:     []string{"nonexistent-binary-xyz123"},
				Languages:   []string{"fake"},
				Extensions:  []string{".fake"},
				RootFiles:   []string{"fake.config"},
				InstallHint: "this server does not exist",
			},
		},
		ready: make(chan struct{}),
	}

	ctx := context.Background()
	registry.DetectAvailable(ctx)

	manager := NewManager(registry)

	// Try to get client for .fake file
	_, err := manager.GetClient(ctx, "/path/to/file.fake")
	if err == nil {
		t.Error("expected error for non-installed server")
	}

	if _, ok := err.(*ServerNotInstalledError); !ok {
		t.Errorf("expected ServerNotInstalledError, got %T: %v", err, err)
	}
}
