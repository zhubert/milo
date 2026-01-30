package cmd

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/spf13/cobra"

	"github.com/zhubert/milo/internal/agent"
	"github.com/zhubert/milo/internal/app"
	"github.com/zhubert/milo/internal/logging"
	"github.com/zhubert/milo/internal/permission"
	"github.com/zhubert/milo/internal/session"
	"github.com/zhubert/milo/internal/tool"
	"github.com/zhubert/milo/internal/version"
)

var (
	resumeFlag string
	newSession bool
	modelFlag  string
)

var rootCmd = &cobra.Command{
	Use:     "milo",
	Short:   "A coding agent powered by Claude",
	Version: version.Version,
	RunE:    runTUI,
}

func init() {
	rootCmd.Flags().StringVar(&resumeFlag, "resume", "", "resume a previous session by ID (or 'last' for most recent)")
	rootCmd.Flags().BoolVar(&newSession, "new", false, "start a new session (ignore any existing session)")
	rootCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Claude model to use (e.g., claude-sonnet-4-20250514, claude-opus-4-5-20251101)")
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

	logger.Info("starting milo", "work_dir", workDir)

	registry := tool.NewRegistry()
	tools := []tool.Tool{
		&tool.ReadTool{},
		&tool.WriteTool{},
		&tool.EditTool{},
		&tool.MoveTool{},
		&tool.UndoTool{},
		&tool.BashTool{WorkDir: workDir},
		&tool.GlobTool{WorkDir: workDir},
		&tool.GrepTool{WorkDir: workDir},
		&tool.ListDirTool{WorkDir: workDir},
		&tool.WebFetchTool{},
		&tool.WebSearchTool{},
	}
	for _, t := range tools {
		if err := registry.Register(t); err != nil {
			return fmt.Errorf("registering tool %s: %w", t.Name(), err)
		}
	}

	perms, err := permission.NewCheckerWithConfig(workDir)
	if err != nil {
		return fmt.Errorf("setting up permissions: %w", err)
	}

	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return errors.New("ANTHROPIC_API_KEY environment variable is not set. " +
			"Get an API key at https://console.anthropic.com/ and export it:\n\n" +
			"  export ANTHROPIC_API_KEY=sk-ant-...")
	}

	// Set up session store in project's .milo directory.
	store, err := session.StoreForWorkDir(workDir)
	if err != nil {
		return fmt.Errorf("setting up session store: %w", err)
	}

	// Determine which session to use.
	var sess *session.Session
	if !newSession {
		if resumeFlag != "" {
			// Resume a specific session.
			if resumeFlag == "last" {
				sess, err = store.MostRecent()
				if err != nil {
					return fmt.Errorf("loading most recent session: %w", err)
				}
			} else {
				sess, err = store.Load(resumeFlag)
				if err != nil {
					return fmt.Errorf("loading session %q: %w", resumeFlag, err)
				}
			}
		}
	}

	// Create a new session if we don't have one.
	if sess == nil {
		sess, err = session.NewSession()
		if err != nil {
			return fmt.Errorf("creating new session: %w", err)
		}
		logger.Info("created new session", "id", sess.ID)
	} else {
		logger.Info("resumed session", "id", sess.ID, "messages", sess.MessageCount())
	}

	client := anthropic.NewClient()

	model := agent.DefaultModel
	if modelFlag != "" {
		model = anthropic.Model(modelFlag)
	}

	ag := agent.New(client, registry, perms, workDir, logger, model)

	m := app.New(ag, workDir, store, sess)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}
