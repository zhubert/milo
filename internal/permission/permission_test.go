package permission

import (
	"encoding/json"
	"testing"
)

func makeInput(data map[string]interface{}) json.RawMessage {
	b, _ := json.Marshal(data)
	return b
}

func TestActionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		action Action
		expect string
	}{
		{Allow, "allow"},
		{Deny, "deny"},
		{Ask, "ask"},
		{Action(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.action.String(); got != tt.expect {
			t.Errorf("Action(%d).String() = %q, want %q", tt.action, got, tt.expect)
		}
	}
}

func TestRuleMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rule     Rule
		toolName string
		input    string
		want     bool
	}{
		{
			name:     "exact tool match with wildcard pattern",
			rule:     Rule{Tool: "bash", Pattern: "*", Action: Allow},
			toolName: "bash",
			input:    "ls -la",
			want:     true,
		},
		{
			name:     "wildcard tool match",
			rule:     Rule{Tool: "*", Pattern: "*", Action: Allow},
			toolName: "anyTool",
			input:    "anything",
			want:     true,
		},
		{
			name:     "tool mismatch",
			rule:     Rule{Tool: "bash", Pattern: "*", Action: Allow},
			toolName: "read",
			input:    "/some/path",
			want:     false,
		},
		{
			name:     "prefix pattern for bash",
			rule:     Rule{Tool: "bash", Pattern: "git:*", Action: Allow},
			toolName: "bash",
			input:    "git status",
			want:     true,
		},
		{
			name:     "prefix pattern mismatch",
			rule:     Rule{Tool: "bash", Pattern: "git:*", Action: Allow},
			toolName: "bash",
			input:    "npm install",
			want:     false,
		},
		{
			name:     "file basename match",
			rule:     Rule{Tool: "read", Pattern: "*.env", Action: Ask},
			toolName: "read",
			input:    "/home/user/project/.env",
			want:     true,
		},
		{
			name:     "exact path match",
			rule:     Rule{Tool: "write", Pattern: "/tmp/test.txt", Action: Allow},
			toolName: "write",
			input:    "/tmp/test.txt",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.rule.Matches(tt.toolName, tt.input)
			if got != tt.want {
				t.Errorf("Rule.Matches(%q, %q) = %v, want %v", tt.toolName, tt.input, got, tt.want)
			}
		})
	}
}

func TestRuleSpecificity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rule    Rule
		wantGt0 bool
	}{
		{
			name:    "wildcard tool and pattern",
			rule:    Rule{Tool: "*", Pattern: "*", Action: Allow},
			wantGt0: false,
		},
		{
			name:    "specific tool, wildcard pattern",
			rule:    Rule{Tool: "bash", Pattern: "*", Action: Allow},
			wantGt0: true,
		},
		{
			name:    "specific tool and pattern",
			rule:    Rule{Tool: "bash", Pattern: "git status", Action: Allow},
			wantGt0: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			score := tt.rule.Specificity()
			if tt.wantGt0 && score <= 0 {
				t.Errorf("Rule.Specificity() = %d, want > 0", score)
			}
			if !tt.wantGt0 && score != 0 {
				t.Errorf("Rule.Specificity() = %d, want 0", score)
			}
		})
	}

	// More specific rules should have higher scores
	general := Rule{Tool: "bash", Pattern: "*", Action: Allow}
	specific := Rule{Tool: "bash", Pattern: "git status --verbose", Action: Allow}

	if specific.Specificity() <= general.Specificity() {
		t.Errorf("specific rule should have higher specificity than general rule")
	}
}

func TestDefaultRules(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	tests := []struct {
		name   string
		tool   string
		input  json.RawMessage
		expect Action
	}{
		{
			name:   "read allows any file",
			tool:   "read",
			input:  makeInput(map[string]interface{}{"file_path": "/home/user/test.go"}),
			expect: Allow,
		},
		{
			name:   "glob allows",
			tool:   "glob",
			input:  makeInput(map[string]interface{}{"pattern": "*.go"}),
			expect: Allow,
		},
		{
			name:   "grep allows",
			tool:   "grep",
			input:  makeInput(map[string]interface{}{"pattern": "TODO"}),
			expect: Allow,
		},
		{
			name:   "write asks by default",
			tool:   "write",
			input:  makeInput(map[string]interface{}{"file_path": "/tmp/test.txt"}),
			expect: Ask,
		},
		{
			name:   "edit asks by default",
			tool:   "edit",
			input:  makeInput(map[string]interface{}{"file_path": "/tmp/test.txt"}),
			expect: Ask,
		},
		{
			name:   "unknown tool asks",
			tool:   "unknown",
			input:  nil,
			expect: Ask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := c.Check(tt.tool, tt.input)
			if got != tt.expect {
				t.Errorf("Check(%q, %s) = %v, want %v", tt.tool, string(tt.input), got, tt.expect)
			}
		})
	}
}

func TestSafeBashCommands(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	safeCommands := []string{
		"git status",
		"git log --oneline",
		"git diff HEAD",
		"git branch -a",
		"ls -la",
		"pwd",
		"go version",
		"whoami",
	}

	for _, cmd := range safeCommands {
		t.Run(cmd, func(t *testing.T) {
			t.Parallel()
			input := makeInput(map[string]interface{}{"command": cmd})
			got := c.Check("bash", input)
			if got != Allow {
				t.Errorf("safe command %q should be allowed, got %v", cmd, got)
			}
		})
	}
}

func TestUnsafeBashCommands(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// These should require Ask (not in safe list, not in deny list)
	unsafeCommands := []string{
		"rm file.txt",
		"npm install",
		"go build",
		"make",
	}

	for _, cmd := range unsafeCommands {
		t.Run(cmd, func(t *testing.T) {
			t.Parallel()
			input := makeInput(map[string]interface{}{"command": cmd})
			got := c.Check("bash", input)
			if got != Ask {
				t.Errorf("unsafe command %q should ask, got %v", cmd, got)
			}
		})
	}
}

func TestDangerousBashCommands(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// These should be denied outright
	dangerousCommands := []string{
		"rm -rf /",
		"rm -rf /home",
		"rm -rf /var/log",
		"chmod -R 777 /",
	}

	for _, cmd := range dangerousCommands {
		t.Run(cmd, func(t *testing.T) {
			t.Parallel()
			input := makeInput(map[string]interface{}{"command": cmd})
			got := c.Check("bash", input)
			if got != Deny {
				t.Errorf("dangerous command %q should be denied, got %v", cmd, got)
			}
		})
	}
}

func TestSensitiveFiles(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	sensitiveFiles := []string{
		"/home/user/.env",
		"/app/config.env",
		"/home/user/.ssh/id_rsa",
		"/app/credentials.json",
	}

	for _, path := range sensitiveFiles {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			input := makeInput(map[string]interface{}{"file_path": path})

			// Reading sensitive files should ask
			if got := c.Check("read", input); got != Ask {
				t.Errorf("read sensitive file %q should ask, got %v", path, got)
			}
		})
	}
}

func TestAllowToolAlways(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// Before AllowToolAlways, bash requires ask.
	input := makeInput(map[string]interface{}{"command": "npm install"})
	if got := c.Check("bash", input); got != Ask {
		t.Fatalf("expected Ask for bash, got %v", got)
	}

	c.AllowToolAlways("bash")

	// After AllowToolAlways, all bash commands should be allowed.
	if got := c.Check("bash", input); got != Allow {
		t.Fatalf("expected Allow for bash after AllowToolAlways, got %v", got)
	}

	// Other commands too
	input2 := makeInput(map[string]interface{}{"command": "rm -rf /tmp/test"})
	if got := c.Check("bash", input2); got != Allow {
		t.Fatalf("expected Allow for any bash after AllowToolAlways, got %v", got)
	}
}

func TestAllowAlwaysSpecific(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	input := makeInput(map[string]interface{}{"command": "npm install"})

	// Before AllowAlways, requires ask
	if got := c.Check("bash", input); got != Ask {
		t.Fatalf("expected Ask for bash, got %v", got)
	}

	c.AllowAlways("bash", input)

	// After AllowAlways, should be allowed
	if got := c.Check("bash", input); got != Allow {
		t.Fatalf("expected Allow after AllowAlways, got %v", got)
	}
}

func TestAddRule(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// By default, npm install asks
	input := makeInput(map[string]interface{}{"command": "npm install"})
	if got := c.Check("bash", input); got != Ask {
		t.Fatalf("expected Ask for npm install, got %v", got)
	}

	// Add a rule to allow npm commands
	c.AddRule(Rule{Tool: "bash", Pattern: "npm:*", Action: Allow})

	// Now it should be allowed
	if got := c.Check("bash", input); got != Allow {
		t.Fatalf("expected Allow for npm install after AddRule, got %v", got)
	}
}

func TestSetDefaultAction(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// Unknown tool defaults to Ask
	if got := c.Check("unknownTool", nil); got != Ask {
		t.Fatalf("expected Ask for unknown tool, got %v", got)
	}

	c.SetDefaultAction(Deny)

	// Now unknown tools should be denied
	if got := c.Check("unknownTool", nil); got != Deny {
		t.Fatalf("expected Deny for unknown tool after SetDefaultAction, got %v", got)
	}
}

func TestRulesReturnsSnapshot(t *testing.T) {
	t.Parallel()

	c := NewChecker()
	initialCount := len(c.Rules())

	c.AddRule(Rule{Tool: "test", Pattern: "*", Action: Allow})

	// Adding a rule should increase the count
	if newCount := len(c.Rules()); newCount != initialCount+1 {
		t.Errorf("expected %d rules after adding one, got %d", initialCount+1, newCount)
	}

	// Modifying the returned slice should not affect the checker
	rules := c.Rules()
	rules[0] = Rule{Tool: "modified", Pattern: "modified", Action: Deny}

	// Original should be unchanged
	actualRules := c.Rules()
	if actualRules[0].Tool == "modified" {
		t.Error("modifying returned rules slice should not affect checker")
	}
}

func TestSpecificityOrdering(t *testing.T) {
	t.Parallel()

	c := &Checker{
		defaultRules:  make([]Rule, 0),
		customRules:   make(map[string]Rule),
		sessionAlways: make(map[string]bool),
		defaultAction: Ask,
	}

	// Add rules in non-specificity order
	c.AddRule(Rule{Tool: "bash", Pattern: "*", Action: Ask})          // Less specific
	c.AddRule(Rule{Tool: "bash", Pattern: "git:*", Action: Allow})    // More specific
	c.AddRule(Rule{Tool: "bash", Pattern: "git push*", Action: Deny}) // Most specific

	// git status should match "git:*" (Allow)
	input1 := makeInput(map[string]interface{}{"command": "git status"})
	if got := c.Check("bash", input1); got != Allow {
		t.Errorf("git status should be allowed, got %v", got)
	}

	// git push should match "git push*" (Deny) - most specific
	input2 := makeInput(map[string]interface{}{"command": "git push origin main"})
	if got := c.Check("bash", input2); got != Deny {
		t.Errorf("git push should be denied, got %v", got)
	}

	// npm install should match "*" (Ask)
	input3 := makeInput(map[string]interface{}{"command": "npm install"})
	if got := c.Check("bash", input3); got != Ask {
		t.Errorf("npm install should ask, got %v", got)
	}
}

func TestExtractInputString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toolName string
		input    json.RawMessage
		want     string
	}{
		{
			name:     "bash command",
			toolName: "bash",
			input:    makeInput(map[string]interface{}{"command": "ls -la"}),
			want:     "ls -la",
		},
		{
			name:     "read file_path",
			toolName: "read",
			input:    makeInput(map[string]interface{}{"file_path": "/home/test.txt"}),
			want:     "/home/test.txt",
		},
		{
			name:     "write file_path",
			toolName: "write",
			input:    makeInput(map[string]interface{}{"file_path": "/home/output.txt", "content": "hello"}),
			want:     "/home/output.txt",
		},
		{
			name:     "edit file_path",
			toolName: "edit",
			input:    makeInput(map[string]interface{}{"file_path": "/home/edit.txt"}),
			want:     "/home/edit.txt",
		},
		{
			name:     "glob path",
			toolName: "glob",
			input:    makeInput(map[string]interface{}{"path": "/home/project"}),
			want:     "/home/project",
		},
		{
			name:     "glob pattern fallback",
			toolName: "glob",
			input:    makeInput(map[string]interface{}{"pattern": "*.go"}),
			want:     "*.go",
		},
		{
			name:     "grep path",
			toolName: "grep",
			input:    makeInput(map[string]interface{}{"path": "/home/search", "pattern": "TODO"}),
			want:     "/home/search",
		},
		{
			name:     "empty input",
			toolName: "bash",
			input:    nil,
			want:     "",
		},
		{
			name:     "invalid json",
			toolName: "bash",
			input:    json.RawMessage(`{invalid}`),
			want:     "",
		},
		{
			name:     "unknown tool",
			toolName: "unknown",
			input:    makeInput(map[string]interface{}{"anything": "value"}),
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractInputString(tt.toolName, tt.input)
			if got != tt.want {
				t.Errorf("extractInputString(%q, %s) = %q, want %q", tt.toolName, string(tt.input), got, tt.want)
			}
		})
	}
}

func TestIsFilePathTool(t *testing.T) {
	t.Parallel()

	fileTools := []string{"read", "write", "edit", "glob", "grep"}
	for _, tool := range fileTools {
		if !isFilePathTool(tool) {
			t.Errorf("isFilePathTool(%q) = false, want true", tool)
		}
	}

	nonFileTools := []string{"bash", "unknown", ""}
	for _, tool := range nonFileTools {
		if isFilePathTool(tool) {
			t.Errorf("isFilePathTool(%q) = true, want false", tool)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	done := make(chan bool)

	// Multiple goroutines reading
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = c.Check("bash", makeInput(map[string]interface{}{"command": "test"}))
				_ = c.Rules()
			}
			done <- true
		}()
	}

	// Multiple goroutines writing
	for i := 0; i < 5; i++ {
		go func(n int) {
			for j := 0; j < 20; j++ {
				c.AddRule(Rule{Tool: "test", Pattern: "*", Action: Allow})
				c.AllowToolAlways("test")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}
}

func TestRuleString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rule Rule
		want string
	}{
		{
			rule: Rule{Tool: "bash", Pattern: "npm:*", Action: Allow},
			want: "Bash(npm:*)",
		},
		{
			rule: Rule{Tool: "bash", Pattern: "rm -rf *", Action: Deny},
			want: "Bash(rm -rf *):deny",
		},
		{
			rule: Rule{Tool: "write", Pattern: "*.tmp", Action: Ask},
			want: "Write(*.tmp):ask",
		},
		{
			rule: Rule{Tool: "read", Pattern: "*", Action: Allow},
			want: "Read(*)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.rule.String()
			if got != tt.want {
				t.Errorf("Rule.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestColonPatternSyntax(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// Add rule with colon syntax: git:* means command "git" with any args
	c.AddRule(Rule{Tool: "bash", Pattern: "git:*", Action: Allow})

	tests := []struct {
		command string
		want    Action
	}{
		{"git", Allow},                  // Exact command match
		{"git status", Allow},           // Command with args
		{"git commit -m test", Allow},   // Command with multiple args
		{"git push origin main", Allow}, // Command with multiple args
		{"gitconfig", Ask},              // NOT a match - "gitconfig" is not "git"
		{"git-lfs pull", Ask},           // NOT a match - "git-lfs" is not "git"
		{"npm install", Ask},            // Doesn't match
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()
			input := makeInput(map[string]interface{}{"command": tt.command})
			got := c.Check("bash", input)
			if got != tt.want {
				t.Errorf("Check(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestCompoundCommandMatching(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// Add rule for go commands: "go:*" means command "go" with any args
	c.AddRule(Rule{Tool: "bash", Pattern: "go:*", Action: Allow})

	tests := []struct {
		command string
		want    Action
	}{
		{"go test ./...", Allow},                      // Direct match
		{"cd /path && go test ./...", Allow},          // Compound with &&
		{"cd /path; go build", Allow},                 // Compound with ;
		{"echo hello || go run main.go", Allow},       // Compound with ||
		{"cd /path && npm install && go test", Allow}, // Multiple parts, one matches
		{"cd /path && npm install", Ask},              // No go command
		{"gopher", Ask},                               // "gopher" is not "go"
		{"cd /path && go-task build", Ask},            // "go-task" is not "go"
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()
			input := makeInput(map[string]interface{}{"command": tt.command})
			got := c.Check("bash", input)
			if got != tt.want {
				t.Errorf("Check(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestSplitCompoundCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command string
		want    []string
	}{
		{"ls", []string{"ls"}},
		{"cd /path && go test", []string{"cd /path", "go test"}},
		{"echo a; echo b", []string{"echo a", "echo b"}},
		{"cmd1 || cmd2", []string{"cmd1", "cmd2"}},
		{"a && b; c || d", []string{"a", "b", "c", "d"}},
		{"  spaced  &&  commands  ", []string{"spaced", "commands"}},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()
			got := splitCompoundCommand(tt.command)
			if len(got) != len(tt.want) {
				t.Errorf("splitCompoundCommand(%q) = %v, want %v", tt.command, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitCompoundCommand(%q)[%d] = %q, want %q", tt.command, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    Rule
		wantErr bool
	}{
		{
			input: "Bash(git:*)",
			want:  Rule{Tool: "bash", Pattern: "git:*", Action: Allow},
		},
		{
			input: "bash(npm:*)",
			want:  Rule{Tool: "bash", Pattern: "npm:*", Action: Allow},
		},
		{
			input: "Bash(rm -rf *):deny",
			want:  Rule{Tool: "bash", Pattern: "rm -rf *", Action: Deny},
		},
		{
			input: "Write(*.tmp):ask",
			want:  Rule{Tool: "write", Pattern: "*.tmp", Action: Ask},
		},
		{
			input: "Read(*)",
			want:  Rule{Tool: "read", Pattern: "*", Action: Allow},
		},
		{
			input: "Bash()",
			want:  Rule{Tool: "bash", Pattern: "*", Action: Allow},
		},
		{
			input: "Bash(go build ./...):allow",
			want:  Rule{Tool: "bash", Pattern: "go build ./...", Action: Allow},
		},
		{
			input:   "",
			wantErr: true,
		},
		{
			input:   "invalid",
			wantErr: true,
		},
		{
			input:   "(npm:*)",
			wantErr: true,
		},
		{
			input:   "Bash(npm:*",
			wantErr: true,
		},
		{
			input:   "Bash(*):invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRule(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRule(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Tool != tt.want.Tool {
					t.Errorf("ParseRule(%q).Tool = %q, want %q", tt.input, got.Tool, tt.want.Tool)
				}
				if got.Pattern != tt.want.Pattern {
					t.Errorf("ParseRule(%q).Pattern = %q, want %q", tt.input, got.Pattern, tt.want.Pattern)
				}
				if got.Action != tt.want.Action {
					t.Errorf("ParseRule(%q).Action = %v, want %v", tt.input, got.Action, tt.want.Action)
				}
			}
		})
	}
}

func TestParseRuleRoundTrip(t *testing.T) {
	t.Parallel()

	rules := []Rule{
		{Tool: "bash", Pattern: "npm:*", Action: Allow},
		{Tool: "bash", Pattern: "rm -rf *", Action: Deny},
		{Tool: "write", Pattern: "*.tmp", Action: Ask},
		{Tool: "read", Pattern: "*", Action: Allow},
		{Tool: "bash", Pattern: "git:*", Action: Allow},
	}

	for _, rule := range rules {
		t.Run(rule.String(), func(t *testing.T) {
			t.Parallel()
			str := rule.String()
			parsed, err := ParseRule(str)
			if err != nil {
				t.Fatalf("ParseRule(%q) error = %v", str, err)
			}
			if parsed.Tool != rule.Tool || parsed.Pattern != rule.Pattern || parsed.Action != rule.Action {
				t.Errorf("Round trip failed: %+v -> %q -> %+v", rule, str, parsed)
			}
		})
	}
}
