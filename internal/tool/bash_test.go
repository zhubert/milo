package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestBashToolExecute(t *testing.T) {
	t.Parallel()

	bashTool := &BashTool{}

	t.Run("simple echo", func(t *testing.T) {
		t.Parallel()

		input, err := json.Marshal(bashInput{Command: "echo hello"})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := bashTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}
		if !strings.Contains(result.Output, "hello") {
			t.Errorf("output should contain 'hello', got: %s", result.Output)
		}
	})

	t.Run("command with stderr", func(t *testing.T) {
		t.Parallel()

		input, err := json.Marshal(bashInput{Command: "echo out; echo err >&2"})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := bashTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The command itself succeeds (exit 0), so IsError should be false.
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Output)
		}
		if !strings.Contains(result.Output, "out") {
			t.Errorf("output should contain 'out', got: %s", result.Output)
		}
		if !strings.Contains(result.Output, "err") {
			t.Errorf("output should contain 'err', got: %s", result.Output)
		}
	})

	t.Run("command timeout", func(t *testing.T) {
		t.Parallel()

		input, err := json.Marshal(bashInput{Command: "sleep 10", Timeout: 100})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := bashTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError for timed-out command")
		}
		if !strings.Contains(result.Output, "timed out") {
			t.Errorf("output should mention timeout, got: %s", result.Output)
		}
	})

	t.Run("nonexistent command", func(t *testing.T) {
		t.Parallel()

		input, err := json.Marshal(bashInput{Command: "nonexistent_command_12345"})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := bashTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError for nonexistent command")
		}
	})

	t.Run("failing command", func(t *testing.T) {
		t.Parallel()

		input, err := json.Marshal(bashInput{Command: "exit 1"})
		if err != nil {
			t.Fatalf("marshaling input: %v", err)
		}

		result, err := bashTool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError for failing command")
		}
	})
}

func TestBashToolNormalizeInput(t *testing.T) {
	t.Parallel()

	bashTool := &BashTool{WorkDir: "/home/user/project"}

	tests := []struct {
		name            string
		input           string
		expectedCommand string
	}{
		{
			name:            "strips cd workdir with &&",
			input:           `{"command":"cd /home/user/project && go test ./..."}`,
			expectedCommand: "go test ./...",
		},
		{
			name:            "keeps cd to different directory",
			input:           `{"command":"cd /other/path && go test ./..."}`,
			expectedCommand: "cd /other/path && go test ./...",
		},
		{
			name:            "no cd prefix unchanged",
			input:           `{"command":"go test ./..."}`,
			expectedCommand: "go test ./...",
		},
		{
			name:            "strips cd with semicolon",
			input:           `{"command":"cd /home/user/project; make build"}`,
			expectedCommand: "make build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := bashTool.NormalizeInput([]byte(tt.input))

			var parsed bashInput
			if err := json.Unmarshal(result, &parsed); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}
			if parsed.Command != tt.expectedCommand {
				t.Errorf("NormalizeInput command = %q, want %q", parsed.Command, tt.expectedCommand)
			}
		})
	}
}

func TestStripCDToWorkDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		command  string
		workDir  string
		expected string
	}{
		{
			name:     "strips cd workdir with &&",
			command:  "cd /home/user/project && go test ./...",
			workDir:  "/home/user/project",
			expected: "go test ./...",
		},
		{
			name:     "strips cd workdir with semicolon",
			command:  "cd /home/user/project; go test ./...",
			workDir:  "/home/user/project",
			expected: "go test ./...",
		},
		{
			name:     "keeps cd to different directory",
			command:  "cd /other/path && go test ./...",
			workDir:  "/home/user/project",
			expected: "cd /other/path && go test ./...",
		},
		{
			name:     "no cd prefix",
			command:  "go test ./...",
			workDir:  "/home/user/project",
			expected: "go test ./...",
		},
		{
			name:     "cd without separator",
			command:  "cd /home/user/project",
			workDir:  "/home/user/project",
			expected: "cd /home/user/project",
		},
		{
			name:     "handles extra whitespace",
			command:  "cd /home/user/project   &&   go test ./...",
			workDir:  "/home/user/project",
			expected: "go test ./...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := stripCDToWorkDir(tt.command, tt.workDir)
			if result != tt.expected {
				t.Errorf("stripCDToWorkDir(%q, %q) = %q, want %q",
					tt.command, tt.workDir, result, tt.expected)
			}
		})
	}
}
