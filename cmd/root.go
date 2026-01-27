package cmd

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/spf13/cobra"

	"github.com/zhubert/looper/internal/agent"
	"github.com/zhubert/looper/internal/app"
	"github.com/zhubert/looper/internal/permission"
	"github.com/zhubert/looper/internal/tool"
)

var rootCmd = &cobra.Command{
	Use:   "looper",
	Short: "A coding agent powered by Claude",
	RunE:  runTUI,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	registry := tool.NewRegistry()
	tools := []tool.Tool{
		&tool.ReadTool{},
		&tool.WriteTool{},
		&tool.EditTool{},
		&tool.BashTool{WorkDir: workDir},
	}
	for _, t := range tools {
		if err := registry.Register(t); err != nil {
			return fmt.Errorf("registering tool %s: %w", t.Name(), err)
		}
	}

	perms := permission.NewChecker()
	client := anthropic.NewClient()
	ag := agent.New(client, registry, perms, workDir)

	m := app.New(ag, workDir)
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}
