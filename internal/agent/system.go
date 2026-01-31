package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/zhubert/milo/internal/tool"
)

// readAgentConfig attempts to read AGENTS.md or CLAUDE.md from the working directory.
// Returns the content if found, or empty string if neither file exists.
func readAgentConfig(workDir string) string {
	// Try AGENTS.md first, then CLAUDE.md
	filenames := []string{"AGENTS.md", "CLAUDE.md"}
	
	for _, filename := range filenames {
		path := filepath.Join(workDir, filename)
		content, err := os.ReadFile(path)
		if err == nil {
			return string(content)
		}
	}
	
	return ""
}

// BuildSystemPrompt constructs the system prompt for the agent,
// including environment info and available tool descriptions.
func BuildSystemPrompt(workDir string, registry *tool.Registry) string {
	var b strings.Builder

	b.WriteString("You are a coding assistant. You help users with software engineering tasks by reading, writing, and editing files, and by running shell commands.\n\n")

	// Read and include agent configuration if available
	agentConfig := readAgentConfig(workDir)
	if agentConfig != "" {
		b.WriteString("## Project Configuration\n\n")
		b.WriteString(agentConfig)
		b.WriteString("\n\n")
	}

	b.WriteString("## Environment\n\n")
	fmt.Fprintf(&b, "- Working directory: %s\n", workDir)
	fmt.Fprintf(&b, "- Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "- Date: %s\n", time.Now().Format("2006-01-02"))
	b.WriteString("\n")

	b.WriteString("## Available Tools\n\n")
	for _, t := range registry.List() {
		fmt.Fprintf(&b, "### %s\n%s\n\n", t.Name(), t.Description())
	}

	b.WriteString("## Guidelines\n\n")
	b.WriteString("- Always read files before editing them.\n")
	b.WriteString("- Use absolute file paths.\n")
	b.WriteString("- Bash commands run in the working directory. Do not use cd; just run commands directly.\n")
	b.WriteString("- When exploring a codebase, use tree to see the full structure instead of multiple list_dir calls.\n")
	b.WriteString("- When reading multiple files, use multi_read instead of multiple read calls.\n")
	b.WriteString("- Explain what you're doing before using tools.\n")
	b.WriteString("- When a task is complete, summarize what was done.\n")

	return b.String()
}
