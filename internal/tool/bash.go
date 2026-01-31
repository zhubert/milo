package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultBashTimeout = 2 * time.Minute

// stripCDToWorkDir removes a "cd <workdir> &&" or "cd <workdir>;" prefix from a command
// if the path matches the working directory. Commands already run in the working directory,
// so this prefix is redundant.
func stripCDToWorkDir(command, workDir string) string {
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "cd ") {
		return command
	}

	// Find the separator (&& or ;)
	rest := trimmed[3:] // after "cd "
	var sepIdx int
	var sepLen int

	andIdx := strings.Index(rest, "&&")
	semiIdx := strings.Index(rest, ";")

	if andIdx == -1 && semiIdx == -1 {
		return command // no separator, keep as-is
	}

	if andIdx == -1 {
		sepIdx = semiIdx
		sepLen = 1
	} else if semiIdx == -1 {
		sepIdx = andIdx
		sepLen = 2
	} else if andIdx < semiIdx {
		sepIdx = andIdx
		sepLen = 2
	} else {
		sepIdx = semiIdx
		sepLen = 1
	}

	path := strings.TrimSpace(rest[:sepIdx])
	remainder := strings.TrimSpace(rest[sepIdx+sepLen:])

	// Check if the path matches the working directory
	if path == workDir {
		return remainder
	}

	return command
}

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

// NormalizeInput strips redundant "cd <workdir> &&" prefixes from bash commands.
// This ensures permission checks and execution see the actual command being run.
func (t *BashTool) NormalizeInput(input json.RawMessage) json.RawMessage {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return input
	}

	normalized := stripCDToWorkDir(in.Command, t.WorkDir)
	if normalized == in.Command {
		return input // No change needed
	}

	in.Command = normalized
	result, err := json.Marshal(in)
	if err != nil {
		return input
	}
	return result
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing bash input: %w", err)
	}

	// Strip unnecessary "cd <workdir> &&" prefix - commands already run in the working directory
	in.Command = stripCDToWorkDir(in.Command, t.WorkDir)

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
