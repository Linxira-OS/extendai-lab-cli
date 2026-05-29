package model

import (
	"testing"
)

func TestDefaultCommandRules(t *testing.T) {
	rules := DefaultCommandRules()
	if len(rules) == 0 {
		t.Fatal("DefaultCommandRules() should return at least one rule")
	}

	// Check that expected rules exist
	ruleNames := make(map[string]bool)
	for _, r := range rules {
		ruleNames[r.Name] = true
	}

	expected := []string{"rm-root", "rm-rf-star", "sudo", "chmod-777"}
	for _, name := range expected {
		if !ruleNames[name] {
			t.Errorf("DefaultCommandRules() missing rule %q", name)
		}
	}
}

func TestCheckCommand(t *testing.T) {
	rules := DefaultCommandRules()

	tests := []struct {
		name      string
		command   string
		wantAllow bool
		wantAction string
	}{
		{"safe command", "echo hello", true, ""},
		{"ls", "ls -la", true, ""},
		{"rm root", "rm -rf /", false, "block"},
		{"rm home", "rm -rf ~", false, "block"},
		{"rm wildcard", "rm -rf *", false, "block"},
		{"rm current", "rm -rf .", false, "block"},
		{"sudo", "sudo apt install", false, "block"},
		{"chmod 777", "chmod 777 /tmp", true, "warn"},
		{"git force push", "git push --force", true, "warn"},
		{"git reset hard", "git reset --hard HEAD~1", true, "warn"},
		{"git clean", "git clean -fd", true, "warn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, action, _ := CheckCommand(tt.command, rules)
			if allowed != tt.wantAllow {
				t.Errorf("CheckCommand(%q) allowed = %v, want %v", tt.command, allowed, tt.wantAllow)
			}
			if action != tt.wantAction {
				t.Errorf("CheckCommand(%q) action = %q, want %q", tt.command, action, tt.wantAction)
			}
		})
	}
}

func TestIsDangerousCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"safe", "echo hello", false},
		{"rm rf root", "rm -rf /", true},
		{"rm rf home", "rm -rf ~", true},
		{"rm rf current", "rm -rf .", true},
		{"rm rf wildcard", "rm -rf *", true},
		{"sudo rm", "sudo rm -rf /tmp", true},
		{"mkfs", "mkfs.ext4 /dev/sda1", true},
		{"dd", "dd if=/dev/zero of=/dev/sda", true},
		{"chmod 777 root", "chmod 777 /", true},
		{"chmod 777 recursive", "chmod -R 777 /", true},
		{"safe git", "git status", false},
		{"safe npm", "npm install", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDangerousCommand(tt.command); got != tt.want {
				t.Errorf("IsDangerousCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestSanitizeCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{"no spaces", "echo hello", "echo hello"},
		{"leading spaces", "  echo hello  ", "echo hello"},
		{"tabs", "\techo hello\t", "echo hello"},
		{"mixed", " \t echo hello \t ", "echo hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeCommand(tt.command); got != tt.want {
				t.Errorf("SanitizeCommand(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
