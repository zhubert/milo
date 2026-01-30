package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/zhubert/milo/internal/agent"
	"github.com/zhubert/milo/internal/session"
	"github.com/zhubert/milo/internal/version"
)

// Colors for terminal output.
const (
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
)

// Runner handles the simple stdin/stdout interaction loop.
type Runner struct {
	agent        *agent.Agent
	session      *session.Session
	sessionStore *session.Store
	workDir      string
	cancel       context.CancelFunc
}

// New creates a new Runner.
func New(ag *agent.Agent, workDir string, store *session.Store, sess *session.Session) *Runner {
	return &Runner{
		agent:        ag,
		session:      sess,
		sessionStore: store,
		workDir:      workDir,
	}
}

// Run starts the main interaction loop.
func (r *Runner) Run() error {
	r.printWelcome()

	// Handle ctrl+c gracefully.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	reader := bufio.NewReader(os.Stdin)

	for {
		// Print prompt.
		fmt.Print(colorBold + "> " + colorReset)

		// Read input with interrupt handling.
		inputCh := make(chan string, 1)
		errCh := make(chan error, 1)

		go func() {
			line, err := reader.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			inputCh <- strings.TrimSpace(line)
		}()

		select {
		case <-sigCh:
			fmt.Println("\nGoodbye.")
			return nil
		case err := <-errCh:
			if err.Error() == "EOF" {
				fmt.Println("\nGoodbye.")
				return nil
			}
			return fmt.Errorf("reading input: %w", err)
		case input := <-inputCh:
			if input == "" {
				continue
			}

			// Check for exit commands.
			lower := strings.ToLower(input)
			if lower == "exit" || lower == "quit" {
				fmt.Println("Goodbye.")
				return nil
			}

			// Process the input.
			if err := r.processInput(input, sigCh); err != nil {
				fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
			}
		}
	}
}

func (r *Runner) processInput(input string, sigCh chan os.Signal) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	defer func() {
		r.cancel = nil
	}()

	// Start streaming response.
	ch := r.agent.SendMessage(ctx, input)

	fmt.Println() // blank line before response

	var hasOutput bool
	var pendingTool string // Track current tool for result display

	for {
		select {
		case <-sigCh:
			cancel()
			fmt.Println(colorYellow + "\n[Cancelled]" + colorReset)
			return nil

		case chunk, ok := <-ch:
			if !ok {
				// Channel closed unexpectedly.
				if hasOutput {
					fmt.Println()
				}
				return nil
			}

			switch chunk.Type {
			case agent.ChunkText:
				hasOutput = true
				fmt.Print(chunk.Text)

			case agent.ChunkToolUse:
				if hasOutput {
					fmt.Println()
					hasOutput = false
				}
				// Show tool with file info if available
				toolInfo := formatToolInfo(chunk.ToolName, chunk.ToolInput)
				pendingTool = chunk.ToolName
				fmt.Printf("  %s→%s %s ", colorDim, colorReset, toolInfo)

			case agent.ChunkToolResult:
				if chunk.Result != nil {
					// Show status and line count
					var status string
					if chunk.Result.IsError {
						status = colorRed + "✗" + colorReset
					} else {
						status = colorGreen + "✓" + colorReset
					}
					lines := countLines(chunk.Result.Output)
					if lines > 0 {
						fmt.Printf("%s %s(%d lines)%s\n", status, colorDim, lines, colorReset)
					} else {
						fmt.Printf("%s\n", status)
					}
				} else {
					fmt.Println()
				}
				pendingTool = ""

			case agent.ChunkPermissionRequest:
				if pendingTool != "" {
					fmt.Println() // finish pending tool line
					pendingTool = ""
				}
				// Show the command/input being requested
				fmt.Printf("\n%s─── Permission Required ───%s\n", colorYellow, colorReset)
				fmt.Printf("%s%s%s\n", colorBold, chunk.ToolName, colorReset)
				if chunk.ToolInput != "" {
					// Try to show the command or file path nicely
					permInfo := formatPermissionInfo(chunk.ToolName, chunk.ToolInput)
					fmt.Printf("%s%s%s\n", colorDim, permInfo, colorReset)
				}
				fmt.Printf("%sAllow? [y/n/a(lways)]: %s", colorYellow, colorReset)
				resp := r.readPermissionResponse()
				r.agent.PermResp <- resp

			case agent.ChunkParallelProgress:
				// Skip parallel progress - too noisy

			case agent.ChunkContextCompacted:
				fmt.Printf("  %s→ context compacted%s\n", colorDim, colorReset)

			case agent.ChunkDone:
				if hasOutput {
					fmt.Println()
				}
				fmt.Println()
				r.saveSession()
				return nil

			case agent.ChunkError:
				if hasOutput {
					fmt.Println()
				}
				if chunk.Err != nil {
					return chunk.Err
				}
				return fmt.Errorf("unknown error")
			}
		}
	}
}

func (r *Runner) readPermissionResponse() agent.PermissionResponse {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return agent.PermissionDenied
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return agent.PermissionGranted
		case "n", "no":
			return agent.PermissionDenied
		case "a", "always":
			return agent.PermissionGrantedAlways
		default:
			fmt.Print("Please enter y, n, or a: ")
		}
	}
}

func (r *Runner) saveSession() {
	if r.sessionStore == nil || r.session == nil {
		return
	}

	r.session.SetMessages(r.agent.Messages())

	if r.session.Title == "" && len(r.session.Messages) > 0 {
		r.session.Title = extractSessionTitle(r.session.Messages)
	}

	_ = r.sessionStore.Save(r.session)
}

func extractSessionTitle(messages []anthropic.MessageParam) string {
	for _, msg := range messages {
		if string(msg.Role) == "user" {
			for _, block := range msg.Content {
				if block.OfText != nil && block.OfText.Text != "" {
					text := block.OfText.Text
					if len(text) > 50 {
						return text[:47] + "..."
					}
					return text
				}
			}
		}
	}
	return "Untitled"
}

// ASCII art logo with gradient colors.
var logo = []string{
	"\033[38;5;206m███╗   ███╗██╗██╗      ██████╗ \033[0m",
	"\033[38;5;177m████╗ ████║██║██║     ██╔═══██╗\033[0m",
	"\033[38;5;141m██╔████╔██║██║██║     ██║   ██║\033[0m",
	"\033[38;5;117m██║╚██╔╝██║██║██║     ██║   ██║\033[0m",
	"\033[38;5;117m██║ ╚═╝ ██║██║███████╗╚██████╔╝\033[0m",
	"\033[38;5;117m╚═╝     ╚═╝╚═╝╚══════╝ ╚═════╝ \033[0m",
}

func (r *Runner) printWelcome() {
	// Print logo with info on the right.
	info := []string{
		"",
		fmt.Sprintf("v%s", version.Version),
		r.agent.ModelDisplayName(),
		shortenPath(r.workDir),
		"",
		"",
	}

	for i, line := range logo {
		fmt.Printf("%s  %s%s%s\n", line, colorDim, info[i], colorReset)
	}
	fmt.Println()
}

func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// extractFilePath attempts to extract a file_path from JSON input.
func extractFilePath(input string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return ""
	}
	if fp, ok := data["file_path"].(string); ok {
		return fp
	}
	// Try "path" as well (used by some tools)
	if fp, ok := data["path"].(string); ok {
		return fp
	}
	return ""
}

// shortenFilePath returns the last n components of a path.
func shortenFilePath(path string, components int) string {
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) <= components {
		return path
	}
	return "…/" + strings.Join(parts[len(parts)-components:], string(filepath.Separator))
}

// formatToolInfo creates a display string for a tool use.
func formatToolInfo(name, input string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return name
	}

	switch name {
	case "bash":
		if cmd, ok := data["command"].(string); ok {
			// Truncate long commands
			if len(cmd) > 60 {
				cmd = cmd[:57] + "..."
			}
			return fmt.Sprintf("%s %s%s%s", name, colorDim, cmd, colorReset)
		}
	case "read", "write", "edit", "glob", "grep":
		if fp, ok := data["file_path"].(string); ok {
			short := shortenFilePath(fp, 3)
			return fmt.Sprintf("%s %s%s%s", name, colorDim, short, colorReset)
		}
		if p, ok := data["path"].(string); ok {
			short := shortenFilePath(p, 3)
			return fmt.Sprintf("%s %s%s%s", name, colorDim, short, colorReset)
		}
	case "list_dir":
		if p, ok := data["path"].(string); ok {
			short := shortenFilePath(p, 3)
			return fmt.Sprintf("%s %s%s%s", name, colorDim, short, colorReset)
		}
	}

	return name
}

// countLines returns the number of lines in a string.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// formatPermissionInfo extracts the key info from tool input for permission display.
func formatPermissionInfo(toolName, input string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		// If not JSON, just return truncated input
		if len(input) > 200 {
			return input[:200] + "…"
		}
		return input
	}

	switch toolName {
	case "bash":
		if cmd, ok := data["command"].(string); ok {
			// Show the command, potentially truncated
			if len(cmd) > 200 {
				return cmd[:200] + "…"
			}
			return cmd
		}
	case "write":
		if fp, ok := data["file_path"].(string); ok {
			return fmt.Sprintf("write to: %s", fp)
		}
	case "edit":
		if fp, ok := data["file_path"].(string); ok {
			return fmt.Sprintf("edit: %s", fp)
		}
	default:
		// Try file_path
		if fp, ok := data["file_path"].(string); ok {
			return fp
		}
	}

	// Fallback: truncated JSON
	if len(input) > 200 {
		return input[:200] + "…"
	}
	return input
}
