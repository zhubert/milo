package agent

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/zhubert/looper/internal/tool"
)

// BuildSystemPrompt constructs the system prompt for the agent,
// including environment info and available tool descriptions.
func BuildSystemPrompt(workDir string, registry *tool.Registry) string {
	var b strings.Builder

	b.WriteString("You are a coding assistant. You help users with software engineering tasks by reading, writing, and editing files, and by running shell commands.\n\n")

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
	b.WriteString("- Explain what you're doing before using tools.\n")
	b.WriteString("- When a task is complete, summarize what was done.\n")

	return b.String()
}
