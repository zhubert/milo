package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

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
    add <tool> <pattern> [action] - Add a rule (default: allow)
    remove <tool:pattern>   - Remove a rule
  /help, /h, /?            - Show this help message`
	m.chat.AddSystemMessage(help)
	return nil
}

func (m *Model) handlePermissionsCommand(args []string) tea.Cmd {
	perms := m.agent.Permissions()

	if len(args) == 0 {
		// Show help for permissions command
		help := `Permission commands:
  /permissions list              - Show all custom rules
  /permissions add <tool> <pattern> [action]
                                - Add a rule (default: allow)
  /permissions remove <key>     - Remove a rule by key (tool:pattern)

Examples:
  /permissions add bash "npm *"
  /permissions add bash "go build*"
  /permissions add bash "rm -rf*" deny
  /permissions remove bash:npm *`
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
		sb.WriteString(fmt.Sprintf("  %s:%s → %s\n", rule.Tool, rule.Pattern, rule.Action))
	}
	m.chat.AddSystemMessage(sb.String())
	return nil
}

func (m *Model) addPermission(perms *permission.Checker, args []string) tea.Cmd {
	if len(args) < 2 {
		m.chat.AddSystemMessage("Usage: /permissions add <tool> <pattern> [action]")
		return nil
	}

	toolName := args[0]
	pattern := args[1]

	// Default to allow if no action specified
	action := permission.Allow
	if len(args) >= 3 {
		actionStr := strings.ToLower(args[2])
		switch actionStr {
		case "allow":
			action = permission.Allow
		case "deny":
			action = permission.Deny
		case "ask":
			action = permission.Ask
		default:
			m.chat.AddSystemMessage(fmt.Sprintf("Invalid action %q. Must be allow, deny, or ask.", actionStr))
			return nil
		}
	}

	rule := permission.Rule{
		Tool:    toolName,
		Pattern: pattern,
		Action:  action,
	}
	perms.AddRule(rule)

	// Save to config
	if err := perms.Save(); err != nil {
		m.footer.SetFlash(ui.ErrorStyle.Render(fmt.Sprintf("Rule added but failed to save: %v", err)))
	} else {
		m.footer.SetFlash(ui.SuccessStyle.Render("Rule added and saved"))
	}

	m.chat.AddSystemMessage(fmt.Sprintf("Added rule: %s:%s → %s", toolName, pattern, action))
	return ui.FlashTick()
}

func (m *Model) removePermission(perms *permission.Checker, args []string) tea.Cmd {
	if len(args) < 1 {
		m.chat.AddSystemMessage("Usage: /permissions remove <tool:pattern>")
		return nil
	}

	key := args[0]
	// If they provided tool and pattern separately, join them
	if len(args) >= 2 && !strings.Contains(args[0], ":") {
		key = args[0] + ":" + args[1]
	}

	if perms.RemoveRule(key) {
		// Save to config
		if err := perms.Save(); err != nil {
			m.footer.SetFlash(ui.ErrorStyle.Render(fmt.Sprintf("Rule removed but failed to save: %v", err)))
		} else {
			m.footer.SetFlash(ui.SuccessStyle.Render("Rule removed and saved"))
		}
		m.chat.AddSystemMessage(fmt.Sprintf("Removed rule: %s", key))
	} else {
		m.chat.AddSystemMessage(fmt.Sprintf("Rule not found: %s", key))
	}
	return ui.FlashTick()
}
