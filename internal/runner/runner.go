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
	"github.com/zhubert/milo/internal/permission"
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

			// Check for slash commands.
			if strings.HasPrefix(input, "/") {
				r.handleSlashCommand(input)
				continue
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

	var textBuffer strings.Builder // Buffer text for markdown rendering
	var pendingTool string          // Track current tool for result display

	// flushText renders and prints any buffered text
	flushText := func() {
		if textBuffer.Len() > 0 {
			rendered := renderMarkdown(textBuffer.String())
			fmt.Print(rendered)
			textBuffer.Reset()
		}
	}

	for {
		select {
		case <-sigCh:
			cancel()
			flushText()
			fmt.Println(colorYellow + "\n[Cancelled]" + colorReset)
			return nil

		case chunk, ok := <-ch:
			if !ok {
				// Channel closed unexpectedly.
				flushText()
				return nil
			}

			switch chunk.Type {
			case agent.ChunkText:
				textBuffer.WriteString(chunk.Text)

			case agent.ChunkToolUse:
				flushText()
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
				flushText()
				if pendingTool != "" {
					fmt.Println() // finish pending tool line
					pendingTool = ""
				}
				// Show the command/input being requested in function-call style
				permInfo := formatPermissionInfo(chunk.ToolName, chunk.ToolInput)
				fmt.Printf("\n%sAllow %s%s(%s%s%s)%s? [y/n/a]: %s",
					colorYellow, colorBold, chunk.ToolName, colorDim, permInfo, colorYellow, colorReset, colorReset)
				resp := r.readPermissionResponse()
				r.agent.PermResp <- resp

			case agent.ChunkParallelProgress:
				// Skip parallel progress - too noisy

			case agent.ChunkContextCompacted:
				flushText()
				fmt.Printf("  %s→ context compacted%s\n", colorDim, colorReset)

			case agent.ChunkDone:
				flushText()
				fmt.Println()
				r.saveSession()
				return nil

			case agent.ChunkError:
				flushText()
				if chunk.Err != nil {
					return chunk.Err
				}
				return fmt.Errorf("unknown error")
			}
		}
	}
}

func (r *Runner) handleSlashCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/model", "/m":
		r.handleModelCommand(args)
	case "/permissions", "/perms", "/p":
		r.handlePermissionsCommand(args)
	case "/help", "/h", "/?":
		r.handleHelpCommand()
	default:
		fmt.Printf("%sUnknown command: %s. Type /help for available commands.%s\n", colorRed, cmd, colorReset)
	}
}

func (r *Runner) handleHelpCommand() {
	help := `
Available commands:
  /model, /m               - Change or view the current model
    list                   - Show available models
    <model-id>             - Switch to a model (e.g. /m claude-opus-4-5)
  /permissions, /perms, /p - Manage permission rules
    list                   - Show all custom rules
    add <rule>             - Add a rule, e.g. Bash(git:*)
    remove <rule>          - Remove a rule
  /help, /h, /?            - Show this help message

  exit, quit               - Close the application
`
	fmt.Println(help)
}

func (r *Runner) handleModelCommand(args []string) {
	if len(args) == 0 {
		r.listModels()
		return
	}

	subcmd := strings.ToLower(args[0])
	if subcmd == "list" || subcmd == "ls" || subcmd == "l" {
		r.listModels()
		return
	}

	// Treat argument as model ID
	r.switchModel(args[0])
}

func (r *Runner) listModels() {
	current := r.agent.Model()
	models := agent.AvailableModels()

	fmt.Printf("\nCurrent model: %s%s%s\n\n", colorBold, r.agent.ModelDisplayName(), colorReset)
	fmt.Println("Available models:")
	for _, opt := range models {
		marker := "  "
		if opt.ID == current {
			marker = colorGreen + "→ " + colorReset
		}
		fmt.Printf("%s%-35s %s%s%s\n", marker, opt.ID, colorDim, opt.DisplayName, colorReset)
	}
	fmt.Println("\nUsage: /model <model-id>")
}

func (r *Runner) switchModel(modelID string) {
	models := agent.AvailableModels()

	// Find matching model (partial match allowed)
	var match *agent.ModelOption
	for _, opt := range models {
		if strings.Contains(strings.ToLower(opt.ID), strings.ToLower(modelID)) {
			match = &opt
			break
		}
	}

	if match == nil {
		fmt.Printf("%sModel not found: %s%s\n", colorRed, modelID, colorReset)
		fmt.Println("Use /model list to see available models.")
		return
	}

	r.agent.SetModel(match.ID)
	fmt.Printf("Switched to %s%s%s\n", colorGreen, match.DisplayName, colorReset)
}

func (r *Runner) handlePermissionsCommand(args []string) {
	perms := r.agent.Permissions()

	if len(args) == 0 {
		help := `
Permission commands:
  /permissions list           - Show all custom rules
  /permissions add <rule>     - Add a rule (default: allow)
  /permissions remove <rule>  - Remove a rule

Examples:
  /p add Bash(npm:*)
  /p add Bash(go build:*)
  /p add Bash(rm -rf *):deny
  /p remove Bash(npm:*)
`
		fmt.Println(help)
		return
	}

	subcmd := strings.ToLower(args[0])
	subargs := args[1:]

	switch subcmd {
	case "list", "ls", "l":
		r.listPermissions(perms)
	case "add", "a":
		r.addPermission(perms, subargs)
	case "remove", "rm", "delete", "del":
		r.removePermission(perms, subargs)
	default:
		fmt.Printf("%sUnknown permissions subcommand: %s%s\n", colorRed, subcmd, colorReset)
	}
}

func (r *Runner) listPermissions(perms *permission.Checker) {
	rules := perms.CustomRules()
	if len(rules) == 0 {
		fmt.Println("No custom permission rules configured.")
		return
	}

	fmt.Println("\nCustom permission rules:")
	for _, rule := range rules {
		fmt.Printf("  %s\n", rule.String())
	}
	fmt.Println()
}

func (r *Runner) addPermission(perms *permission.Checker, args []string) {
	if len(args) == 0 {
		fmt.Printf("%sUsage: /permissions add <rule>%s\n", colorRed, colorReset)
		return
	}

	ruleStr := strings.Join(args, " ")
	rule, err := permission.ParseRule(ruleStr)
	if err != nil {
		fmt.Printf("%sError parsing rule: %v%s\n", colorRed, err, colorReset)
		return
	}
	perms.AddRule(rule)
	fmt.Printf("Added rule: %s%s%s\n", colorGreen, ruleStr, colorReset)
}

func (r *Runner) removePermission(perms *permission.Checker, args []string) {
	if len(args) == 0 {
		fmt.Printf("%sUsage: /permissions remove <rule>%s\n", colorRed, colorReset)
		return
	}

	rule := strings.Join(args, " ")
	if perms.RemoveRule(rule) {
		fmt.Printf("Removed rule: %s%s%s\n", colorGreen, rule, colorReset)
	} else {
		fmt.Printf("%sRule not found: %s%s\n", colorYellow, rule, colorReset)
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
	case "multi_read":
		if files, ok := data["files"].([]any); ok {
			return fmt.Sprintf("%s %s(%d files)%s", name, colorDim, len(files), colorReset)
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

// renderMarkdown converts markdown to styled terminal output.
func renderMarkdown(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")
	inCodeBlock := false
	var codeLines []string
	var codeLang string

	for _, line := range lines {
		// Code block fences
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				inCodeBlock = true
				codeLang = strings.TrimPrefix(line, "```")
				codeLines = nil
			} else {
				// Render the code block
				result.WriteString(renderCodeBlock(codeLines, codeLang))
				inCodeBlock = false
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Headers
		if strings.HasPrefix(line, "### ") {
			text := strings.TrimPrefix(line, "### ")
			result.WriteString(colorBold + text + colorReset + "\n")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			text := strings.TrimPrefix(line, "## ")
			result.WriteString(colorBold + colorCyan + text + colorReset + "\n")
			continue
		}
		if strings.HasPrefix(line, "# ") {
			text := strings.TrimPrefix(line, "# ")
			result.WriteString(colorBold + colorCyan + text + colorReset + "\n")
			continue
		}

		// Bullet lists
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result.WriteString("  • " + renderInline(line[2:]) + "\n")
			continue
		}

		// Numbered lists (basic support)
		if len(line) > 2 && line[0] >= '1' && line[0] <= '9' && line[1] == '.' && line[2] == ' ' {
			result.WriteString("  " + line[:2] + " " + renderInline(line[3:]) + "\n")
			continue
		}

		// Regular line with inline formatting
		result.WriteString(renderInline(line) + "\n")
	}

	// Handle unclosed code block
	if inCodeBlock && len(codeLines) > 0 {
		result.WriteString(renderCodeBlock(codeLines, codeLang))
	}

	return result.String()
}

// renderCodeBlock formats a code block with background styling.
func renderCodeBlock(lines []string, lang string) string {
	var b strings.Builder
	// Code block colors
	codeBg := "\033[48;5;236m" // dark gray background
	codeFg := "\033[38;5;159m" // light cyan text

	if lang != "" {
		b.WriteString(colorDim + " " + lang + " " + colorReset + "\n")
	}
	for _, line := range lines {
		b.WriteString(codeBg + codeFg + " " + line + " " + colorReset + "\n")
	}
	return b.String()
}

// renderInline handles bold and inline code.
func renderInline(text string) string {
	// Bold: **text**
	text = renderPattern(text, "**", "**", colorBold, colorReset)
	// Inline code: `text`
	codeStyle := "\033[48;5;236m\033[38;5;159m"
	text = renderPattern(text, "`", "`", codeStyle, colorReset)
	return text
}

// renderPattern replaces delimited text with styled text.
func renderPattern(text, open, close, startStyle, endStyle string) string {
	var result strings.Builder
	for {
		start := strings.Index(text, open)
		if start == -1 {
			result.WriteString(text)
			break
		}
		end := strings.Index(text[start+len(open):], close)
		if end == -1 {
			result.WriteString(text)
			break
		}
		end += start + len(open)

		result.WriteString(text[:start])
		inner := text[start+len(open) : end]
		result.WriteString(startStyle + inner + endStyle)
		text = text[end+len(close):]
	}
	return result.String()
}

// formatPermissionInfo extracts the key info from tool input for permission display.
func formatPermissionInfo(toolName, input string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		// If not JSON, just return truncated input
		if len(input) > 80 {
			return input[:77] + "..."
		}
		return input
	}

	switch toolName {
	case "bash":
		if cmd, ok := data["command"].(string); ok {
			if len(cmd) > 80 {
				return cmd[:77] + "..."
			}
			return cmd
		}
	case "write", "edit", "read":
		if fp, ok := data["file_path"].(string); ok {
			return fp
		}
	default:
		if fp, ok := data["file_path"].(string); ok {
			return fp
		}
	}

	// Fallback: truncated JSON
	if len(input) > 80 {
		return input[:77] + "..."
	}
	return input
}
