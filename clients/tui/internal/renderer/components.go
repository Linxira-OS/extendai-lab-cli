package renderer

import (
	"fmt"
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// ─── header ─────────────────────────────────────────────────

func renderHeader(c protocol.Component, ctx *RenderContext) string {
	title, _ := c.Props["title"].(string)
	if title == "" {
		title = "ExtendAI Lab"
	}
	// Status indicator
	status, _ := c.Props["status"].(string)
	indicator := lipgloss.NewStyle().Foreground(theme.Colors.Success).Render("●")
	if status == "disconnected" {
		indicator = lipgloss.NewStyle().Foreground(theme.Colors.Error).Render("●")
	}
	return theme.HeaderStyle.Width(ctx.Width).Render(indicator + " " + title)
}

// ─── message-list ───────────────────────────────────────────

func renderMessageList(c protocol.Component, ctx *RenderContext) string {
	var b strings.Builder
	for _, child := range c.Children {
		b.WriteString(renderMessage(child, ctx))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// ─── message ────────────────────────────────────────────────

func renderMessage(c protocol.Component, ctx *RenderContext) string {
	role, _ := c.Props["role"].(string)
	content, _ := c.Props["content"].(string)

	// Role label
	var label string
	var labelStyle lipgloss.Style
	switch role {
	case "user":
		label = "┌ You:"
		labelStyle = theme.UserMsgStyle
	case "assistant":
		label = "┌ Assistant:"
		labelStyle = theme.AssistantMsgStyle
	case "system":
		label = "──"
		labelStyle = theme.StatusStyle
	case "error":
		label = "✗ Error:"
		labelStyle = theme.ErrorStyle
	case "tool":
		label = "┌ Tool:"
		labelStyle = lipgloss.NewStyle().Foreground(theme.Colors.Accent).Bold(true)
	default:
		label = ">"
		labelStyle = theme.AssistantMsgStyle
	}

	var b strings.Builder
	b.WriteString(labelStyle.Render(label))
	b.WriteString("\n")

	// Render content as markdown
	mdWidth := ctx.Width - 4
	if mdWidth < 20 {
		mdWidth = 20
	}
	rendered := RenderMarkdown(content, mdWidth)
	b.WriteString(rendered)

	// Close bracket for user/assistant/tool
	switch role {
	case "user":
		b.WriteString("\n")
		b.WriteString(theme.UserMsgStyle.Render("└"))
	case "assistant":
		b.WriteString("\n")
		b.WriteString(theme.AssistantMsgStyle.Render("└"))
	case "tool":
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.Accent).Render("└"))
	}

	return b.String()
}

// ─── markdown ────────────────────────────────────────────────

func renderMarkdown(c protocol.Component, ctx *RenderContext) string {
	content, _ := c.Props["content"].(string)
	width := ctx.Width - 4
	if w, ok := c.Props["maxWidth"].(float64); ok {
		width = int(w)
	}
	return RenderMarkdown(content, width)
}

// ─── table ──────────────────────────────────────────────────

func renderTable(c protocol.Component, ctx *RenderContext) string {
	props := ParseSimpleTable(c.Props)
	return RenderTable(props, ctx.Width)
}

// ─── status-bar ─────────────────────────────────────────────

func renderStatusBar(c protocol.Component, ctx *RenderContext) string {
	left := parseStatusItems(c.Props["left"])
	center := parseStatusItems(c.Props["center"])
	right := parseStatusItems(c.Props["right"])

	// Build three sections
	leftText := joinStatusItems(left)
	centerText := joinStatusItems(center)
	rightText := joinStatusItems(right)

	w := ctx.Width
	sectionW := w / 3

	leftRendered := lipgloss.NewStyle().Width(sectionW).Align(lipgloss.Left).Render(leftText)
	centerRendered := lipgloss.NewStyle().Width(sectionW).Align(lipgloss.Center).Render(centerText)
	rightRendered := lipgloss.NewStyle().Width(w - sectionW*2).Align(lipgloss.Right).Render(rightText)

	return theme.FooterStyle.Width(w).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, leftRendered, centerRendered, rightRendered),
	)
}

func parseStatusItems(v interface{}) []protocol.StatusItem {
	var items []protocol.StatusItem
	if arr, ok := v.([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				si := protocol.StatusItem{}
				if text, ok := m["text"].(string); ok {
					si.Text = text
				}
				if action, ok := m["action"].(string); ok {
					si.Action = action
				}
				items = append(items, si)
			}
		}
	}
	return items
}

func joinStatusItems(items []protocol.StatusItem) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = item.Text
	}
	return strings.Join(parts, "  ")
}

// ─── input ──────────────────────────────────────────────────

func renderInput(c protocol.Component, ctx *RenderContext) string {
	placeholder, _ := c.Props["placeholder"].(string)
	if placeholder == "" {
		placeholder = "Type a message..."
	}
	value, _ := c.Props["value"].(string)

	prompt := "> "
	if v, ok := c.Props["prompt"].(string); ok {
		prompt = v
	}

	display := prompt + value
	if value == "" {
		display = prompt + placeholder
	}

	return theme.InputStyle.Width(ctx.Width - 4).Render(display)
}

// ─── panel ──────────────────────────────────────────────────

func renderPanel(c protocol.Component, ctx *RenderContext) string {
	title, _ := c.Props["title"].(string)
	borderColor, _ := c.Props["borderColor"].(string)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(ctx.Width - 2)

	if borderColor != "" {
		style = style.BorderForeground(lipgloss.Color(borderColor))
	} else {
		style = style.BorderForeground(theme.Colors.Muted)
	}

	var content string
	if len(c.Children) > 0 {
		registry := NewRegistry()
		content = registry.RenderAll(c.Children, ctx)
	} else {
		content, _ = c.Props["content"].(string)
	}

	if title != "" {
		content = lipgloss.NewStyle().Bold(true).Foreground(theme.Colors.Primary).Render(title) + "\n" + content
	}

	return style.Render(content)
}

// ─── progress ───────────────────────────────────────────────

func renderProgress(c protocol.Component, ctx *RenderContext) string {
	label, _ := c.Props["label"].(string)
	pct := 0.0
	if v, ok := c.Props["percent"].(float64); ok {
		pct = v
	}
	width := ctx.Width - len(label) - 4
	if width < 10 {
		width = 10
	}

	filled := int(float64(width) * pct / 100.0)
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	bar = lipgloss.NewStyle().Foreground(theme.Colors.Primary).Render(bar)

	if label != "" {
		return fmt.Sprintf("%s %s %3.0f%%", label, bar, pct)
	}
	return fmt.Sprintf("%s %3.0f%%", bar, pct)
}

// ─── spinner ────────────────────────────────────────────────

func renderSpinner(c protocol.Component, ctx *RenderContext) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := 0
	if v, ok := c.Props["frame"].(float64); ok {
		idx = int(v) % len(frames)
	}
	label, _ := c.Props["label"].(string)

	spinner := lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render(frames[idx])
	if label != "" {
		return spinner + " " + label
	}
	return spinner
}

// ─── tool-output ────────────────────────────────────────────

func renderToolOutput(c protocol.Component, ctx *RenderContext) string {
	content, _ := c.Props["content"].(string)
	// Tool output uses monospace, no markdown processing
	return lipgloss.NewStyle().
		Background(theme.Colors.Surface).
		Padding(0, 1).
		Width(ctx.Width - 2).
		Render(content)
}

// ─── diff ───────────────────────────────────────────────────

func renderDiff(c protocol.Component, ctx *RenderContext) string {
	left, _ := c.Props["left"].(string)
	right, _ := c.Props["right"].(string)
	leftLabel, _ := c.Props["leftLabel"].(string)
	rightLabel, _ := c.Props["rightLabel"].(string)

	if leftLabel == "" {
		leftLabel = "Original"
	}
	if rightLabel == "" {
		rightLabel = "Modified"
	}

	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Colors.Primary)

	var b strings.Builder
	halfW := (ctx.Width - 6) / 2
	if halfW < 20 {
		halfW = 20
	}

	// Labels
	b.WriteString(labelStyle.Render(fmt.Sprintf(" %-*s │ %s", halfW, leftLabel, rightLabel)))
	b.WriteRune('\n')

	// Divider
	div := strings.Repeat("─", halfW) + "─┼─" + strings.Repeat("─", halfW)
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.Muted).Render(div))
	b.WriteRune('\n')

	// Left and right content
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	removedStyle := lipgloss.NewStyle().Foreground(theme.Colors.Error)
	addedStyle := lipgloss.NewStyle().Foreground(theme.Colors.Success)

	for i := 0; i < maxLines; i++ {
		leftLine := ""
		rightLine := ""
		if i < len(leftLines) {
			leftLine = truncate(leftLines[i], halfW)
		}
		if i < len(rightLines) {
			rightLine = truncate(rightLines[i], halfW)
		}

		// Color based on change type
		if i < len(leftLines) && i >= len(rightLines) {
			leftLine = removedStyle.Render(leftLine)
		} else if i >= len(leftLines) && i < len(rightLines) {
			rightLine = addedStyle.Render(rightLine)
		} else if i < len(leftLines) && i < len(rightLines) && leftLines[i] != rightLines[i] {
			leftLine = removedStyle.Render(leftLine)
			rightLine = addedStyle.Render(rightLine)
		}

		b.WriteString(fmt.Sprintf(" %-*s │ %s", halfW, leftLine, rightLine))
		b.WriteRune('\n')
	}

	return b.String()
}

// ─── tree ───────────────────────────────────────────────────

func renderTree(c protocol.Component, ctx *RenderContext) string {
	items, _ := c.Props["items"].([]interface{})
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	indent := 0
	if v, ok := c.Props["indent"].(float64); ok {
		indent = int(v)
	}

	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			label, _ := m["label"].(string)
			selected, _ := m["selected"].(bool)
			children, _ := m["children"].([]interface{})

			prefix := "○"
			if selected {
				prefix = "●"
			}

			indentStr := strings.Repeat("  ", indent)
			b.WriteString(lipgloss.NewStyle().
				Foreground(theme.Colors.Text).
				Render(fmt.Sprintf("%s%s %s", indentStr, prefix, label)))
			b.WriteRune('\n')

			if len(children) > 0 {
				childProps := make(map[string]interface{})
				childProps["items"] = children
				childProps["indent"] = float64(indent + 1)
				childComp := protocol.Component{
					Type:  protocol.CompTree,
					Props: childProps,
				}
				b.WriteString(renderTree(childComp, ctx))
			}
		}
	}

	return b.String()
}

// ─── RenderCommand → View ────────────────────────────────────

// RenderAllComponents renders all components from a RenderCommand.
func RenderAllComponents(cmd *protocol.RenderCommand, width int) string {
	ctx := &RenderContext{
		Width:    width,
		TabWidth: 4,
	}
	registry := NewRegistry()
	return registry.RenderAll(cmd.Components, ctx)
}
