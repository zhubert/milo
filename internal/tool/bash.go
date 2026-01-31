package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultBashTimeout = 2 * time.Minute

// BashTool executes shell commands.
type BashTool struct {
	WorkDir string
}

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"` // milliseconds
}

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	return "Execute a bash command and return its output. " +
		"The command runs in a shell with a configurable timeout (default 2 minutes). " +
		"Both stdout and stderr are captured."
}

func (t *BashTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The bash command to execute",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in milliseconds (default 120000)",
			},
		},
		Required: []string{"command"},
	}
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing bash input: %w", err)
	}

	timeout := defaultBashTimeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
	if t.WorkDir != "" {
		cmd.Dir = t.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdin = nil // Explicitly close stdin to prevent commands from blocking on input
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var output string
	if stdout.Len() > 0 {
		output = stdout.String()
	}
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Result{Output: fmt.Sprintf("command timed out after %s", timeout), IsError: true}, nil
		}
		if output == "" {
			output = err.Error()
		}
		return Result{Output: output, IsError: true}, nil
	}

	if output == "" {
		output = "(no output)"
	}

	return Result{Output: output}, nil
}
