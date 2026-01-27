package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "looper",
	Short: "A coding agent powered by Claude",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("looper")
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
