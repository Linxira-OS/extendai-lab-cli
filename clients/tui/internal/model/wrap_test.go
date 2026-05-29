package model

import (
	"testing"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"empty", "", 10, ""},
		{"short", "hello", 10, "hello"},
		{"exact width", "hello", 5, "hello"},
		{"needs wrap", "hello world this is a test", 10, "hello\nworld this\nis a test"},
		{"preserve newlines", "hello\nworld", 10, "hello\nworld"},
		{"long line", "abcdefghijklmnop", 5, "abcde\nfghij\nklmno\np"},
		{"with spaces", "hello world", 5, "hello\nworld"},
		{"zero width", "hello", 0, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapText(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("wrapText(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

func TestWrapLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  int // number of lines
	}{
		{"short", "hello", 10, 1},
		{"exact", "hello", 5, 1},
		{"wrap once", "hello world", 5, 2},
		{"wrap multiple", "abcdefghijklmnop", 5, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapLine(tt.input, tt.width)
			if len(got) != tt.want {
				t.Errorf("wrapLine(%q, %d) returned %d lines, want %d", tt.input, tt.width, len(got), tt.want)
			}
		})
	}
}

func TestFindBreakPointSimple(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  int
	}{
		{"short", "hello", 10, 5},
		{"exact", "hello", 5, 5},
		{"with space", "hello world", 5, 5},
		{"space at end", "hello ", 5, 5},
		{"no space", "abcdefghijklmnop", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findBreakPointSimple(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("findBreakPointSimple(%q, %d) = %d, want %d", tt.input, tt.width, got, tt.want)
			}
		})
	}
}
