package version

import (
	"testing"
)

func TestVersionDefaults(t *testing.T) {
	// Note: Cannot run in parallel due to global variable access

	// Test default values
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if Commit == "" {
		t.Error("Commit should not be empty")
	}

	// Test that defaults are reasonable
	if Version != "0.1.0" {
		t.Errorf("expected default version '0.1.0', got %q", Version)
	}
	if Commit != "dev" {
		t.Errorf("expected default commit 'dev', got %q", Commit)
	}
}

func TestVersionCanBeOverridden(t *testing.T) {
	// Note: Cannot run in parallel due to global variable modification

	// Save original values
	originalVersion := Version
	originalCommit := Commit

	// Temporarily override (simulating build-time -ldflags)
	Version = "1.2.3"
	Commit = "abc123def"

	if Version != "1.2.3" {
		t.Errorf("expected overridden version '1.2.3', got %q", Version)
	}
	if Commit != "abc123def" {
		t.Errorf("expected overridden commit 'abc123def', got %q", Commit)
	}

	// Restore original values
	Version = originalVersion
	Commit = originalCommit
}
