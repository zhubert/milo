package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/spf13/cobra"

	"github.com/zhubert/looper/internal/agent"
	"github.com/zhubert/looper/internal/tool"
)

var rootCmd = &cobra.Command{
	Use:   "looper",
	Short: "A coding agent powered by Claude",
	RunE:  runCLI,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCLI(cmd *cobra.Command, args []string) error {
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

	client := anthropic.NewClient()
	ag := agent.New(client, registry, workDir)

	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("looper — type a message (ctrl+d to quit)")
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}

		msg := strings.TrimSpace(scanner.Text())
		if msg == "" {
			continue
		}

		ch := ag.SendMessage(ctx, msg)
		for chunk := range ch {
			switch chunk.Type {
			case agent.ChunkText:
				fmt.Print(chunk.Text)
			case agent.ChunkToolUse:
				fmt.Printf("\n[tool: %s]\n", chunk.ToolName)
			case agent.ChunkToolResult:
				if chunk.Result != nil {
					prefix := "result"
					if chunk.Result.IsError {
						prefix = "error"
					}
					output := chunk.Result.Output
					if len(output) > 500 {
						output = output[:500] + "..."
					}
					fmt.Printf("[%s: %s]\n", prefix, output)
				}
			case agent.ChunkDone:
				fmt.Println()
			case agent.ChunkError:
				fmt.Fprintf(os.Stderr, "\nerror: %v\n", chunk.Err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	return nil
}
