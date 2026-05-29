package model

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// ─── Footer Props (pure data, no App reference) ──────────────

// FooterProps holds all data needed to render the footer.
// Built from Model, then passed to a pure render function.
type FooterProps struct {
	// Left region
	Dir           string
	ServerOK      bool

	// Center region: context bar
	ContextBar    string  // pre-rendered context bar string
	ContextPct    float64 // 0-100, for color thresholding

	// Right region
	ModelName     string
	ThemeName     string
	ModeLabel     string // "main" / "input" / "panel"
	ModeColor     lipgloss.Color

	// Working state (animated)
	IsWorking     bool
	WorkingLabel  string
	WorkingColor  lipgloss.Color
	SpinnerFrame  string

	// Cost tracking (future)
	SessionCost   string // e.g. "$0.03"
}

// ─── Context thresholds (CodeWhale-style) ────────────────────

const (
	ContextWarningPct  = 85.0
	ContextCriticalPct = 95.0
)

func contextStatusColor(pct float64) lipgloss.Color {
	switch {
	case pct >= ContextCriticalPct:
		return theme.Colors.Error
	case pct >= ContextWarningPct:
		return theme.Colors.Warning
	default:
		return theme.Colors.Success
	}
}

// ─── Build props from Model ──────────────────────────────────

func (m Model) buildFooterProps(width int) FooterProps {
	// Left: directory
	dir := m.currentDir
	if dir == "" {
		dir = "~"
	}

	// Center: context bar
	ctxBar, ctxPct := m.buildContextBar(width)

	// Right: mode label
	modeLabel := "[main]"
	modeColor := theme.Colors.TextDim
	switch m.focusZone {
	case FocusInput:
		modeLabel = "[input]"
		modeColor = theme.Colors.Primary
	case FocusPanel:
		modeLabel = "[panel]"
		modeColor = theme.Colors.Secondary
	}

	// Working state
	props := FooterProps{
		Dir:         dir,
		ServerOK:    m.serverConnected,
		ContextBar:  ctxBar,
		ContextPct:  ctxPct,
		ModelName:   m.modelName,
		ThemeName:   theme.ThemeDisplayName(theme.CurrentTheme),
		ModeLabel:   modeLabel,
		ModeColor:   modeColor,
		IsWorking:   m.ai.IsWorking(),
		WorkingLabel: m.ai.Label,
		SpinnerFrame: m.ai.SpinnerFrame(),
	}

	// Set working color based on AI status
	if m.ai.Status == protocol.AIStreaming {
		props.WorkingColor = theme.Colors.Primary
	} else if m.ai.Status == protocol.AIThinking {
		props.WorkingColor = theme.Colors.Accent
	}

	return props
}

// ─── Build context bar (segmented colored blocks) ────────────

func (m *Model) buildContextBar(width int) (string, float64) {
	if m.session == nil || width < 20 {
		return "", 0
	}

	type stat struct {
		color  lipgloss.Color
		chars  int
		tokens int // CJK-aware token estimate
	}

	stats := []stat{
		{color: theme.Colors.Primary},  // system
		{color: theme.Colors.Success},  // user
		{color: theme.Colors.Accent},   // assistant
		{color: theme.Colors.Warning},  // tool
		{color: theme.Colors.Error},    // error
	}
	roles := []string{RoleSystem, RoleUser, RoleAssistant, RoleTool, RoleError}

	totalTokens := 0
	for _, msg := range m.session.GetMessages() {
		for i, role := range roles {
			if msg.Role == role {
				c := utf8.RuneCountInString(msg.Content)
				t := EstimateTokens(msg.Content)
				stats[i].chars += c
				stats[i].tokens += t
				totalTokens += t
				break
			}
		}
	}

	// Calculate max tokens from model info
	maxTokens := 128000
	if m.apiClient != nil && m.apiClient.Info() != nil && m.apiClient.Info().ContextLength > 0 {
		maxTokens = m.apiClient.Info().ContextLength
	}

	// Choose format based on available width
	// Narrow: "███░░ 0.1K/128K" (no spaces around /)
	// Wide:   "███░░░░░░░░ 0.1K / 128K"
	isNarrow := width < 60

	// Reserve space for text label
	textReserve := 18 // " 0.1K / 128K"
	if isNarrow {
		textReserve = 14 // " 0.1K/128K"
	}
	barWidth := width - textReserve
	if barWidth < 5 {
		barWidth = 5
	}

	// If no messages, show empty bar (all gray)
	if totalTokens == 0 {
		bar := lipgloss.NewStyle().
			Background(theme.Colors.Muted).
			Render(strings.Repeat(" ", barWidth))
		var text string
		if isNarrow {
			text = lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(
				fmt.Sprintf("%s %.1fK/%s", bar, 0.0, formatTokenK(maxTokens)))
		} else {
			text = lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(
				fmt.Sprintf("%s 0 / %s", bar, formatTokenK(maxTokens)))
		}
		return text, 0
	}

	// Build segments: each segment width = tokens / maxTokens * barWidth
	var segments []string
	usedWidth := 0
	for _, s := range stats {
		if s.chars > 0 {
			segW := int(float64(s.tokens) / float64(maxTokens) * float64(barWidth))
			if segW == 0 && s.chars > 0 {
				segW = 1
			}
			// Cap segment width to prevent overflow
			if usedWidth+segW > barWidth {
				segW = barWidth - usedWidth
			}
			if segW > 0 {
				segments = append(segments, lipgloss.NewStyle().
					Background(s.color).
					Render(strings.Repeat(" ", segW)))
				usedWidth += segW
			}
		}
	}

	// Add gray "unused" segment to fill remaining bar width
	remaining := barWidth - usedWidth
	if remaining > 0 {
		segments = append(segments, lipgloss.NewStyle().
			Background(theme.Colors.Muted).
			Render(strings.Repeat(" ", remaining)))
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, segments...)

	// Format text (using CJK-aware token estimate)
	currentK := formatTokenK(totalTokens)
	totalK := formatTokenK(maxTokens)

	// Color based on usage percentage
	pct := float64(totalTokens) / float64(maxTokens) * 100
	if pct > 100 {
		pct = 100
	}
	ctxColor := contextStatusColor(pct)

	var text string
	if isNarrow {
		text = lipgloss.NewStyle().Foreground(ctxColor).Render(
			fmt.Sprintf("%s %.1fK/%s", bar, float64(totalTokens)/1000, totalK))
	} else {
		text = lipgloss.NewStyle().Foreground(ctxColor).Render(
			fmt.Sprintf("%s %s / %s", bar, currentK, totalK))
	}

	return text, pct
}

func formatTokenK(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000.0)
	}
	return fmt.Sprintf("%.1fK", float64(tokens)/1000.0)
}

// ─── Pure footer render ─────────────────────────────────────

func (m Model) renderFooter(width int) string {
	props := m.buildFooterProps(width)
	return renderFooterFromProps(width, props)
}

func renderFooterFromProps(width int, p FooterProps) string {
	// ── Left region: dir + server dot ──
	dotColor := theme.Colors.Success
	if !p.ServerOK {
		dotColor = theme.Colors.Error
	}
	left := lipgloss.NewStyle().Foreground(dotColor).Render("●") + " " +
		lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(p.Dir)

	// ── Right region: mode + model + theme ──
	rightParts := []string{
		lipgloss.NewStyle().Foreground(p.ModeColor).Render(p.ModeLabel),
	}
	if p.ModelName != "" {
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render(p.ModelName))
	}
	rightParts = append(rightParts,
		lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(p.ThemeName))
	right := strings.Join(rightParts, "  ")

	// ── Center region: always show context bar ──
	// If working, overlay a small spinner on the right side of the bar
	center := p.ContextBar
	if p.IsWorking && p.SpinnerFrame != "" {
		label := p.WorkingLabel
		if label == "" {
			label = "working"
		}
		spinnerText := lipgloss.NewStyle().Foreground(p.WorkingColor).Render(
			fmt.Sprintf("%s %s", p.SpinnerFrame, label))
		// Append spinner after context bar, with padding
		center = center + "  " + spinnerText

		// Add water-spout animation (CodeWhale-style)
		// Use a small wave (8 chars) to show activity
		frame := int(time.Now().UnixMilli() / 100)
		wave := waterSpoutAnimation(frame, 8)
		waveText := lipgloss.NewStyle().Foreground(p.WorkingColor).Render(wave)
		center = center + " " + waveText
	}

	// ── Layout: left | center | right ──
	leftW := lipgloss.Width(left) + 2   // padding
	rightW := lipgloss.Width(right) + 2 // padding
	centerW := width - leftW - rightW
	if centerW < 0 {
		centerW = 0
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Padding(0, 1).Render(left),
		lipgloss.NewStyle().Padding(0, 1).Width(centerW).Align(lipgloss.Center).Render(center),
		lipgloss.NewStyle().Padding(0, 1).Render(right),
	)
}

// ─── Footer working state animation (CodeWhale-style) ───────

// footerWorkingLabel returns the animated working label.
// Rotates through frames: working | working . | working .. | working ...
func footerWorkingLabel(frame int) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[frame%len(frames)]
}

// waterSpoutAnimation returns a water-spout wave animation string.
// Uses sine wave formula: primary = sin(x*0.52 - t*8.0)
// Characters: ▁▂▃▄▅▆▇█▅▃▂▁
func waterSpoutAnimation(t int, width int) string {
	if width < 4 {
		return ""
	}

	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█', '▇', '▆', '▅', '▄', '▃', '▂'}

	var result []rune
	for x := 0; x < width; x++ {
		// Sine wave formula
		val := math.Sin(float64(x)*0.52 - float64(t)*0.8)
		// Map to 0-1 range
		idx := int((val + 1) / 2 * float64(len(chars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(chars) {
			idx = len(chars) - 1
		}
		result = append(result, chars[idx])
	}

	return string(result)
}

// ─── Footer toast (status messages) ─────────────────────────

type FooterToast struct {
	Text  string
	Color lipgloss.Color
}

func (m Model) activeFooterToast() *FooterToast {
	// Quit confirmation takes precedence
	if m.ai.Status == protocol.AIError && m.ai.ErrorMsg != "" {
		return &FooterToast{
			Text:  m.ai.ErrorMsg,
			Color: theme.Colors.Error,
		}
	}
	return nil
}
