package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	servers := r.Servers()
	if len(servers) == 0 {
		t.Error("expected default servers, got none")
	}

	// Check that we have gopls
	var foundGopls bool
	for _, s := range servers {
		if s.Name == "gopls" {
			foundGopls = true
			break
		}
	}
	if !foundGopls {
		t.Error("expected gopls in default servers")
	}
}

func TestRegistryDetectAvailable(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run detection
	r.DetectAvailable(ctx)

	// Should be ready after detection completes
	if !r.IsReady() {
		t.Error("expected registry to be ready after detection")
	}

	// WaitReady should return immediately now
	if err := r.WaitReady(ctx); err != nil {
		t.Errorf("WaitReady failed: %v", err)
	}
}

func TestRegistryWaitReadyTimeout(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	// Don't run detection, so it should never be ready
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := r.WaitReady(ctx)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestFindForFile(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	// Run detection first
	ctx := context.Background()
	r.DetectAvailable(ctx)

	tests := []struct {
		name        string
		filePath    string
		wantServer  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "Go file",
			filePath:   "/path/to/file.go",
			wantServer: "gopls",
		},
		{
			name:       "TypeScript file",
			filePath:   "/path/to/file.ts",
			wantServer: "typescript-language-server",
		},
		{
			name:       "JavaScript file",
			filePath:   "/path/to/file.js",
			wantServer: "typescript-language-server",
		},
		{
			name:       "Rust file",
			filePath:   "/path/to/file.rs",
			wantServer: "rust-analyzer",
		},
		{
			name:       "Python file",
			filePath:   "/path/to/file.py",
			wantServer: "pyright",
		},
		{
			name:       "C file",
			filePath:   "/path/to/file.c",
			wantServer: "clangd",
		},
		{
			name:        "Unknown file type",
			filePath:    "/path/to/file.xyz",
			wantErr:     true,
			errContains: "no language server configured",
		},
		{
			name:        "No extension",
			filePath:    "/path/to/Makefile",
			wantErr:     true,
			errContains: "no language server configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, err := r.FindForFile(tc.filePath)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tc.errContains != "" && !contains(err.Error(), tc.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tc.errContains)
				}
				return
			}

			// If server is not installed, we'll get a different error
			if err != nil {
				// Check if it's a "not installed" error, which is expected
				if _, ok := err.(*ServerNotInstalledError); ok {
					return // This is fine, server just isn't installed
				}
				t.Errorf("unexpected error: %v", err)
				return
			}

			if server.Name != tc.wantServer {
				t.Errorf("got server %q, want %q", server.Name, tc.wantServer)
			}
		})
	}
}

func TestFindProjectRoot(t *testing.T) {
	t.Parallel()

	// Create a temp directory structure
	tmpDir := t.TempDir()

	// Create nested directories
	projectDir := filepath.Join(tmpDir, "project")
	srcDir := filepath.Join(projectDir, "src", "pkg")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}

	// Create a go.mod in project root
	goMod := filepath.Join(projectDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module test"), 0644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	// Create a file deep in the tree
	testFile := filepath.Join(srcDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("writing main.go: %v", err)
	}

	// FindProjectRoot should find the project dir (containing go.mod)
	root := FindProjectRoot(testFile, []string{"go.mod"})
	if root != projectDir {
		t.Errorf("got root %q, want %q", root, projectDir)
	}
}

func TestFindProjectRootNoMarker(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a file but no project marker
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("writing test.go: %v", err)
	}

	// Should return the file's directory when no marker found
	root := FindProjectRoot(testFile, []string{"go.mod"})
	if root != tmpDir {
		t.Errorf("got root %q, want %q", root, tmpDir)
	}
}

func TestServerConfigAvailability(t *testing.T) {
	t.Parallel()

	config := &ServerConfig{
		Name:    "test-server",
		Command: []string{"test-server"},
	}

	// Initially not available
	if config.IsAvailable() {
		t.Error("expected server to be unavailable initially")
	}

	// Mark as available
	config.setAvailable(true)
	if !config.IsAvailable() {
		t.Error("expected server to be available after setAvailable(true)")
	}

	// Mark as unavailable
	config.setAvailable(false)
	if config.IsAvailable() {
		t.Error("expected server to be unavailable after setAvailable(false)")
	}
}

func TestNoServerError(t *testing.T) {
	t.Parallel()

	err := &NoServerError{FileType: ".xyz files"}
	expected := "no language server configured for .xyz files"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestServerNotInstalledError(t *testing.T) {
	t.Parallel()

	err := &ServerNotInstalledError{
		Server:      "gopls",
		InstallHint: "go install golang.org/x/tools/gopls@latest",
	}
	expected := "gopls is not installed. Install with: go install golang.org/x/tools/gopls@latest"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
