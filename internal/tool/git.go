package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultGitTimeout = 30 * time.Second

// GitTool provides git version control operations with enhanced safety and structure.
type GitTool struct {
	WorkDir string
}

type gitInput struct {
	Operation string   `json:"operation"`        // status, log, diff, branch, add, commit, etc.
	Args      []string `json:"args,omitempty"`   // additional arguments
	Path      string   `json:"path,omitempty"`   // specific file/directory path
	Message   string   `json:"message,omitempty"` // for commit operations
	Force     bool     `json:"force,omitempty"`  // for dangerous operations
	Timeout   int      `json:"timeout,omitempty"` // timeout in milliseconds
}

// GitStatus represents the structured output of git status.
type GitStatus struct {
	Branch         string            `json:"branch"`
	Ahead          int               `json:"ahead,omitempty"`
	Behind         int               `json:"behind,omitempty"`
	Staged         []string          `json:"staged,omitempty"`
	Modified       []string          `json:"modified,omitempty"`
	Untracked      []string          `json:"untracked,omitempty"`
	Deleted        []string          `json:"deleted,omitempty"`
	Renamed        map[string]string `json:"renamed,omitempty"`
	HasConflicts   bool              `json:"has_conflicts"`
	IsClean        bool              `json:"is_clean"`
	RemoteTracking string            `json:"remote_tracking,omitempty"`
}

func (t *GitTool) Name() string { return "git" }

func (t *GitTool) Description() string {
	return `Execute git operations with enhanced safety and structured output.

Provides common git operations with better error handling and structured responses:
- status: Get repository status with parsed output
- log: Show commit history with formatting options
- diff: Show changes with optional staging area or specific files
- branch: List, create, or switch branches
- add: Stage files for commit
- commit: Create commits with validation
- pull/push: Sync with remote repositories
- show: Display commit details
- remote: Manage remote repositories
- stash: Stash and unstash changes

Dangerous operations (like force push, hard reset) require explicit force=true parameter.
All operations validate that you're in a git repository and provide structured error messages.`
}

func (t *GitTool) InputSchema() anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"operation": map[string]any{
				"type": "string",
				"description": "Git operation to perform",
				"enum": []string{
					"status", "log", "diff", "branch", "add", "commit",
					"pull", "push", "show", "remote", "stash", "checkout",
					"merge", "rebase", "reset", "clean", "tag", "clone",
				},
			},
			"args": map[string]any{
				"type":        "array",
				"description": "Additional arguments for the git command",
				"items": map[string]any{
					"type": "string",
				},
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Specific file or directory path (optional)",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "Commit message (required for commit operation)",
			},
			"force": map[string]any{
				"type":        "boolean",
				"description": "Allow dangerous operations (default: false)",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in milliseconds (default 30000)",
			},
		},
		Required: []string{"operation"},
	}
}

func (t *GitTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in gitInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("parsing git input: %w", err)
	}

	// Set timeout
	timeout := defaultGitTimeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Check if we're in a git repository (except for clone operation)
	if in.Operation != "clone" {
		if err := t.validateGitRepo(); err != nil {
			return Result{Output: err.Error(), IsError: true}, nil
		}
	}

	// Handle specific operations
	switch in.Operation {
	case "status":
		return t.executeStatus(ctx)
	case "log":
		return t.executeLog(ctx, in.Args, in.Path)
	case "diff":
		return t.executeDiff(ctx, in.Args, in.Path)
	case "branch":
		return t.executeBranch(ctx, in.Args)
	case "add":
		return t.executeAdd(ctx, in.Args, in.Path)
	case "commit":
		return t.executeCommit(ctx, in.Message, in.Args)
	case "pull":
		return t.executePull(ctx, in.Args)
	case "push":
		return t.executePush(ctx, in.Args, in.Force)
	case "show":
		return t.executeShow(ctx, in.Args)
	case "remote":
		return t.executeRemote(ctx, in.Args)
	case "stash":
		return t.executeStash(ctx, in.Args)
	case "checkout":
		return t.executeCheckout(ctx, in.Args, in.Force)
	case "merge":
		return t.executeMerge(ctx, in.Args, in.Force)
	case "rebase":
		return t.executeRebase(ctx, in.Args, in.Force)
	case "reset":
		return t.executeReset(ctx, in.Args, in.Force)
	case "clean":
		return t.executeClean(ctx, in.Args, in.Force)
	case "tag":
		return t.executeTag(ctx, in.Args)
	case "clone":
		return t.executeClone(ctx, in.Args)
	default:
		return Result{Output: fmt.Sprintf("unsupported git operation: %s", in.Operation), IsError: true}, nil
	}
}

func (t *GitTool) validateGitRepo() error {
	gitDir := filepath.Join(t.WorkDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository (no .git directory found)")
	}
	return nil
}

func (t *GitTool) runGitCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if t.WorkDir != "" {
		cmd.Dir = t.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git command timed out")
		}
		return output, err
	}

	return output, nil
}

func (t *GitTool) executeStatus(ctx context.Context) (Result, error) {
	// Get porcelain status for parsing
	porcelainOutput, err := t.runGitCommand(ctx, "status", "--porcelain", "--branch")
	if err != nil {
		return Result{Output: fmt.Sprintf("git status failed: %v", err), IsError: true}, nil
	}

	status := t.parseGitStatus(porcelainOutput)

	// Get human-readable status as well
	humanOutput, _ := t.runGitCommand(ctx, "status")

	// Format structured output
	structuredOutput, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return Result{Output: humanOutput}, nil
	}

	output := fmt.Sprintf("=== Git Status (Structured) ===\n%s\n\n=== Git Status (Human Readable) ===\n%s", 
		string(structuredOutput), humanOutput)

	return Result{Output: output}, nil
}

func (t *GitTool) parseGitStatus(porcelainOutput string) GitStatus {
	status := GitStatus{
		Renamed: make(map[string]string),
	}

	lines := strings.Split(strings.TrimSpace(porcelainOutput), "\n")
	if len(lines) == 0 {
		return status
	}

	// Parse branch line (first line in --branch mode)
	if len(lines) > 0 && strings.HasPrefix(lines[0], "##") {
		branchLine := strings.TrimPrefix(lines[0], "## ")
		t.parseBranchInfo(branchLine, &status)
		lines = lines[1:] // Remove branch line from further processing
	}

	// Parse file status lines
	for _, line := range lines {
		if line == "" {
			continue
		}

		if len(line) < 3 {
			continue
		}

		statusCode := line[:2]
		filename := line[3:]

		switch statusCode {
		case "A ", "AM":
			status.Staged = append(status.Staged, filename)
		case " M", "MM":
			status.Modified = append(status.Modified, filename)
		case "??":
			status.Untracked = append(status.Untracked, filename)
		case " D", "AD":
			status.Deleted = append(status.Deleted, filename)
		case "UU", "AA", "DD":
			status.HasConflicts = true
			status.Modified = append(status.Modified, filename)
		default:
			if strings.Contains(statusCode, "R") {
				// Handle renames (format: "R  old -> new")
				parts := strings.Split(filename, " -> ")
				if len(parts) == 2 {
					status.Renamed[parts[0]] = parts[1]
				}
			}
		}
	}

	status.IsClean = len(status.Staged) == 0 && len(status.Modified) == 0 && 
		len(status.Untracked) == 0 && len(status.Deleted) == 0 && 
		len(status.Renamed) == 0

	return status
}

func (t *GitTool) parseBranchInfo(branchLine string, status *GitStatus) {
	// Handle various branch line formats:
	// "main...origin/main [ahead 2, behind 1]"
	// "main...origin/main [ahead 2]"
	// "main...origin/main [behind 1]"
	// "main"

	parts := strings.Fields(branchLine)
	if len(parts) == 0 {
		return
	}

	branchInfo := parts[0]
	if strings.Contains(branchInfo, "...") {
		branches := strings.Split(branchInfo, "...")
		status.Branch = branches[0]
		if len(branches) > 1 {
			status.RemoteTracking = branches[1]
		}
	} else {
		status.Branch = branchInfo
	}

	// Parse ahead/behind information
	if len(parts) > 1 {
		remaining := strings.Join(parts[1:], " ")
		aheadRegex := regexp.MustCompile(`ahead (\d+)`)
		behindRegex := regexp.MustCompile(`behind (\d+)`)

		if match := aheadRegex.FindStringSubmatch(remaining); len(match) > 1 {
			if ahead, err := strconv.Atoi(match[1]); err == nil {
				status.Ahead = ahead
			}
		}

		if match := behindRegex.FindStringSubmatch(remaining); len(match) > 1 {
			if behind, err := strconv.Atoi(match[1]); err == nil {
				status.Behind = behind
			}
		}
	}
}

func (t *GitTool) executeLog(ctx context.Context, args []string, path string) (Result, error) {
	gitArgs := []string{"log", "--oneline", "--graph", "--decorate"}
	gitArgs = append(gitArgs, args...)
	
	if path != "" {
		gitArgs = append(gitArgs, "--", path)
	}

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git log failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeDiff(ctx context.Context, args []string, path string) (Result, error) {
	gitArgs := []string{"diff"}
	gitArgs = append(gitArgs, args...)
	
	if path != "" {
		gitArgs = append(gitArgs, "--", path)
	}

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git diff failed: %v", err), IsError: true}, nil
	}

	if output == "" {
		output = "(no differences found)"
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeBranch(ctx context.Context, args []string) (Result, error) {
	gitArgs := []string{"branch"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git branch failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeAdd(ctx context.Context, args []string, path string) (Result, error) {
	gitArgs := []string{"add"}
	
	if path != "" {
		gitArgs = append(gitArgs, path)
	}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git add failed: %v", err), IsError: true}, nil
	}

	if output == "" {
		output = "Files staged successfully"
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeCommit(ctx context.Context, message string, args []string) (Result, error) {
	if message == "" && !contains(args, "-m") && !contains(args, "--message") {
		return Result{Output: "commit message is required", IsError: true}, nil
	}

	gitArgs := []string{"commit"}
	if message != "" {
		gitArgs = append(gitArgs, "-m", message)
	}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git commit failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executePull(ctx context.Context, args []string) (Result, error) {
	gitArgs := []string{"pull"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git pull failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executePush(ctx context.Context, args []string, force bool) (Result, error) {
	// Check for dangerous operations
	dangerous := force || contains(args, "--force") || contains(args, "-f") || 
		contains(args, "--force-with-lease")

	if dangerous && !force {
		return Result{
			Output: "Dangerous push operation detected. Use force=true to confirm: " + strings.Join(args, " "),
			IsError: true,
		}, nil
	}

	gitArgs := []string{"push"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git push failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeShow(ctx context.Context, args []string) (Result, error) {
	gitArgs := []string{"show"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git show failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeRemote(ctx context.Context, args []string) (Result, error) {
	gitArgs := []string{"remote"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git remote failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeStash(ctx context.Context, args []string) (Result, error) {
	gitArgs := []string{"stash"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git stash failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeCheckout(ctx context.Context, args []string, force bool) (Result, error) {
	// Check for dangerous operations
	dangerous := force || contains(args, "--force") || contains(args, "-f")

	if dangerous && !force {
		return Result{
			Output: "Dangerous checkout operation detected. Use force=true to confirm: " + strings.Join(args, " "),
			IsError: true,
		}, nil
	}

	gitArgs := []string{"checkout"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git checkout failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeMerge(ctx context.Context, args []string, force bool) (Result, error) {
	gitArgs := []string{"merge"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git merge failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeRebase(ctx context.Context, args []string, force bool) (Result, error) {
	// Rebase operations can be dangerous
	dangerous := force || contains(args, "--force-rebase") || contains(args, "-f")

	if dangerous && !force {
		return Result{
			Output: "Dangerous rebase operation detected. Use force=true to confirm: " + strings.Join(args, " "),
			IsError: true,
		}, nil
	}

	gitArgs := []string{"rebase"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git rebase failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeReset(ctx context.Context, args []string, force bool) (Result, error) {
	// Reset operations can be dangerous
	dangerous := force || contains(args, "--hard")

	if dangerous && !force {
		return Result{
			Output: "Dangerous reset operation detected. Use force=true to confirm: " + strings.Join(args, " "),
			IsError: true,
		}, nil
	}

	gitArgs := []string{"reset"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git reset failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeClean(ctx context.Context, args []string, force bool) (Result, error) {
	// Clean operations are dangerous
	dangerous := force || contains(args, "-f") || contains(args, "--force") || 
		contains(args, "-d") || contains(args, "-x")

	if dangerous && !force {
		return Result{
			Output: "Dangerous clean operation detected. Use force=true to confirm: " + strings.Join(args, " "),
			IsError: true,
		}, nil
	}

	gitArgs := []string{"clean"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git clean failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeTag(ctx context.Context, args []string) (Result, error) {
	gitArgs := []string{"tag"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git tag failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

func (t *GitTool) executeClone(ctx context.Context, args []string) (Result, error) {
	if len(args) == 0 {
		return Result{Output: "clone requires a repository URL", IsError: true}, nil
	}

	gitArgs := []string{"clone"}
	gitArgs = append(gitArgs, args...)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return Result{Output: fmt.Sprintf("git clone failed: %v", err), IsError: true}, nil
	}

	return Result{Output: output}, nil
}

// contains checks if a slice contains a specific string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}