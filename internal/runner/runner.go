package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/charmbracelet/glamour"
	"github.com/chzyer/readline"

	"github.com/zhubert/milo/internal/agent"
	"github.com/zhubert/milo/internal/permission"
	"github.com/zhubert/milo/internal/session"
	"github.com/zhubert/milo/internal/todo"
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
	rl           *readline.Instance
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

	// Set up readline with history.
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          colorBold + "> " + colorReset,
		HistoryFile:     filepath.Join(os.Getenv("HOME"), ".milo_history"),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("initializing readline: %w", err)
	}
	defer rl.Close()
	r.rl = rl

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				continue // Ctrl+C clears line, continue prompting
			}
			if err == io.EOF {
				fmt.Println("Goodbye.")
				return nil
			}
			return fmt.Errorf("reading input: %w", err)
		}

		input := strings.TrimSpace(line)
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

func (r *Runner) processInput(input string, sigCh chan os.Signal) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	defer func() {
		r.cancel = nil
	}()

	// Start streaming response.
	ch := r.agent.SendMessage(ctx, input)

	fmt.Println() // blank line before response

	var textBuffer strings.Builder   // Buffer text for markdown rendering
	var pendingTool string           // Track current tool for result display
	var initialTodosShown bool       // Have we shown the initial todo list?
	var currentInProgressTask string // Current in-progress task (to detect changes)
	var hasActiveTask bool           // Is there a task in progress? (for indentation)

	// toolIndent returns the appropriate indentation for tool output
	toolIndent := func() string {
		if hasActiveTask {
			return "    " // Extra indent under task header
		}
		return "  "
	}

	// flushText renders and prints any buffered text
	flushText := func() {
		if textBuffer.Len() > 0 {
			rendered := renderMarkdown(textBuffer.String())
			fmt.Print(rendered)
			textBuffer.Reset()
		}
	}

	// shouldFlush checks if we should flush (line complete, not in code block)
	shouldFlush := func() bool {
		s := textBuffer.String()
		if !strings.HasSuffix(s, "\n") {
			return false
		}
		// Count code fences to determine if we're in a code block
		fenceCount := strings.Count(s, "```")
		return fenceCount%2 == 0 // Even = not in code block
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
				if shouldFlush() {
					flushText()
				}

			case agent.ChunkToolUse:
				// Skip displaying todo tool - shown via task headers
				if chunk.ToolName == "todo" {
					continue
				}
				flushText()
				// Show tool with file info if available
				toolInfo := formatToolInfo(chunk.ToolName, chunk.ToolInput)
				pendingTool = chunk.ToolName
				fmt.Printf("%s%s→%s %s ", toolIndent(), colorDim, colorReset, toolInfo)
				os.Stdout.Sync() // Flush to show tool info immediately

			case agent.ChunkToolResult:
				// Only show result if there's a pending tool line to complete
				if pendingTool == "" {
					continue
				}
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
				prompt := fmt.Sprintf("%sAllow %s%s(%s%s%s)%s? [y/n/a]: %s",
					colorYellow, colorBold, chunk.ToolName, colorDim, permInfo, colorYellow, colorReset, colorReset)
				resp := r.readPermissionResponseWithPrompt(prompt)
				r.agent.PermResp <- resp

			case agent.ChunkParallelProgress:
				// Skip parallel progress - too noisy

			case agent.ChunkContextCompacted:
				flushText()
				fmt.Printf("%s%s→ context compacted%s\n", toolIndent(), colorDim, colorReset)

			case agent.ChunkTodoUpdate:
				flushText()
				if len(chunk.Todos) == 0 {
					continue
				}

				// Find current in-progress task
				var currentTask string
				for _, t := range chunk.Todos {
					if t.Status == todo.StatusInProgress {
						if t.ActiveForm != "" {
							currentTask = t.ActiveForm
						} else {
							currentTask = t.Content
						}
						break
					}
				}

				// Show initial todo list on first update
				if !initialTodosShown {
					fmt.Printf("\n  %sTasks:%s\n", colorBold, colorReset)
					for _, t := range chunk.Todos {
						fmt.Printf("  %s○%s %s%s%s\n", colorDim, colorReset, colorDim, t.Content, colorReset)
					}
					fmt.Println()
					initialTodosShown = true
				}

				// Show new task header when in-progress task changes
				if currentTask != "" && currentTask != currentInProgressTask {
					fmt.Printf("  %s●%s %s\n", colorYellow, colorReset, currentTask)
					currentInProgressTask = currentTask
					hasActiveTask = true
				} else if currentTask == "" {
					hasActiveTask = false
				}

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
    <number>               - Switch by number (e.g. /m 3)
    <model-id>             - Switch by name (e.g. /m opus)
  /permissions, /perms, /p - Manage permission rules
    list                   - Show all custom rules (numbered)
    add <rule>             - Add a rule, e.g. Bash(git:*)
    rm <number>            - Remove by number (e.g. /p rm 2)
    rm <rule>              - Remove by rule text
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
	for i, opt := range models {
		marker := "  "
		if opt.ID == current {
			marker = colorGreen + "→ " + colorReset
		}
		fmt.Printf("%s%s[%d]%s %-35s %s%s%s\n", marker, colorCyan, i+1, colorReset, opt.ID, colorDim, opt.DisplayName, colorReset)
	}
	fmt.Println("\nUsage: /m <number> or /m <model-id>")
}

func (r *Runner) switchModel(modelID string) {
	models := agent.AvailableModels()

	// Check if input is a number (1-indexed selection)
	if num, err := strconv.Atoi(modelID); err == nil {
		if num >= 1 && num <= len(models) {
			match := models[num-1]
			r.agent.SetModel(match.ID)
			fmt.Printf("Switched to %s%s%s\n", colorGreen, match.DisplayName, colorReset)
			return
		}
		fmt.Printf("%sInvalid model number: %d (choose 1-%d)%s\n", colorRed, num, len(models), colorReset)
		return
	}

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
  /p list              - Show all custom rules (numbered)
  /p add <rule>        - Add a rule (default: allow)
  /p rm <number>       - Remove by number (e.g. /p rm 2)
  /p rm <rule>         - Remove by rule text

Examples:
  /p add Bash(npm:*)
  /p add Bash(go build:*)
  /p add Bash(rm -rf *):deny
  /p rm 1
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
	for i, rule := range rules {
		fmt.Printf("  %s[%d]%s %s\n", colorCyan, i+1, colorReset, rule.String())
	}
	fmt.Println("\nUsage: /p rm <number> or /p rm <rule>")
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
		fmt.Printf("%sUsage: /permissions remove <number> or <rule>%s\n", colorRed, colorReset)
		return
	}

	rules := perms.CustomRules()

	// Check if input is a number (1-indexed selection)
	if num, err := strconv.Atoi(args[0]); err == nil {
		if num >= 1 && num <= len(rules) {
			rule := rules[num-1]
			perms.RemoveRule(rule.Key())
			fmt.Printf("Removed rule: %s%s%s\n", colorGreen, rule.String(), colorReset)
			return
		}
		if len(rules) == 0 {
			fmt.Printf("%sNo rules to remove%s\n", colorRed, colorReset)
		} else {
			fmt.Printf("%sInvalid rule number: %d (choose 1-%d)%s\n", colorRed, num, len(rules), colorReset)
		}
		return
	}

	rule := strings.Join(args, " ")
	if perms.RemoveRule(rule) {
		fmt.Printf("Removed rule: %s%s%s\n", colorGreen, rule, colorReset)
	} else {
		fmt.Printf("%sRule not found: %s%s\n", colorYellow, rule, colorReset)
	}
}

func (r *Runner) readPermissionResponseWithPrompt(prompt string) agent.PermissionResponse {
	// Set the prompt for readline (this ensures proper display)
	oldPrompt := r.rl.Config.Prompt
	r.rl.SetPrompt(prompt)

	first := true
	for {
		if !first {
			r.rl.SetPrompt("Please enter y, n, or a: ")
		}
		first = false

		line, err := r.rl.Readline()
		if err != nil {
			r.rl.SetPrompt(oldPrompt)
			return agent.PermissionDenied
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			r.rl.SetPrompt(oldPrompt)
			return agent.PermissionGranted
		case "n", "no":
			r.rl.SetPrompt(oldPrompt)
			return agent.PermissionDenied
		case "a", "always":
			r.rl.SetPrompt(oldPrompt)
			return agent.PermissionGrantedAlways
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

// markdownRenderer is the glamour renderer for terminal markdown.
var markdownRenderer *glamour.TermRenderer

func init() {
	var err error
	markdownRenderer, err = glamour.NewTermRenderer(
		glamour.WithStylePath("tokyo-night"),
		glamour.WithWordWrap(0), // No wrapping - let terminal handle it
	)
	if err != nil {
		// Fallback: no rendering
		markdownRenderer = nil
	}
}

// addDefaultLanguageToCodeBlocks adds "text" as the language for code blocks
// that don't specify a language. This prevents chroma from auto-detecting
// and showing incorrect syntax highlighting (e.g., treating tree characters as errors).
//
// Handles both:
// - Fenced code blocks (```) without a language
// - 4-space indented code blocks (converted to fenced with "text")
func addDefaultLanguageToCodeBlocks(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")
	inFencedBlock := false
	inIndentedBlock := false
	var indentedLines []string

	flushIndentedBlock := func() {
		if len(indentedLines) > 0 {
			result.WriteString("```text\n")
			for _, l := range indentedLines {
				// Remove the 4-space indent
				if len(l) >= 4 {
					result.WriteString(l[4:])
				} else {
					result.WriteString(l)
				}
				result.WriteString("\n")
			}
			result.WriteString("```\n")
			indentedLines = nil
		}
		inIndentedBlock = false
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Handle fenced code blocks
		if strings.HasPrefix(line, "```") {
			flushIndentedBlock()
			if !inFencedBlock {
				// Opening fence
				lang := strings.TrimPrefix(line, "```")
				lang = strings.TrimSpace(lang)
				if lang == "" {
					result.WriteString("```text\n")
				} else {
					result.WriteString(line + "\n")
				}
				inFencedBlock = true
			} else {
				// Closing fence
				result.WriteString(line + "\n")
				inFencedBlock = false
			}
			continue
		}

		// Inside a fenced block, pass through unchanged
		if inFencedBlock {
			result.WriteString(line + "\n")
			continue
		}

		// Check for 4-space indented line
		isIndented := len(line) >= 4 && line[:4] == "    "
		isBlank := strings.TrimSpace(line) == ""

		if isIndented {
			inIndentedBlock = true
			indentedLines = append(indentedLines, line)
		} else if inIndentedBlock && isBlank {
			// Blank line inside indented block - could be continuation
			// Check if next non-blank line is also indented
			nextIndented := false
			for j := i + 1; j < len(lines); j++ {
				nextLine := lines[j]
				if strings.TrimSpace(nextLine) == "" {
					continue
				}
				nextIndented = len(nextLine) >= 4 && nextLine[:4] == "    "
				break
			}
			if nextIndented {
				indentedLines = append(indentedLines, line)
			} else {
				flushIndentedBlock()
				result.WriteString(line + "\n")
			}
		} else {
			flushIndentedBlock()
			result.WriteString(line + "\n")
		}
	}

	// Flush any remaining indented block
	flushIndentedBlock()

	// Remove the trailing newline we added if original didn't have one
	s := result.String()
	if strings.HasSuffix(s, "\n") && !strings.HasSuffix(content, "\n") {
		s = s[:len(s)-1]
	}
	return s
}

// renderMarkdown converts markdown to styled terminal output using glamour.
func renderMarkdown(content string) string {
	if markdownRenderer == nil {
		return content
	}
	// Add "text" language to unlabeled code blocks to prevent auto-detection
	content = addDefaultLanguageToCodeBlocks(content)
	rendered, err := markdownRenderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSuffix(rendered, "\n")
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
