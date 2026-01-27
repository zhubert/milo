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
