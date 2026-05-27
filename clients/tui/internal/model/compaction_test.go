package model

import (
	"testing"
)

func TestDefaultCompactionConfig(t *testing.T) {
	config := DefaultCompactionConfig()

	if !config.Enabled {
		t.Error("Enabled should be true")
	}

	if config.TokenThreshold != 800_000 {
		t.Errorf("TokenThreshold = %d, want 800000", config.TokenThreshold)
	}

	if config.MinTokens != 500_000 {
		t.Errorf("MinTokens = %d, want 500000", config.MinTokens)
	}

	if config.KeepRecent != 4 {
		t.Errorf("KeepRecent = %d, want 4", config.KeepRecent)
	}
}

func TestPlanCompaction(t *testing.T) {
	// Test with no session
	m := &Model{}
	config := DefaultCompactionConfig()
	plan := m.PlanCompaction(config)
	if plan != nil {
		t.Error("PlanCompaction should return nil for empty session")
	}
}

func TestShouldCompact(t *testing.T) {
	// Test with no session
	m := &Model{}
	config := DefaultCompactionConfig()
	if m.ShouldCompact(config) {
		t.Error("ShouldCompact should return false for empty session")
	}
}

func TestBuildRelayPrompt(t *testing.T) {
	// Test with nil plan
	m := &Model{}
	prompt := m.BuildRelayPrompt(nil)
	if prompt != "" {
		t.Error("BuildRelayPrompt should return empty for nil plan")
	}
}

func TestRenderCompactionRelay(t *testing.T) {
	relay := CompactionRelay{
		Goal:        "Test goal",
		Constraints: "Test constraints",
		Progress: ProgressSection{
			Done:      "Test done",
			InProgress: "Test in progress",
			Blocked:   "Test blocked",
		},
		Decisions: "Test decisions",
		NextStep:  "Test next step",
	}

	result := RenderCompactionRelay(relay, 80)

	if result == "" {
		t.Error("RenderCompactionRelay returned empty string")
	}

	// Verify contains expected elements
	if !containsSubstr(result, "Test goal") {
		t.Error("Result doesn't contain goal")
	}

	if !containsSubstr(result, "Test done") {
		t.Error("Result doesn't contain progress done")
	}
}
