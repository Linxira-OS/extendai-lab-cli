package model

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// ─── Context Inspector (CodeWhale-style) ─────────────────────

// ContextUsage holds the computed context statistics.
type ContextUsage struct {
	UsedTokens  int
	MaxTokens   int
	Percent     float64
	Status      string // "ok" | "high" | "critical"
	RoleStats   []RoleStat
	MessageCount int
}

// RoleStat holds per-role character/token counts.
type RoleStat struct {
	Role  string
	Name  string
	Color lipgloss.Color
	Chars int
	Tokens int // estimated: chars / 4
	Pct    float64
}

// buildContextUsage computes context usage from session messages.
func (m *Model) buildContextUsage() ContextUsage {
	if m.session == nil {
		return ContextUsage{Status: "no session"}
	}

	stats := []RoleStat{
		{Role: RoleSystem, Name: "sys", Color: theme.Colors.Primary},
		{Role: RoleUser, Name: "usr", Color: theme.Colors.Success},
		{Role: RoleAssistant, Name: "asst", Color: theme.Colors.Accent},
		{Role: RoleTool, Name: "tool", Color: theme.Colors.Warning},
		{Role: RoleError, Name: "err", Color: theme.Colors.Error},
	}

	totalChars := 0
	msgs := m.session.GetMessages()
	for _, msg := range msgs {
		for i, s := range stats {
			if msg.Role == s.Role {
				c := utf8.RuneCountInString(msg.Content)
				stats[i].Chars += c
				totalChars += c
				break
			}
		}
	}

	// Conservative token estimation: chars / 4
	totalTokens := totalChars / 4

	// Get max context from model info
	maxTokens := 128000 // default
	if m.apiClient != nil && m.apiClient.Info() != nil && m.apiClient.Info().ContextLength > 0 {
		maxTokens = m.apiClient.Info().ContextLength
	}

	pct := float64(totalTokens) / float64(maxTokens) * 100
	if pct > 100 {
		pct = 100
	}

	status := "ok"
	if pct >= ContextCriticalPct {
		status = "critical"
	} else if pct >= ContextWarningPct {
		status = "high"
	}

	// Calculate per-role percentages
	for i := range stats {
		if totalChars > 0 {
			stats[i].Tokens = stats[i].Chars / 4
			stats[i].Pct = float64(stats[i].Chars) / float64(totalChars) * 100
		}
	}

	return ContextUsage{
		UsedTokens:   totalTokens,
		MaxTokens:    maxTokens,
		Percent:      pct,
		Status:       status,
		RoleStats:    stats,
		MessageCount: len(msgs),
	}
}

// ─── Render context inspector for sidebar ────────────────────

func (m *Model) renderContextSidebar(width int) string {
	usage := m.buildContextUsage()
	if usage.Status == "no session" {
		return "no session"
	}

	var b strings.Builder

	// Header: status + tokens
	statusColor := theme.Colors.Success
	switch usage.Status {
	case "high":
		statusColor = theme.Colors.Warning
	case "critical":
		statusColor = theme.Colors.Error
	}

	statusIcon := lipgloss.NewStyle().Foreground(statusColor).Render("●")
	header := fmt.Sprintf("%s %s", statusIcon,
		lipgloss.NewStyle().Foreground(theme.Colors.Text).Render(
			fmt.Sprintf("~%s / %s (%.1f%%)",
				formatTokenK(usage.UsedTokens),
				formatTokenK(usage.MaxTokens),
				usage.Percent)))
	b.WriteString(header)
	b.WriteString("\n")

	// Message count
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(
		fmt.Sprintf("%d messages", usage.MessageCount)))
	b.WriteString("\n\n")

	// Per-role breakdown with colored bars
	barMax := width - 16 // room for "name 100% ███"
	if barMax < 4 {
		barMax = 4
	}

	for _, s := range usage.RoleStats {
		if s.Chars == 0 {
			continue
		}

		// Name column (right-aligned, 4 chars)
		nameStr := lipgloss.NewStyle().Foreground(s.Color).Render(
			fmt.Sprintf("%4s", s.Name))

		// Bar
		barLen := int(s.Pct * float64(barMax) / 100)
		if barLen < 1 && s.Pct > 0 {
			barLen = 1
		}
		bar := lipgloss.NewStyle().Foreground(s.Color).Render(
			strings.Repeat("█", barLen))

		// Percentage
		pctStr := lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(
			fmt.Sprintf("%3.0f%%", s.Pct))

		b.WriteString(fmt.Sprintf("%s %s %s", nameStr, bar, pctStr))
		b.WriteString("\n")
	}

	// Token detail line
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(
		fmt.Sprintf("≈%s tokens (chars/4)", formatTokenK(usage.UsedTokens))))

	return strings.TrimRight(b.String(), "\n")
}

// ─── Render context inspector popup (full detail) ────────────

func (m *Model) renderContextInspector(width int) string {
	usage := m.buildContextUsage()
	if usage.Status == "no session" {
		return "no session"
	}

	var b strings.Builder

	// Title
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.Primary).
		Bold(true).
		Render("Session Context"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", width-2))
	b.WriteString("\n\n")

	// Summary
	statusColor := theme.Colors.Success
	switch usage.Status {
	case "high":
		statusColor = theme.Colors.Warning
	case "critical":
		statusColor = theme.Colors.Error
	}

	b.WriteString(fmt.Sprintf("Status: %s\n",
		lipgloss.NewStyle().Foreground(statusColor).Render(usage.Status)))
	b.WriteString(fmt.Sprintf("Tokens: ~%s / %s (%.1f%%)\n",
		formatTokenK(usage.UsedTokens),
		formatTokenK(usage.MaxTokens),
		usage.Percent))
	b.WriteString(fmt.Sprintf("Messages: %d\n", usage.MessageCount))
	b.WriteString("\n")

	// Role breakdown
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.Text).Bold(true).Render("Role Breakdown"))
	b.WriteString("\n")

	for _, s := range usage.RoleStats {
		if s.Chars == 0 {
			continue
		}

		colorBar := lipgloss.NewStyle().Foreground(s.Color).Render("■")
		b.WriteString(fmt.Sprintf("  %s %-6s  %6d chars  %6s tokens  %5.1f%%\n",
			colorBar,
			lipgloss.NewStyle().Foreground(s.Color).Render(s.Name),
			s.Chars,
			formatTokenK(s.Tokens),
			s.Pct))
	}

	b.WriteString("\n")

	// Model info
	if m.apiClient != nil && m.apiClient.Info() != nil {
		info := m.apiClient.Info()
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.Text).Bold(true).Render("Model Info"))
		b.WriteString("\n")

		if info.ContextLength > 0 {
			b.WriteString(fmt.Sprintf("  Context Window: %s\n", formatTokenK(info.ContextLength)))
		}
		if info.SupportsVision {
			b.WriteString("  Vision: ✓\n")
		}
		if info.SupportsTools {
			b.WriteString("  Tools: ✓\n")
		}
		if info.SupportsReasoning {
			b.WriteString("  Reasoning: ✓\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(
		"Token estimate: chars ÷ 4 (conservative)"))

	return strings.TrimRight(b.String(), "\n")
}
