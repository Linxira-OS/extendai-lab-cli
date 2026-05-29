package model

import (
	"regexp"
	"strings"
)

// ─── Command Safety Rules (from oh-my-pi) ─────────────────────

// CommandRule defines a pattern and action for command interception.
type CommandRule struct {
	Name        string
	Pattern     *regexp.Regexp
	Action      string // "block" | "warn" | "modify"
	Message     string // message to show when blocked/warned
	Replacement string // replacement command if Action == "modify"
}

// DefaultCommandRules returns the default set of command safety rules.
func DefaultCommandRules() []CommandRule {
	return []CommandRule{
		// Block dangerous rm commands
		{
			Name:    "rm-root",
			Pattern: regexp.MustCompile(`rm\s+(-[a-zA-Z]*\s+)*(\/|~|\$HOME)`),
			Action:  "block",
			Message: "Blocked: rm on root/home directory is not allowed",
		},
		{
			Name:    "rm-rf-star",
			Pattern: regexp.MustCompile(`rm\s+(-[a-zA-Z]*\s+)*\*`),
			Action:  "block",
			Message: "Blocked: rm with wildcard (*) is not allowed",
		},
		{
			Name:    "rm-rf-current",
			Pattern: regexp.MustCompile(`rm\s+(-[a-zA-Z]*\s+)*\.`),
			Action:  "block",
			Message: "Blocked: rm on current directory (.) is not allowed",
		},

		// Block dangerous chmod
		{
			Name:    "chmod-777",
			Pattern: regexp.MustCompile(`chmod\s+777`),
			Action:  "warn",
			Message: "Warning: chmod 777 gives full permissions to everyone",
		},

		// Block sudo commands
		{
			Name:    "sudo",
			Pattern: regexp.MustCompile(`sudo\s+`),
			Action:  "block",
			Message: "Blocked: sudo commands are not allowed in this context",
		},

		// Block commands that modify system files
		{
			Name:    "modify-etc",
			Pattern: regexp.MustCompile(`(mv|cp|rm|chmod|chown)\s+.*\/etc\/`),
			Action:  "block",
			Message: "Blocked: modifying /etc/ files is not allowed",
		},

		// Warn about git push --force
		{
			Name:    "git-force-push",
			Pattern: regexp.MustCompile(`git\s+push\s+.*--force`),
			Action:  "warn",
			Message: "Warning: git push --force can overwrite remote history",
		},

		// Warn about git reset --hard
		{
			Name:    "git-reset-hard",
			Pattern: regexp.MustCompile(`git\s+reset\s+--hard`),
			Action:  "warn",
			Message: "Warning: git reset --hard discards uncommitted changes",
		},

		// Block commands that could delete git history
		{
			Name:    "git-clean-d",
			Pattern: regexp.MustCompile(`git\s+clean\s+.*-[a-zA-Z]*d`),
			Action:  "warn",
			Message: "Warning: git clean -fd removes untracked files and directories",
		},
	}
}

// CheckCommand checks a command against the safety rules.
// Returns (allowed, action, message).
func CheckCommand(command string, rules []CommandRule) (bool, string, string) {
	for _, rule := range rules {
		if rule.Pattern.MatchString(command) {
			switch rule.Action {
			case "block":
				return false, "block", rule.Message
			case "warn":
				// Warn but allow
				return true, "warn", rule.Message
			case "modify":
				// Return modified command
				return true, "modify", rule.Replacement
			}
		}
	}
	return true, "", ""
}

// IsDangerousCommand checks if a command is potentially dangerous.
// This is a simpler version that doesn't use regex.
func IsDangerousCommand(command string) bool {
	dangerous := []string{
		"rm -rf /",
		"rm -rf ~",
		"rm -rf .",
		"rm -rf *",
		"sudo rm",
		"mkfs",
		"dd if=",
		"> /dev/",
		"chmod 777 /",
		"chmod -r 777 /",
	}

	cmdLower := strings.ToLower(strings.TrimSpace(command))
	for _, d := range dangerous {
		if strings.Contains(cmdLower, d) {
			return true
		}
	}
	return false
}

// SanitizeCommand sanitizes a command for safe execution.
// Currently just trims whitespace.
func SanitizeCommand(command string) string {
	return strings.TrimSpace(command)
}
