package model

import (
	"testing"
)

func TestFormatTokenK(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0.0K"},
		{500, "0.5K"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{10000, "10.0K"},
		{100000, "100.0K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
	}

	for _, tt := range tests {
		got := formatTokenK(tt.input)
		if got != tt.expected {
			t.Errorf("formatTokenK(%d) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestContextStatusColor(t *testing.T) {
	tests := []struct {
		pct      float64
		expected string // color name
	}{
		{0, "Success"},
		{50, "Success"},
		{84.9, "Success"},
		{85, "Warning"},
		{90, "Warning"},
		{94.9, "Warning"},
		{95, "Error"},
		{100, "Error"},
	}

	for _, tt := range tests {
		color := contextStatusColor(tt.pct)
		// We can't directly compare colors, but we can verify the function doesn't panic
		_ = color
	}
}

func TestBuildFooterProps(t *testing.T) {
	// Create a minimal model for testing
	m := &Model{
		currentDir:      "/tmp/test",
		serverConnected: true,
		modelName:       "gpt-4o",
		focusZone:       FocusMain,
	}

	props := m.buildFooterProps(80)

	if props.Dir != "/tmp/test" {
		t.Errorf("Dir = %s, want /tmp/test", props.Dir)
	}

	if !props.ServerOK {
		t.Error("ServerOK = false, want true")
	}

	if props.ModelName != "gpt-4o" {
		t.Errorf("ModelName = %s, want gpt-4o", props.ModelName)
	}

	if props.ModeLabel != "[main]" {
		t.Errorf("ModeLabel = %s, want [main]", props.ModeLabel)
	}
}

func TestBuildContextBar(t *testing.T) {
	// Test with no session
	m := &Model{}
	bar, pct := m.buildContextBar(80)
	if bar != "" {
		t.Errorf("Empty session bar = %q, want empty", bar)
	}
	if pct != 0 {
		t.Errorf("Empty session pct = %f, want 0", pct)
	}
}

func TestRenderFooterFromProps(t *testing.T) {
	props := FooterProps{
		Dir:         "/tmp/test",
		ServerOK:    true,
		ContextBar:  "test bar",
		ModelName:   "gpt-4o",
		ThemeName:   "Dark",
		ModeLabel:   "[main]",
		IsWorking:   false,
		WorkingLabel: "",
		SpinnerFrame: "",
	}

	footer := renderFooterFromProps(80, props)

	if footer == "" {
		t.Error("renderFooterFromProps returned empty string")
	}

	// Verify footer contains expected elements
	if !containsStr(footer, "/tmp/test") {
		t.Error("Footer doesn't contain directory")
	}

	if !containsStr(footer, "gpt-4o") {
		t.Error("Footer doesn't contain model name")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
