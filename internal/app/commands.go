package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zhubert/milo/internal/permission"
	"github.com/zhubert/milo/internal/ui"
)

// handleSlashCommand processes slash commands and returns an appropriate tea.Cmd.
func (m *Model) handleSlashCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/permissions", "/perms", "/p":
		return m.handlePermissionsCommand(args)
	case "/help", "/h", "/?":
		return m.handleHelpCommand()
	default:
		m.chat.AddSystemMessage(fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd))
		return nil
	}
}

func (m *Model) handleHelpCommand() tea.Cmd {
	help := `Available commands:
  /permissions, /perms, /p  - Manage permission rules
    list                    - Show all custom rules
    add <rule>              - Add a rule, e.g. Bash(git:*)
    remove <rule>           - Remove a rule
  /help, /h, /?            - Show this help message

  Special commands:
  exit, quit               - Close the application`
	m.chat.AddSystemMessage(help)
	return nil
}

func (m *Model) handlePermissionsCommand(args []string) tea.Cmd {
	perms := m.agent.Permissions()

	if len(args) == 0 {
		// Show help for permissions command
		help := `Permission commands:
  /permissions list           - Show all custom rules
  /permissions add <rule>     - Add a rule (default: allow)
  /permissions remove <rule>  - Remove a rule

Examples:
  /p add Bash(npm:*)
  /p add Bash(go build:*)
  /p add Bash(rm -rf *):deny
  /p remove Bash(npm:*)`
		m.chat.AddSystemMessage(help)
		return nil
	}

	subcmd := strings.ToLower(args[0])
	subargs := args[1:]

	switch subcmd {
	case "list", "ls", "l":
		return m.listPermissions(perms)
	case "add", "a":
		return m.addPermission(perms, subargs)
	case "remove", "rm", "delete", "del":
		return m.removePermission(perms, subargs)
	default:
		m.chat.AddSystemMessage(fmt.Sprintf("Unknown permissions subcommand: %s", subcmd))
		return nil
	}
}

func (m *Model) listPermissions(perms *permission.Checker) tea.Cmd {
	rules := perms.CustomRules()
	if len(rules) == 0 {
		m.chat.AddSystemMessage("No custom permission rules configured.")
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Custom permission rules:\n")
	for _, rule := range rules {
		sb.WriteString(fmt.Sprintf("  %s\n", rule.String()))
	}
	m.chat.AddSystemMessage(sb.String())
	return nil
}

func (m *Model) addPermission(perms *permission.Checker, args []string) tea.Cmd {
	if len(args) < 1 {
		m.chat.AddSystemMessage("Usage: /permissions add <rule>\nExample: /p add Bash(npm *)")
		return nil
	}

	// Join all args to handle rules with spaces
	ruleStr := strings.Join(args, " ")
	rule, err := permission.ParseRule(ruleStr)
	if err != nil {
		m.chat.AddSystemMessage(fmt.Sprintf("Invalid rule: %v", err))
		return nil
	}

	perms.AddRule(rule)

	// Save to config
	if err := perms.Save(); err != nil {
		m.footer.SetFlash(ui.ErrorStyle.Render(fmt.Sprintf("Rule added but failed to save: %v", err)))
	} else {
		m.footer.SetFlash(ui.SuccessStyle.Render("Rule added and saved"))
	}

	m.chat.AddSystemMessage(fmt.Sprintf("Added rule: %s", rule.String()))
	return ui.FlashTick()
}

func (m *Model) removePermission(perms *permission.Checker, args []string) tea.Cmd {
	if len(args) < 1 {
		m.chat.AddSystemMessage("Usage: /permissions remove <rule>\nExample: /p remove Bash(npm *)")
		return nil
	}

	// Join all args to handle rules with spaces
	ruleStr := strings.Join(args, " ")
	rule, err := permission.ParseRule(ruleStr)
	if err != nil {
		m.chat.AddSystemMessage(fmt.Sprintf("Invalid rule: %v", err))
		return nil
	}

	key := rule.Key()
	if perms.RemoveRule(key) {
		// Save to config
		if err := perms.Save(); err != nil {
			m.footer.SetFlash(ui.ErrorStyle.Render(fmt.Sprintf("Rule removed but failed to save: %v", err)))
		} else {
			m.footer.SetFlash(ui.SuccessStyle.Render("Rule removed and saved"))
		}
		m.chat.AddSystemMessage(fmt.Sprintf("Removed rule: %s", rule.String()))
	} else {
		m.chat.AddSystemMessage(fmt.Sprintf("Rule not found: %s", rule.String()))
	}
	return ui.FlashTick()
}
