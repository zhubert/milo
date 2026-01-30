package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zhubert/milo/internal/session"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List saved sessions",
	Long:  `List all saved conversation sessions for the current project.`,
	RunE:  runSessions,
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
}

func runSessions(cmd *cobra.Command, args []string) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	store, err := session.StoreForWorkDir(workDir)
	if err != nil {
		return fmt.Errorf("opening session store: %w", err)
	}

	summaries, err := store.List()
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(summaries) == 0 {
		fmt.Println("No saved sessions.")
		return nil
	}

	fmt.Printf("%-10s  %-20s  %-8s  %s\n", "ID", "UPDATED", "MESSAGES", "TITLE")
	fmt.Println("─────────────────────────────────────────────────────────────────────")

	for _, s := range summaries {
		title := s.Title
		if title == "" {
			title = "(untitled)"
		}
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		fmt.Printf("%-10s  %-20s  %-8d  %s\n",
			s.ID,
			formatTime(s.UpdatedAt),
			s.MessageCount,
			title,
		)
	}

	fmt.Println()
	fmt.Println("Resume a session with: milo --resume <id>")
	fmt.Println("Resume the most recent: milo --resume last")

	return nil
}

func formatTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return t.Format("Jan 2, 2006")
	}
}
