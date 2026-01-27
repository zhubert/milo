package cmd

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/spf13/cobra"

	"github.com/zhubert/looper/internal/agent"
	"github.com/zhubert/looper/internal/app"
	"github.com/zhubert/looper/internal/logging"
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
	logger, cleanup, err := logging.Setup()
	if err != nil {
		return fmt.Errorf("setting up logging: %w", err)
	}
	defer func() {
		if cerr := cleanup(); cerr != nil {
			fmt.Fprintf(os.Stderr, "closing log file: %v\n", cerr)
		}
	}()

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	logger.Info("starting looper", "work_dir", workDir)

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

	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return errors.New("ANTHROPIC_API_KEY environment variable is not set. " +
			"Get an API key at https://console.anthropic.com/ and export it:\n\n" +
			"  export ANTHROPIC_API_KEY=sk-ant-...")
	}

	client := anthropic.NewClient()
	ag := agent.New(client, registry, perms, workDir, logger)

	m := app.New(ag, workDir)
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}
