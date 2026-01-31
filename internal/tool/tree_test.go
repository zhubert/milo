package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeToolExecute(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a simple directory structure
	subDir := filepath.Join(dir, "src")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0644); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("writing main.go: %v", err)
	}

	tool := &TreeTool{WorkDir: dir}

	t.Run("basic tree output", func(t *testing.T) {
		t.Parallel()

		input := treeInput{Path: dir}
		inputJSON, _ := json.Marshal(input)

		result, err := tool.Execute(context.Background(), inputJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Errorf("unexpected IsError: %s", result.Output)
		}

		// Should show root directory name
		if !strings.HasPrefix(result.Output, filepath.Base(dir)+"/") {
			t.Errorf("output should start with root dir name, got: %s", result.Output)
		}

		// Should show subdirectory
		if !strings.Contains(result.Output, "src/") {
			t.Errorf("output should contain 'src/', got: %s", result.Output)
		}

		// Should show files
		if !strings.Contains(result.Output, "README.md") {
			t.Errorf("output should contain 'README.md', got: %s", result.Output)
		}
		if !strings.Contains(result.Output, "main.go") {
			t.Errorf("output should contain 'main.go', got: %s", result.Output)
		}
	})

	t.Run("uses workdir when no path provided", func(t *testing.T) {
		t.Parallel()

		result, err := tool.Execute(context.Background(), []byte("{}"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Errorf("unexpected IsError: %s", result.Output)
		}

		// Should use WorkDir
		if !strings.Contains(result.Output, "src/") {
			t.Errorf("output should contain 'src/' when using workdir, got: %s", result.Output)
		}
	})
}

func TestTreeDepthLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create nested directory structure
	currentDir := dir
	for i := 0; i < 12; i++ {
		currentDir = filepath.Join(currentDir, "level"+string(rune('a'+i)))
		if err := os.Mkdir(currentDir, 0755); err != nil {
			t.Fatalf("creating nested dir: %v", err)
		}
	}

	tool := &TreeTool{WorkDir: dir}

	t.Run("default depth is 4", func(t *testing.T) {
		t.Parallel()

		input := treeInput{Path: dir}
		inputJSON, _ := json.Marshal(input)

		result, err := tool.Execute(context.Background(), inputJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should show levels a, b, c, d (depth 4)
		if !strings.Contains(result.Output, "levela/") {
			t.Errorf("should show levela")
		}
		if !strings.Contains(result.Output, "levelb/") {
			t.Errorf("should show levelb")
		}
		if !strings.Contains(result.Output, "levelc/") {
			t.Errorf("should show levelc")
		}
		if !strings.Contains(result.Output, "leveld/") {
			t.Errorf("should show leveld")
		}
		// Should not show deeper levels
		if strings.Contains(result.Output, "levele/") {
			t.Errorf("should not show levele with default depth")
		}
	})

	t.Run("custom depth", func(t *testing.T) {
		t.Parallel()

		input := treeInput{Path: dir, Depth: 2}
		inputJSON, _ := json.Marshal(input)

		result, err := tool.Execute(context.Background(), inputJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should show levels a, b (depth 2)
		if !strings.Contains(result.Output, "levela/") {
			t.Errorf("should show levela")
		}
		if !strings.Contains(result.Output, "levelb/") {
			t.Errorf("should show levelb")
		}
		// Should not show deeper
		if strings.Contains(result.Output, "levelc/") {
			t.Errorf("should not show levelc with depth 2")
		}
	})

	t.Run("max depth cap at 10", func(t *testing.T) {
		t.Parallel()

		input := treeInput{Path: dir, Depth: 100}
		inputJSON, _ := json.Marshal(input)

		result, err := tool.Execute(context.Background(), inputJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should show up to level j (10 levels deep)
		if !strings.Contains(result.Output, "levelj/") {
			t.Errorf("should show up to levelj (depth 10)")
		}
		// Should not show beyond
		if strings.Contains(result.Output, "levelk/") {
			t.Errorf("should not show levelk (beyond max depth)")
		}
	})
}

func TestTreeSkippedDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create directories that should be skipped
	skippedDirs := []string{".git", "node_modules", "vendor", "__pycache__", ".venv"}
	for _, d := range skippedDirs {
		if err := os.Mkdir(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("creating %s: %v", d, err)
		}
		// Add a file inside to verify the whole dir is skipped
		if err := os.WriteFile(filepath.Join(dir, d, "file.txt"), []byte("x"), 0644); err != nil {
			t.Fatalf("writing file in %s: %v", d, err)
		}
	}

	// Create a directory that should NOT be skipped
	if err := os.Mkdir(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatalf("creating src: %v", err)
	}

	tool := &TreeTool{WorkDir: dir}

	input := treeInput{Path: dir}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Skipped directories should not appear
	for _, d := range skippedDirs {
		if strings.Contains(result.Output, d+"/") {
			t.Errorf("should not contain skipped directory %q", d)
		}
	}

	// Normal directories should appear
	if !strings.Contains(result.Output, "src/") {
		t.Errorf("should contain non-skipped directory 'src'")
	}
}

func TestTreeHiddenFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create hidden files and directories
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing .hidden: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".hidden_dir"), 0755); err != nil {
		t.Fatalf("creating .hidden_dir: %v", err)
	}

	// Create exceptions that SHOULD be shown
	if err := os.Mkdir(filepath.Join(dir, ".github"), 0755); err != nil {
		t.Fatalf("creating .github: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}

	// Create normal file for comparison
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing visible.txt: %v", err)
	}

	tool := &TreeTool{WorkDir: dir}

	input := treeInput{Path: dir}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Regular hidden files/dirs should be excluded
	if strings.Contains(result.Output, ".hidden_dir") {
		t.Error("should not show .hidden_dir")
	}
	// Note: .hidden file check - the tree skips hidden except .github/.gitignore
	// Let's verify exceptions are shown
	if !strings.Contains(result.Output, ".github") {
		t.Error("should show .github (exception)")
	}
	if !strings.Contains(result.Output, ".gitignore") {
		t.Error("should show .gitignore (exception)")
	}

	// Normal files should be shown
	if !strings.Contains(result.Output, "visible.txt") {
		t.Error("should show visible.txt")
	}
}

func TestTreeSorting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create mix of files and directories
	if err := os.Mkdir(filepath.Join(dir, "beta_dir"), 0755); err != nil {
		t.Fatalf("creating beta_dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "alpha_dir"), 0755); err != nil {
		t.Fatalf("creating alpha_dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "charlie.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing charlie.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "able.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing able.txt: %v", err)
	}

	tool := &TreeTool{WorkDir: dir}

	input := treeInput{Path: dir}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Directories should come before files
	dirIdx := strings.Index(result.Output, "alpha_dir/")
	fileIdx := strings.Index(result.Output, "able.txt")
	if dirIdx == -1 || fileIdx == -1 {
		t.Fatalf("expected both alpha_dir and able.txt in output")
	}
	if dirIdx > fileIdx {
		t.Error("directories should appear before files")
	}

	// Directories should be alphabetically sorted
	alphaIdx := strings.Index(result.Output, "alpha_dir/")
	betaIdx := strings.Index(result.Output, "beta_dir/")
	if alphaIdx > betaIdx {
		t.Error("alpha_dir should appear before beta_dir")
	}

	// Files should be alphabetically sorted
	ableIdx := strings.Index(result.Output, "able.txt")
	charlieIdx := strings.Index(result.Output, "charlie.txt")
	if ableIdx > charlieIdx {
		t.Error("able.txt should appear before charlie.txt")
	}
}

func TestTreeMaxEntries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create 600 files to exceed max entries (500)
	for i := 0; i < 600; i++ {
		filename := filepath.Join(dir, "file"+string(rune('a'+i/26))+string(rune('a'+i%26))+".txt")
		if err := os.WriteFile(filename, []byte("x"), 0644); err != nil {
			t.Fatalf("writing file %d: %v", i, err)
		}
	}

	tool := &TreeTool{WorkDir: dir}

	input := treeInput{Path: dir, Depth: 1}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have truncation message
	if !strings.Contains(result.Output, "truncated") {
		t.Error("output should contain truncation message")
	}
	if !strings.Contains(result.Output, "500") {
		t.Error("truncation message should mention 500 entries")
	}
}

func TestTreeErrorCases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tests := []struct {
		name    string
		tool    *TreeTool
		input   treeInput
		errMsg  string
	}{
		{
			name:   "relative path",
			tool:   &TreeTool{WorkDir: dir},
			input:  treeInput{Path: "relative/path"},
			errMsg: "must be an absolute path",
		},
		{
			name:   "path is a file not directory",
			tool:   &TreeTool{WorkDir: dir},
			input:  treeInput{Path: testFile},
			errMsg: "not a directory",
		},
		{
			name:   "nonexistent path",
			tool:   &TreeTool{WorkDir: dir},
			input:  treeInput{Path: filepath.Join(dir, "nonexistent")},
			errMsg: "error accessing path",
		},
		{
			name:   "no workdir and no path",
			tool:   &TreeTool{WorkDir: ""},
			input:  treeInput{},
			errMsg: "no working directory set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputJSON, _ := json.Marshal(tt.input)

			result, err := tt.tool.Execute(context.Background(), inputJSON)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Error("expected IsError to be true")
			}
			if !strings.Contains(result.Output, tt.errMsg) {
				t.Errorf("output should contain %q, got: %s", tt.errMsg, result.Output)
			}
		})
	}
}

func TestTreeToolMetadata(t *testing.T) {
	t.Parallel()

	tool := &TreeTool{}

	if tool.Name() != "tree" {
		t.Errorf("Name: got %q, want %q", tool.Name(), "tree")
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	if !tool.IsParallelSafe() {
		t.Error("TreeTool should be parallel safe")
	}

	schema := tool.InputSchema()
	if schema.Properties == nil {
		t.Error("InputSchema should have properties")
	}
}

func TestTreeInvalidJSON(t *testing.T) {
	t.Parallel()

	tool := &TreeTool{WorkDir: t.TempDir()}

	result, err := tool.Execute(context.Background(), []byte("invalid json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parsing tree input") {
		t.Errorf("expected parsing error, got: %v", err)
	}
	_ = result
}

func TestTreeConnectors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create structure to test tree connectors
	if err := os.WriteFile(filepath.Join(dir, "first.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing first.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("writing last.txt: %v", err)
	}

	tool := &TreeTool{WorkDir: dir}

	input := treeInput{Path: dir}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have tree connectors
	if !strings.Contains(result.Output, "├── ") && !strings.Contains(result.Output, "└── ") {
		t.Error("output should contain tree connectors")
	}
}

func TestShouldSkip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{name: ".git", input: ".git", expect: true},
		{name: "node_modules", input: "node_modules", expect: true},
		{name: "vendor", input: "vendor", expect: true},
		{name: "__pycache__", input: "__pycache__", expect: true},
		{name: ".venv", input: ".venv", expect: true},
		{name: "venv", input: "venv", expect: true},
		{name: ".idea", input: ".idea", expect: true},
		{name: ".vscode", input: ".vscode", expect: true},
		{name: "dist", input: "dist", expect: true},
		{name: "build", input: "build", expect: true},
		{name: ".next", input: ".next", expect: true},
		{name: ".nuxt", input: ".nuxt", expect: true},
		{name: "target", input: "target", expect: true},
		{name: "src (not skipped)", input: "src", expect: false},
		{name: "lib (not skipped)", input: "lib", expect: false},
		{name: "internal (not skipped)", input: "internal", expect: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldSkip(tt.input); got != tt.expect {
				t.Errorf("shouldSkip(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}
