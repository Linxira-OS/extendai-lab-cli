package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// ─── Streaming Thinking (CodeWhale-style) ─────────────────────

// ThinkingState tracks the lifecycle of a thinking block.
type ThinkingState struct {
	Active     bool
	Content    string
	StartedAt  time.Time
	Duration   time.Duration
	Expanded   bool // user can expand/collapse
	Translated bool // placeholder replaced with actual content
}

// NewThinkingState creates a new thinking state.
func NewThinkingState() *ThinkingState {
	return &ThinkingState{
		Active:    true,
		StartedAt: time.Now(),
		Expanded:  true, // default expanded
	}
}

// Append adds content to the thinking block.
func (t *ThinkingState) Append(content string) {
	if !t.Active {
		return
	}
	t.Content += content
}

// Finalize marks the thinking block as complete.
func (t *ThinkingState) Finalize() {
	t.Active = false
	t.Duration = time.Since(t.StartedAt)
}

// ─── Render thinking block ───────────────────────────────────

// RenderThinkingBlock renders a thinking block with spinner/duration.
func RenderThinkingBlock(state *ThinkingState, width int) string {
	if state == nil {
		return ""
	}

	var b strings.Builder

	// Header with spinner or duration
	headerStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Accent).
		Bold(true)

	if state.Active {
		// Animated spinner
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame := int(time.Since(state.StartedAt).Milliseconds()/80) % len(frames)
		spinner := frames[frame]
		elapsed := time.Since(state.StartedAt).Seconds()

		header := fmt.Sprintf("%s Thinking... (%.1fs)", spinner, elapsed)
		b.WriteString(headerStyle.Render(header))
	} else {
		// Completed with duration
		header := fmt.Sprintf("✓ Thinking (%.1fs)", state.Duration.Seconds())
		b.WriteString(headerStyle.Render(header))
	}
	b.WriteString("\n")

	// Content (dimmed, italic)
	if state.Content != "" {
		contentStyle := lipgloss.NewStyle().
			Foreground(theme.Colors.TextDim).
			Italic(true)

		// Truncate if too long
		content := state.Content
		maxChars := width * 10 // ~10 lines max for collapsed view
		if !state.Expanded && len(content) > maxChars {
			content = content[:maxChars] + "..."
		}

		lines := strings.Split(content, "\n")
		for _, line := range lines {
			// Indent thinking content
			b.WriteString("  ")
			b.WriteString(contentStyle.Render(line))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	return b.String()
}

// ─── Thinking with CoT summarization ─────────────────────────

// RenderThinkingSummary renders a summarized thinking block.
// Only shows key points, not the full raw thinking.
func RenderThinkingSummary(state *ThinkingState, width int) string {
	if state == nil || state.Content == "" {
		return ""
	}

	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Accent).
		Bold(true)

	if state.Active {
		b.WriteString(headerStyle.Render("⠋ Thinking..."))
	} else {
		b.WriteString(headerStyle.Render(
			fmt.Sprintf("✓ Thought for %.1fs", state.Duration.Seconds())))
	}
	b.WriteString("\n")

	// Extract key points (simple heuristic: first line, last line, lines with "key" or "important")
	lines := strings.Split(state.Content, "\n")
	summary := extractKeyPoints(lines)

	summaryStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).
		Italic(true)

	for _, point := range summary {
		b.WriteString("  ")
		b.WriteString(summaryStyle.Render("• " + point))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

// extractKeyPoints extracts key points from thinking content.
func extractKeyPoints(lines []string) []string {
	var points []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Keep lines that look like key points
		lower := strings.ToLower(line)
		if strings.Contains(lower, "key") ||
			strings.Contains(lower, "important") ||
			strings.Contains(lower, "decision") ||
			strings.Contains(lower, "therefore") ||
			strings.Contains(lower, "conclusion") ||
			strings.Contains(lower, "need to") ||
			strings.Contains(lower, "should") {
			points = append(points, line)
		}
	}

	// If no key points found, use first and last non-empty lines
	if len(points) == 0 && len(lines) > 0 {
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				points = append(points, strings.TrimSpace(line))
				break
			}
		}
		if len(lines) > 1 {
			for i := len(lines) - 1; i >= 0; i-- {
				if strings.TrimSpace(lines[i]) != "" {
					points = append(points, strings.TrimSpace(lines[i]))
					break
				}
			}
		}
	}

	// Limit to 5 points
	if len(points) > 5 {
		points = points[:5]
	}

	return points
}

// ─── Thinking toggle command ─────────────────────────────────

// ToggleThinking expands/collapses the thinking block.
func (m *Model) ToggleThinking() {
	// This would be called when user presses a key to toggle thinking display
	// For now, just toggle the expanded state
	// In a real implementation, this would affect the message rendering
}
