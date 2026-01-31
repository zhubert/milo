package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetup(t *testing.T) {
	t.Parallel()

	// Create a temporary home directory for testing
	tempHome := t.TempDir()

	// Override the home directory for this test
	originalHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Errorf("failed to restore HOME: %v", err)
		}
	}()

	logger, cleanup, err := Setup()
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	var cleanupCalled bool
	defer func() {
		if !cleanupCalled {
			if cerr := cleanup(); cerr != nil {
				t.Errorf("cleanup failed: %v", cerr)
			}
		}
	}()

	// Verify logger is not nil
	if logger == nil {
		t.Error("expected non-nil logger")
	}

	// Verify cleanup function is not nil
	if cleanup == nil {
		t.Error("expected non-nil cleanup function")
	}

	// Verify the .milo directory was created
	miloDir := filepath.Join(tempHome, ".milo")
	if _, err := os.Stat(miloDir); os.IsNotExist(err) {
		t.Error("expected .milo directory to be created")
	}

	// Verify the log file was created
	logPath := filepath.Join(miloDir, "debug.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("expected debug.log file to be created")
	}

	// Test that we can write to the logger
	logger.Debug("test message")
	logger.Info("test info message")

	// Call cleanup to close the file
	if err := cleanup(); err != nil {
		t.Errorf("cleanup failed: %v", err)
	}
	cleanupCalled = true

	// Verify the log file contains JSON entries
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, `"level":"DEBUG"`) {
		t.Error("expected DEBUG level entry in log file")
	}
	if !strings.Contains(contentStr, `"level":"INFO"`) {
		t.Error("expected INFO level entry in log file")
	}
	if !strings.Contains(contentStr, "test message") {
		t.Error("expected test message in log file")
	}
}

func TestSetupWithExistingDirectory(t *testing.T) {
	t.Parallel()

	// Create a temporary home directory for testing
	tempHome := t.TempDir()

	// Pre-create the .milo directory
	miloDir := filepath.Join(tempHome, ".milo")
	if err := os.MkdirAll(miloDir, 0o755); err != nil {
		t.Fatalf("failed to create .milo directory: %v", err)
	}

	// Override the home directory for this test
	originalHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Errorf("failed to restore HOME: %v", err)
		}
	}()

	logger, cleanup, err := Setup()
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}
	defer func() {
		if cerr := cleanup(); cerr != nil {
			t.Errorf("cleanup failed: %v", cerr)
		}
	}()

	if logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestSetupTruncatesExistingFile(t *testing.T) {
	t.Parallel()

	// Create a temporary home directory for testing
	tempHome := t.TempDir()
	miloDir := filepath.Join(tempHome, ".milo")
	if err := os.MkdirAll(miloDir, 0o755); err != nil {
		t.Fatalf("failed to create .milo directory: %v", err)
	}

	// Pre-create a log file with some content
	logPath := filepath.Join(miloDir, "debug.log")
	existingContent := "This should be truncated"
	if err := os.WriteFile(logPath, []byte(existingContent), 0o644); err != nil {
		t.Fatalf("failed to create existing log file: %v", err)
	}

	// Override the home directory for this test
	originalHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Errorf("failed to restore HOME: %v", err)
		}
	}()

	logger, cleanup, err := Setup()
	if err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	var cleanupCalled bool
	defer func() {
		if !cleanupCalled {
			if cerr := cleanup(); cerr != nil {
				t.Errorf("cleanup failed: %v", cerr)
			}
		}
	}()

	// Write a new message
	logger.Info("new message after truncation")

	// Close the file so we can read it
	if err := cleanup(); err != nil {
		t.Errorf("cleanup failed: %v", err)
	}
	cleanupCalled = true

	// Verify the file was truncated and contains only the new content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, existingContent) {
		t.Error("expected existing content to be truncated")
	}
	if !strings.Contains(contentStr, "new message after truncation") {
		t.Error("expected new message in truncated log file")
	}
}

func TestSetupInvalidHomeDir(t *testing.T) {
	t.Parallel()

	// Override HOME to an invalid value
	originalHome := os.Getenv("HOME")
	if err := os.Unsetenv("HOME"); err != nil {
		t.Fatalf("failed to unset HOME: %v", err)
	}
	defer func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Errorf("failed to restore HOME: %v", err)
		}
	}()

	logger, cleanup, err := Setup()
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Error("expected Setup() to fail with invalid home directory")
	}
	if logger != nil {
		t.Error("expected nil logger when Setup() fails")
	}
	if cleanup != nil {
		t.Error("expected nil cleanup function when Setup() fails")
	}
}
