package model

import (
	"testing"
)

func TestBuildContextUsage(t *testing.T) {
	// Test with no session
	m := &Model{}
	usage := m.buildContextUsage()
	if usage.Status != "no session" {
		t.Errorf("Empty session status = %s, want 'no session'", usage.Status)
	}
}

func TestRenderContextSidebar(t *testing.T) {
	// Test with no session
	m := &Model{}
	result := m.renderContextSidebar(40)
	if result != "no session" {
		t.Errorf("Empty session sidebar = %q, want 'no session'", result)
	}
}

func TestRenderContextInspector(t *testing.T) {
	// Test with no session
	m := &Model{}
	result := m.renderContextInspector(40)
	if result != "no session" {
		t.Errorf("Empty session inspector = %q, want 'no session'", result)
	}
}

func TestContextUsageCalculation(t *testing.T) {
	// Create a session with messages
	session := NewSession("/tmp/test")
	session.AppendMessage(Message{Role: RoleUser, Content: "Hello"})
	session.AppendMessage(Message{Role: RoleAssistant, Content: "Hi there!"})

	m := &Model{
		session: session,
	}

	usage := m.buildContextUsage()

	if usage.Status == "no session" {
		t.Error("Status should not be 'no session'")
	}

	if usage.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", usage.MessageCount)
	}

	if usage.UsedTokens == 0 {
		t.Error("UsedTokens should not be 0")
	}

	if len(usage.RoleStats) == 0 {
		t.Error("RoleStats should not be empty")
	}
}
