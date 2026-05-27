package model

import (
	"testing"
)

func TestNewCommandPalette(t *testing.T) {
	palette := NewCommandPalette()

	if palette == nil {
		t.Fatal("NewCommandPalette returned nil")
	}

	if len(palette.Entries) == 0 {
		t.Error("Entries is empty")
	}

	if len(palette.Filtered) == 0 {
		t.Error("Filtered is empty")
	}

	if palette.Selected != 0 {
		t.Errorf("Selected = %d, want 0", palette.Selected)
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		query    string
		target   string
		expected bool
	}{
		{"help", "/help", true},
		{"HELP", "/help", true},
		{"clear", "/clear", true},
		{"unknown", "/help", false},
		{"", "/help", true},
	}

	for _, tt := range tests {
		got := fuzzyMatch(tt.query, tt.target)
		if got != tt.expected {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.query, tt.target, got, tt.expected)
		}
	}
}

func TestPaletteHandleInput(t *testing.T) {
	palette := NewCommandPalette()

	// Test typing characters
	palette.HandleInput("h")
	if palette.Query != "h" {
		t.Errorf("Query after 'h' = %q, want 'h'", palette.Query)
	}

	palette.HandleInput("e")
	if palette.Query != "he" {
		t.Errorf("Query after 'e' = %q, want 'he'", palette.Query)
	}

	// Test backspace
	palette.HandleInput("backspace")
	if palette.Query != "h" {
		t.Errorf("Query after backspace = %q, want 'h'", palette.Query)
	}

	// Test up/down navigation
	palette.HandleInput("down")
	if palette.Selected != 1 {
		t.Errorf("Selected after down = %d, want 1", palette.Selected)
	}

	palette.HandleInput("up")
	if palette.Selected != 0 {
		t.Errorf("Selected after up = %d, want 0", palette.Selected)
	}

	// Test enter (should return command)
	cmd, executed := palette.HandleInput("enter")
	if !executed {
		t.Error("Enter should return executed=true")
	}
	if cmd == "" {
		t.Error("Enter should return a command")
	}
}

func TestPaletteRender(t *testing.T) {
	palette := NewCommandPalette()

	result := palette.Render(80, 20)

	if result == "" {
		t.Error("Render returned empty string")
	}

	// Should contain title
	if !containsSubstr(result, "Command Palette") {
		t.Error("Result doesn't contain title")
	}
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		model        string
		inputTokens  int
		outputTokens int
		expectZero   bool
	}{
		{"gpt-4o", 1000, 500, false},
		{"gpt-4", 1000, 500, false},
		{"gpt-3.5-turbo", 1000, 500, false},
		{"claude-3", 1000, 500, false},
		{"deepseek-chat", 1000, 500, false},
		{"llama3", 1000, 500, true}, // local model, zero cost
		{"qwen", 1000, 500, true},  // local model, zero cost
	}

	for _, tt := range tests {
		cost := EstimateCost(tt.model, tt.inputTokens, tt.outputTokens)
		if tt.expectZero && cost.IsPositive() {
			t.Errorf("EstimateCost(%q) should be zero for local model", tt.model)
		}
		if !tt.expectZero && !cost.IsPositive() {
			t.Errorf("EstimateCost(%q) should be positive", tt.model)
		}
	}
}

func TestCostEstimateString(t *testing.T) {
	tests := []struct {
		cost     CostEstimate
		expected string
	}{
		{CostEstimate{USD: 0.01}, "$0.0100"},
		{CostEstimate{CNY: 0.5}, "¥0.5000"},
		{CostEstimate{}, "$0.00"},
	}

	for _, tt := range tests {
		got := tt.cost.String()
		if got != tt.expected {
			t.Errorf("CostEstimate.String() = %q, want %q", got, tt.expected)
		}
	}
}
