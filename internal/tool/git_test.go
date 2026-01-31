package tool

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitTool_Name(t *testing.T) {
	tool := &GitTool{}
	if got := tool.Name(); got != "git" {
		t.Errorf("Name() = %q, want %q", got, "git")
	}
}

func TestGitTool_Description(t *testing.T) {
	tool := &GitTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
	if !strings.Contains(desc, "git operations") {
		t.Error("Description() should mention git operations")
	}
}

func TestGitTool_InputSchema(t *testing.T) {
	tool := &GitTool{}
	schema := tool.InputSchema()
	
	// Check required fields
	required := schema.Required
	if len(required) != 1 || required[0] != "operation" {
		t.Errorf("Required fields = %v, want [operation]", required)
	}
	
	// Check properties exist - just verify they are not nil
	props, ok := schema.Properties.(map[string]any)
	if !ok {
		t.Fatal("Properties should be a map[string]any")
	}
	if props["operation"] == nil {
		t.Error("operation property should exist")
	}
	if props["args"] == nil {
		t.Error("args property should exist")
	}
}

func TestGitTool_ValidateGitRepo(t *testing.T) {
	// Test with non-git directory
	tempDir := t.TempDir()
	tool := &GitTool{WorkDir: tempDir}
	
	err := tool.validateGitRepo()
	if err == nil {
		t.Error("validateGitRepo() should return error for non-git directory")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error message should mention git repository, got: %v", err)
	}
	
	// Test with git directory
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("creating .git directory: %v", err)
	}
	
	err = tool.validateGitRepo()
	if err != nil {
		t.Errorf("validateGitRepo() should not return error for git directory: %v", err)
	}
}

func TestGitTool_ParseGitStatus(t *testing.T) {
	tool := &GitTool{}
	
	tests := []struct {
		name           string
		porcelainOutput string
		expectedBranch string
		expectedClean  bool
		expectedAhead  int
		expectedBehind int
	}{
		{
			name:           "clean repository",
			porcelainOutput: "## main",
			expectedBranch: "main",
			expectedClean:  true,
			expectedAhead:  0,
			expectedBehind: 0,
		},
		{
			name: "repository with changes",
			porcelainOutput: `## main
 M file1.go
?? file2.go`,
			expectedBranch: "main",
			expectedClean:  false,
		},
		{
			name:           "repository ahead and behind",
			porcelainOutput: "## main...origin/main [ahead 2, behind 1]",
			expectedBranch: "main",
			expectedClean:  true,
			expectedAhead:  2,
			expectedBehind: 1,
		},
		{
			name:           "repository only ahead",
			porcelainOutput: "## main...origin/main [ahead 3]",
			expectedBranch: "main",
			expectedClean:  true,
			expectedAhead:  3,
			expectedBehind: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tool.parseGitStatus(tt.porcelainOutput)
			
			if status.Branch != tt.expectedBranch {
				t.Errorf("Branch = %q, want %q", status.Branch, tt.expectedBranch)
			}
			if status.IsClean != tt.expectedClean {
				t.Errorf("IsClean = %v, want %v", status.IsClean, tt.expectedClean)
			}
			if status.Ahead != tt.expectedAhead {
				t.Errorf("Ahead = %d, want %d", status.Ahead, tt.expectedAhead)
			}
			if status.Behind != tt.expectedBehind {
				t.Errorf("Behind = %d, want %d", status.Behind, tt.expectedBehind)
			}
		})
	}
}

func TestGitTool_ParseGitStatus_FileStates(t *testing.T) {
	tool := &GitTool{}
	
	porcelainOutput := `## main
A  staged.go
 M modified.go
?? untracked.go
 D deleted.go
UU conflicted.go`
	
	status := tool.parseGitStatus(porcelainOutput)
	
	if len(status.Staged) != 1 || status.Staged[0] != "staged.go" {
		t.Errorf("Staged = %v, want [staged.go]", status.Staged)
	}
	if len(status.Modified) != 2 || !contains(status.Modified, "modified.go") || !contains(status.Modified, "conflicted.go") {
		t.Errorf("Modified = %v, want containing modified.go and conflicted.go", status.Modified)
	}
	if len(status.Untracked) != 1 || status.Untracked[0] != "untracked.go" {
		t.Errorf("Untracked = %v, want [untracked.go]", status.Untracked)
	}
	if len(status.Deleted) != 1 || status.Deleted[0] != "deleted.go" {
		t.Errorf("Deleted = %v, want [deleted.go]", status.Deleted)
	}
	if !status.HasConflicts {
		t.Error("HasConflicts should be true")
	}
	if status.IsClean {
		t.Error("IsClean should be false")
	}
}

func TestGitTool_Execute_InvalidInput(t *testing.T) {
	tool := &GitTool{}
	
	_, err := tool.Execute(context.Background(), json.RawMessage(`{invalid json}`))
	if err == nil {
		t.Error("Execute() should return error for invalid JSON")
	}
}

func TestGitTool_Execute_UnsupportedOperation(t *testing.T) {
	tempDir := t.TempDir()
	setupGitRepo(t, tempDir)
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "unsupported"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if !result.IsError {
		t.Error("Result should be an error")
	}
	if !strings.Contains(result.Output, "unsupported git operation") {
		t.Errorf("Output should mention unsupported operation, got: %s", result.Output)
	}
}

func TestGitTool_Execute_NonGitDirectory(t *testing.T) {
	tempDir := t.TempDir()
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "status"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if !result.IsError {
		t.Error("Result should be an error")
	}
	if !strings.Contains(result.Output, "not a git repository") {
		t.Errorf("Output should mention not a git repository, got: %s", result.Output)
	}
}

func TestGitTool_Execute_Status(t *testing.T) {
	tempDir := t.TempDir()
	setupGitRepo(t, tempDir)
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "status"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if result.IsError {
		t.Errorf("Result should not be an error: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Git Status") {
		t.Errorf("Output should contain git status info, got: %s", result.Output)
	}
}

func TestGitTool_Execute_CommitWithoutMessage(t *testing.T) {
	tempDir := t.TempDir()
	setupGitRepo(t, tempDir)
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "commit"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if !result.IsError {
		t.Error("Result should be an error for commit without message")
	}
	if !strings.Contains(result.Output, "commit message is required") {
		t.Errorf("Output should mention required commit message, got: %s", result.Output)
	}
}

func TestGitTool_Execute_DangerousPushWithoutForce(t *testing.T) {
	tempDir := t.TempDir()
	setupGitRepo(t, tempDir)
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "push", "args": ["--force"]}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if !result.IsError {
		t.Error("Result should be an error for dangerous push without force=true")
	}
	if !strings.Contains(result.Output, "Dangerous push operation") {
		t.Errorf("Output should mention dangerous operation, got: %s", result.Output)
	}
}

func TestGitTool_Execute_DangerousResetWithoutForce(t *testing.T) {
	tempDir := t.TempDir()
	setupGitRepo(t, tempDir)
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "reset", "args": ["--hard"]}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if !result.IsError {
		t.Error("Result should be an error for dangerous reset without force=true")
	}
	if !strings.Contains(result.Output, "Dangerous reset operation") {
		t.Errorf("Output should mention dangerous operation, got: %s", result.Output)
	}
}

func TestGitTool_Execute_CloneWithoutRepo(t *testing.T) {
	tool := &GitTool{}
	
	input := json.RawMessage(`{"operation": "clone"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if !result.IsError {
		t.Error("Result should be an error for clone without repository URL")
	}
	if !strings.Contains(result.Output, "clone requires a repository URL") {
		t.Errorf("Output should mention required repository URL, got: %s", result.Output)
	}
}

func TestGitTool_Execute_Branch(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available in test environment")
	}
	
	tempDir := t.TempDir()
	setupGitRepo(t, tempDir)
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "branch"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if result.IsError {
		t.Errorf("Result should not be an error: %s", result.Output)
	}
	// Should show current branch (main or master typically)
	if !strings.Contains(result.Output, "main") && !strings.Contains(result.Output, "master") {
		t.Logf("Branch output: %s", result.Output) // Log for debugging
	}
}

func TestGitTool_Execute_Log(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available in test environment")
	}
	
	tempDir := t.TempDir()
	setupGitRepoWithCommit(t, tempDir)
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "log"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if result.IsError {
		t.Errorf("Result should not be an error: %s", result.Output)
	}
	// Should contain commit information
	if !strings.Contains(result.Output, "Initial commit") {
		t.Logf("Log output: %s", result.Output) // Log for debugging
	}
}

func TestGitTool_Execute_Add(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available in test environment")
	}
	
	tempDir := t.TempDir()
	setupGitRepo(t, tempDir)
	
	// Create a file to add
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}
	
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "add", "path": "test.txt"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if result.IsError {
		t.Errorf("Result should not be an error: %s", result.Output)
	}
}

func TestGitTool_Execute_Diff(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available in test environment")
	}
	
	tempDir := t.TempDir()
	setupGitRepoWithCommit(t, tempDir)
	
	// Modify a file
	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("Modified content"), 0644); err != nil {
		t.Fatalf("modifying test file: %v", err)
	}
	
	tool := &GitTool{WorkDir: tempDir}
	
	input := json.RawMessage(`{"operation": "diff"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() should not return error: %v", err)
	}
	
	if result.IsError {
		t.Errorf("Result should not be an error: %s", result.Output)
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		slice    []string
		item     string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
		{[]string{"--force", "origin", "main"}, "--force", true},
	}
	
	for _, tt := range tests {
		result := contains(tt.slice, tt.item)
		if result != tt.expected {
			t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.item, result, tt.expected)
		}
	}
}

// Test helpers

func setupGitRepo(t *testing.T, dir string) {
	t.Helper()
	
	if !isGitAvailable() {
		// Create minimal .git directory for validation tests
		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("creating .git directory: %v", err)
		}
		return
	}
	
	// Initialize real git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	
	// Configure git user (required for commits)
	configUser := exec.Command("git", "config", "user.name", "Test User")
	configUser.Dir = dir
	if err := configUser.Run(); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}
	
	configEmail := exec.Command("git", "config", "user.email", "test@example.com")
	configEmail.Dir = dir
	if err := configEmail.Run(); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
}

func setupGitRepoWithCommit(t *testing.T, dir string) {
	t.Helper()
	
	setupGitRepo(t, dir)
	
	if !isGitAvailable() {
		return
	}
	
	// Create a file and make initial commit
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("Initial content"), 0644); err != nil {
		t.Fatalf("creating README.md: %v", err)
	}
	
	addCmd := exec.Command("git", "add", "README.md")
	addCmd.Dir = dir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	
	commitCmd := exec.Command("git", "commit", "-m", "Initial commit")
	commitCmd.Dir = dir
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
}

func isGitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}