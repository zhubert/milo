package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zhubert/milo/internal/lsp"
)

func TestLSPToolName(t *testing.T) {
	t.Parallel()

	tool := &LSPTool{}
	if tool.Name() != "lsp" {
		t.Errorf("expected name 'lsp', got %q", tool.Name())
	}
}

func TestLSPToolIsParallelSafe(t *testing.T) {
	t.Parallel()

	tool := &LSPTool{}
	if !tool.IsParallelSafe() {
		t.Error("LSP tool should be parallel safe")
	}
}

func TestLSPToolInputSchema(t *testing.T) {
	t.Parallel()

	tool := &LSPTool{}
	schema := tool.InputSchema()

	// Check required fields
	if len(schema.Required) != 1 || schema.Required[0] != "operation" {
		t.Errorf("expected required=[operation], got %v", schema.Required)
	}

	// Check properties exist
	props := schema.Properties.(map[string]any)
	expectedProps := []string{"operation", "file_path", "line", "column", "query"}
	for _, prop := range expectedProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("missing property %q in schema", prop)
		}
	}
}

func TestLSPToolExecuteStatus(t *testing.T) {
	t.Parallel()

	registry := lsp.NewRegistry()
	ctx := context.Background()
	registry.DetectAvailable(ctx)

	manager := lsp.NewManager(registry)
	tool := &LSPTool{
		WorkDir: "/tmp",
		Manager: manager,
	}

	input, _ := json.Marshal(map[string]string{"operation": "status"})
	result, err := tool.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.IsError {
		t.Errorf("status should not return error: %s", result.Output)
	}

	if !strings.Contains(result.Output, "LSP Server Status") {
		t.Errorf("expected status output, got: %s", result.Output)
	}

	// Should list gopls at minimum
	if !strings.Contains(result.Output, "gopls") {
		t.Errorf("expected gopls in status, got: %s", result.Output)
	}
}

func TestLSPToolExecuteInvalidOperation(t *testing.T) {
	t.Parallel()

	registry := lsp.NewRegistry()
	ctx := context.Background()
	registry.DetectAvailable(ctx)

	manager := lsp.NewManager(registry)
	tool := &LSPTool{
		WorkDir: "/tmp",
		Manager: manager,
	}

	input, _ := json.Marshal(map[string]string{"operation": "invalid"})
	result, err := tool.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for invalid operation")
	}

	if !strings.Contains(result.Output, "unknown operation") {
		t.Errorf("expected 'unknown operation' error, got: %s", result.Output)
	}
}

func TestLSPToolExecuteDefinitionMissingFilePath(t *testing.T) {
	t.Parallel()

	registry := lsp.NewRegistry()
	ctx := context.Background()
	registry.DetectAvailable(ctx)

	manager := lsp.NewManager(registry)
	tool := &LSPTool{
		WorkDir: "/tmp",
		Manager: manager,
	}

	input, _ := json.Marshal(map[string]any{
		"operation": "definition",
		"line":      10,
		"column":    5,
	})
	result, err := tool.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing file_path")
	}

	if !strings.Contains(result.Output, "file_path is required") {
		t.Errorf("expected 'file_path is required' error, got: %s", result.Output)
	}
}

func TestLSPToolExecuteDefinitionRelativePath(t *testing.T) {
	t.Parallel()

	registry := lsp.NewRegistry()
	ctx := context.Background()
	registry.DetectAvailable(ctx)

	manager := lsp.NewManager(registry)
	tool := &LSPTool{
		WorkDir: "/tmp",
		Manager: manager,
	}

	input, _ := json.Marshal(map[string]any{
		"operation": "definition",
		"file_path": "relative/path.go",
		"line":      10,
		"column":    5,
	})
	result, err := tool.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for relative path")
	}

	if !strings.Contains(result.Output, "must be an absolute path") {
		t.Errorf("expected 'must be an absolute path' error, got: %s", result.Output)
	}
}

func TestLSPToolExecuteDefinitionInvalidLine(t *testing.T) {
	t.Parallel()

	registry := lsp.NewRegistry()
	ctx := context.Background()
	registry.DetectAvailable(ctx)

	manager := lsp.NewManager(registry)
	tool := &LSPTool{
		WorkDir: "/tmp",
		Manager: manager,
	}

	input, _ := json.Marshal(map[string]any{
		"operation": "definition",
		"file_path": "/path/to/file.go",
		"line":      0,
		"column":    5,
	})
	result, err := tool.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for invalid line")
	}

	if !strings.Contains(result.Output, "line must be >= 1") {
		t.Errorf("expected 'line must be >= 1' error, got: %s", result.Output)
	}
}

func TestLSPToolExecuteSymbolsMissingQuery(t *testing.T) {
	t.Parallel()

	registry := lsp.NewRegistry()
	ctx := context.Background()
	registry.DetectAvailable(ctx)

	manager := lsp.NewManager(registry)
	tool := &LSPTool{
		WorkDir: "/tmp",
		Manager: manager,
	}

	input, _ := json.Marshal(map[string]any{
		"operation": "symbols",
	})
	result, err := tool.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing query")
	}

	if !strings.Contains(result.Output, "query is required") {
		t.Errorf("expected 'query is required' error, got: %s", result.Output)
	}
}

func TestLSPToolExecuteUnsupportedFileType(t *testing.T) {
	t.Parallel()

	registry := lsp.NewRegistry()
	ctx := context.Background()
	registry.DetectAvailable(ctx)

	manager := lsp.NewManager(registry)
	tool := &LSPTool{
		WorkDir: "/tmp",
		Manager: manager,
	}

	input, _ := json.Marshal(map[string]any{
		"operation": "definition",
		"file_path": "/path/to/file.xyz",
		"line":      10,
		"column":    5,
	})
	result, err := tool.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for unsupported file type")
	}

	if !strings.Contains(result.Output, "no language server configured") {
		t.Errorf("expected 'no language server' error, got: %s", result.Output)
	}
}

func TestLSPToolClose(t *testing.T) {
	t.Parallel()

	registry := lsp.NewRegistry()
	manager := lsp.NewManager(registry)
	tool := &LSPTool{
		WorkDir: "/tmp",
		Manager: manager,
	}

	// Close should not error on fresh tool
	if err := tool.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Close with nil manager should not panic
	nilTool := &LSPTool{}
	if err := nilTool.Close(); err != nil {
		t.Errorf("Close with nil manager failed: %v", err)
	}
}
