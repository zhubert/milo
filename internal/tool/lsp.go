package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/zhubert/milo/internal/lsp"
)

// LSPTool provides code navigation using language servers.
type LSPTool struct {
	WorkDir string
	Manager *lsp.Manager
}

type lspInput struct {
	Operation string `json:"operation"`
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Query     string `json:"query"`
}

// Name returns the tool name.
func (t *LSPTool) Name() string { return "lsp" }

// Description returns a description of the tool.
func (t *LSPTool) Description() string {
	return "Code navigation using language servers (gopls, typescript-language-server, rust-analyzer, etc.). " +
		"Operations: 'definition' (go to definition), 'references' (find all references), " +
		"'hover' (get type info and docs), 'symbols' (search for symbols by name), " +
		"'status' (show available language servers)."
}

// InputSchema returns the JSON schema for the tool input.
func (t *LSPTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"definition", "references", "hover", "symbols", "status"},
				"description": "The LSP operation to perform",
			},
			"file_path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the file (required for definition, references, hover)",
			},
			"line": map[string]any{
				"type":        "integer",
				"description": "1-indexed line number in the file",
			},
			"column": map[string]any{
				"type":        "integer",
				"description": "1-indexed column number in the file",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Symbol name to search for (required for symbols operation)",
			},
		},
		Required: []string{"operation"},
	}
}

// IsParallelSafe returns true because LSP operations are read-only.
func (t *LSPTool) IsParallelSafe() bool { return true }

// Execute runs the LSP operation.
func (t *LSPTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in lspInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing lsp input: %w", err)
	}

	switch in.Operation {
	case "status":
		return t.executeStatus()
	case "definition":
		return t.executeDefinition(ctx, in)
	case "references":
		return t.executeReferences(ctx, in)
	case "hover":
		return t.executeHover(ctx, in)
	case "symbols":
		return t.executeSymbols(ctx, in)
	default:
		return Result{
			Output:  fmt.Sprintf("unknown operation %q. Valid operations: definition, references, hover, symbols, status", in.Operation),
			IsError: true,
		}, nil
	}
}

func (t *LSPTool) validatePositionInput(in lspInput) *Result {
	if in.FilePath == "" {
		return &Result{Output: "file_path is required for " + in.Operation, IsError: true}
	}
	if !filepath.IsAbs(in.FilePath) {
		return &Result{Output: "file_path must be an absolute path", IsError: true}
	}
	if in.Line < 1 {
		return &Result{Output: "line must be >= 1", IsError: true}
	}
	if in.Column < 1 {
		return &Result{Output: "column must be >= 1", IsError: true}
	}
	return nil
}

func (t *LSPTool) executeStatus() (Result, error) {
	registry := t.Manager.Registry()

	var sb strings.Builder
	sb.WriteString("LSP Server Status:\n")

	for _, server := range registry.Servers() {
		if server.IsAvailable() {
			sb.WriteString(fmt.Sprintf("  [available] %s (%s)\n",
				server.Name, strings.Join(server.Languages, ", ")))
		} else {
			sb.WriteString(fmt.Sprintf("  [not installed] %s (%s)\n",
				server.Name, strings.Join(server.Languages, ", ")))
			if server.InstallHint != "" {
				sb.WriteString(fmt.Sprintf("    Install with: %s\n", server.InstallHint))
			}
		}
	}

	active := t.Manager.ActiveClients()
	if active > 0 {
		sb.WriteString(fmt.Sprintf("\nActive server connections: %d\n", active))
	}

	return Result{Output: sb.String()}, nil
}

func (t *LSPTool) executeDefinition(ctx context.Context, in lspInput) (Result, error) {
	if errResult := t.validatePositionInput(in); errResult != nil {
		return *errResult, nil
	}

	client, err := t.Manager.GetClient(ctx, in.FilePath)
	if err != nil {
		return Result{Output: err.Error(), IsError: true}, nil
	}

	locations, err := client.Definition(ctx, in.FilePath, in.Line, in.Column)
	if err != nil {
		return Result{Output: fmt.Sprintf("definition lookup failed: %v", err), IsError: true}, nil
	}

	if len(locations) == 0 {
		return Result{Output: "No definition found at the specified position"}, nil
	}

	return t.formatLocations("Definition", client.ServerName(), locations)
}

func (t *LSPTool) executeReferences(ctx context.Context, in lspInput) (Result, error) {
	if errResult := t.validatePositionInput(in); errResult != nil {
		return *errResult, nil
	}

	client, err := t.Manager.GetClient(ctx, in.FilePath)
	if err != nil {
		return Result{Output: err.Error(), IsError: true}, nil
	}

	locations, err := client.References(ctx, in.FilePath, in.Line, in.Column)
	if err != nil {
		return Result{Output: fmt.Sprintf("references lookup failed: %v", err), IsError: true}, nil
	}

	if len(locations) == 0 {
		return Result{Output: "No references found at the specified position"}, nil
	}

	return t.formatLocations("References", client.ServerName(), locations)
}

func (t *LSPTool) executeHover(ctx context.Context, in lspInput) (Result, error) {
	if errResult := t.validatePositionInput(in); errResult != nil {
		return *errResult, nil
	}

	client, err := t.Manager.GetClient(ctx, in.FilePath)
	if err != nil {
		return Result{Output: err.Error(), IsError: true}, nil
	}

	hover, err := client.Hover(ctx, in.FilePath, in.Line, in.Column)
	if err != nil {
		return Result{Output: fmt.Sprintf("hover lookup failed: %v", err), IsError: true}, nil
	}

	if hover == nil || hover.Contents.Value == "" {
		return Result{Output: "No hover information at the specified position"}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Hover info (using %s):\n", client.ServerName()))
	sb.WriteString(fmt.Sprintf("File: %s:%d:%d\n\n", in.FilePath, in.Line, in.Column))
	sb.WriteString(hover.Contents.Value)

	return Result{Output: sb.String()}, nil
}

func (t *LSPTool) executeSymbols(ctx context.Context, in lspInput) (Result, error) {
	if in.Query == "" {
		return Result{Output: "query is required for symbols operation", IsError: true}, nil
	}

	// Use file_path to determine which server to use, or fall back to work dir
	targetFile := in.FilePath
	if targetFile == "" {
		// Try to find a suitable file in the work directory
		targetFile = t.findSuitableFile()
		if targetFile == "" {
			return Result{
				Output:  "file_path is required to determine which language server to use, or no supported files found in workspace",
				IsError: true,
			}, nil
		}
	}

	client, err := t.Manager.GetClient(ctx, targetFile)
	if err != nil {
		return Result{Output: err.Error(), IsError: true}, nil
	}

	symbols, err := client.WorkspaceSymbols(ctx, in.Query)
	if err != nil {
		return Result{Output: fmt.Sprintf("symbol search failed: %v", err), IsError: true}, nil
	}

	if len(symbols) == 0 {
		return Result{Output: fmt.Sprintf("No symbols found matching %q", in.Query)}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Symbols matching %q (using %s):\n\n", in.Query, client.ServerName()))

	for i, sym := range symbols {
		if i >= 50 {
			sb.WriteString(fmt.Sprintf("\n... and %d more results\n", len(symbols)-50))
			break
		}

		filePath, err := lsp.URIToFile(sym.Location.URI)
		if err != nil {
			filePath = sym.Location.URI
		}
		line, col := lsp.FromLSPPosition(sym.Location.Range.Start)

		sb.WriteString(fmt.Sprintf("%s (%s)\n", sym.Name, sym.Kind.String()))
		sb.WriteString(fmt.Sprintf("  %s:%d:%d\n", filePath, line, col))
		if sym.ContainerName != "" {
			sb.WriteString(fmt.Sprintf("  in %s\n", sym.ContainerName))
		}
	}

	return Result{Output: sb.String()}, nil
}

func (t *LSPTool) formatLocations(title, serverName string, locations []lsp.Location) (Result, error) {
	var sb strings.Builder

	if len(locations) == 1 {
		sb.WriteString(fmt.Sprintf("%s found (using %s):\n", title, serverName))
	} else {
		sb.WriteString(fmt.Sprintf("%d %s found (using %s):\n", len(locations), strings.ToLower(title), serverName))
	}

	for i, loc := range locations {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("\n... and %d more results\n", len(locations)-20))
			break
		}

		filePath, err := lsp.URIToFile(loc.URI)
		if err != nil {
			filePath = loc.URI
		}
		line, col := lsp.FromLSPPosition(loc.Range.Start)

		sb.WriteString(fmt.Sprintf("\n%s:%d:%d\n", filePath, line, col))

		// Try to read context around the location
		context := t.readContext(filePath, line)
		if context != "" {
			sb.WriteString("\nContext:\n")
			sb.WriteString(context)
		}
	}

	return Result{Output: sb.String()}, nil
}

func (t *LSPTool) readContext(filePath string, targetLine int) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	startLine := targetLine - 2
	endLine := targetLine + 2

	for scanner.Scan() {
		lineNum++
		if lineNum >= startLine && lineNum <= endLine {
			lines = append(lines, scanner.Text())
		}
		if lineNum > endLine {
			break
		}
	}

	if len(lines) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, line := range lines {
		actualLine := startLine + i
		if actualLine < 1 {
			continue
		}
		prefix := "   "
		if actualLine == targetLine {
			prefix = "->"
		}
		sb.WriteString(fmt.Sprintf("%s %4d | %s\n", prefix, actualLine, line))
	}

	return sb.String()
}

func (t *LSPTool) findSuitableFile() string {
	// Walk the work directory looking for a supported file
	registry := t.Manager.Registry()
	var extensions []string
	for _, server := range registry.Servers() {
		if server.IsAvailable() {
			extensions = append(extensions, server.Extensions...)
		}
	}

	if len(extensions) == 0 {
		return ""
	}

	var found string
	_ = filepath.Walk(t.WorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		for _, e := range extensions {
			if ext == e {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})

	return found
}

// Close cleans up the LSP manager.
func (t *LSPTool) Close() error {
	if t.Manager != nil {
		return t.Manager.CloseAll()
	}
	return nil
}
