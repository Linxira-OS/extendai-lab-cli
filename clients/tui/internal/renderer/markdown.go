package renderer

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// MarkdownPipeline renders markdown content with terminal-appropriate styling.
//
// Terminal rendering constraints:
//   - NO font size control — headings use color + bold/italic only
//   - Tables: custom implementation (glamour table support is poor)
//   - Tabs: replaced with spaces before rendering
//   - Code blocks: monospace with background highlight
//
// The pipeline:
//   1. Pre-process: replace \t, normalize line endings
//   2. Render via glamour (handles headings, lists, code blocks, bold/italic)
//   3. Tables are handled by the dedicated Table component type
func RenderMarkdown(content string, width int) string {
	if content == "" {
		return ""
	}

	// Step 1: Pre-process — normalize tabs and line endings
	content = preprocessMarkdown(content)

	// Step 2: Render via glamour
	// glamour respects terminal constraints:
	//   - NO font size — headings use color + bold only (exactly what we need)
	//   - Lists, code blocks, bold/italic all properly rendered
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)
	if err != nil {
		return content
	}

	rendered, err := renderer.Render(content)
	if err != nil || rendered == "" {
		return content
	}

	return rendered
}

// preprocessMarkdown normalizes content for terminal rendering.
func preprocessMarkdown(s string) string {
	// Replace tabs with 4 spaces (configurable via protocol)
	s = strings.ReplaceAll(s, "\t", "    ")

	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Remove trailing whitespace per line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}
