package renderer

import (
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// RenderTable renders a structured table with proper column alignment.
//
// Terminal rendering of tables requires special care:
//   - Tabs (\t) in cell content must be replaced with spaces
//   - Column alignment: left/center/right using padding
//   - UTF-8 box-drawing for borders (┌─┬─┐ │ │ └─┴─┘)
//   - Long text truncated with "..." to preserve column structure
func RenderTable(props protocol.TableRenderProps, width int) string {
	if len(props.Headers) == 0 && len(props.Rows) == 0 {
		return ""
	}

	tabReplace := props.TabReplace
	if tabReplace == "" {
		tabReplace = "    " // 4 spaces
	}

	// Normalize: replace tabs in all cells
	for i := range props.Headers {
		props.Headers[i] = strings.ReplaceAll(props.Headers[i], "\t", tabReplace)
	}
	for ri := range props.Rows {
		for ci := range props.Rows[ri] {
			props.Rows[ri][ci] = strings.ReplaceAll(props.Rows[ri][ci], "\t", tabReplace)
		}
	}

	// Calculate column widths
	colCount := len(props.Headers)
	if colCount == 0 && len(props.Rows) > 0 {
		colCount = len(props.Rows[0])
	}
	if colCount == 0 {
		return ""
	}

	// Ensure alignment array is filled
	align := make([]string, colCount)
	for i := range align {
		if i < len(props.Alignment) {
			align[i] = props.Alignment[i]
		} else {
			align[i] = "left"
		}
	}

	// Calculate column widths (max content width per column)
	minColWidth := 4
	maxColWidth := 40
	colWidths := make([]int, colCount)

	// Initialize with header widths
	for i := 0; i < colCount && i < len(props.Headers); i++ {
		w := DisplayWidth(props.Headers[i])
		if w > colWidths[i] {
			colWidths[i] = w
		}
	}

	// Expand for cell content
	for _, row := range props.Rows {
		for i := 0; i < colCount && i < len(row); i++ {
			w := DisplayWidth(row[i])
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// Apply explicit column widths if provided
	for i := 0; i < colCount && i < len(props.ColumnWidths); i++ {
		if props.ColumnWidths[i] > 0 {
			colWidths[i] = props.ColumnWidths[i]
		}
	}

	// Cap at maxColWidth
	for i := range colWidths {
		if colWidths[i] > maxColWidth {
			colWidths[i] = maxColWidth
		}
	}

	// Calculate total table width
	totalWidth := 1 // left border
	for _, w := range colWidths {
		totalWidth += w + 3 // padding + border
	}

	// Clamp to terminal width if too wide
	availWidth := width - 4
	if availWidth < 20 {
		availWidth = 20
	}
	if totalWidth > availWidth {
		// Scale down proportionally
		scale := float64(availWidth-1-colCount) / float64(totalWidth-1-colCount)
		if scale < 0.3 {
			scale = 0.3
		}
		for i := range colWidths {
			colWidths[i] = int(float64(colWidths[i]) * scale)
			if colWidths[i] < minColWidth {
				colWidths[i] = minColWidth
			}
		}
	}

	// Build the table
	var b strings.Builder

	// Determine available styles
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Colors.Primary)
	borderStyle := lipgloss.NewStyle().Foreground(theme.Colors.Muted)
	cellStyle := lipgloss.NewStyle().Foreground(theme.Colors.Text)

	// Top border: ┌───┬───┐
	b.WriteString(borderStyle.Render("┌"))
	for i, w := range colWidths {
		b.WriteString(borderStyle.Render(strings.Repeat("─", w+2)))
		if i < len(colWidths)-1 {
			b.WriteString(borderStyle.Render("┬"))
		}
	}
	b.WriteString(borderStyle.Render("┐"))
	b.WriteRune('\n')

	// Header row
	if len(props.Headers) > 0 {
		b.WriteString(borderStyle.Render("│"))
		for i := 0; i < colCount; i++ {
			cell := ""
			if i < len(props.Headers) {
				cell = truncate(props.Headers[i], colWidths[i])
			}
			cell = padCell(cell, colWidths[i], align[i])
			b.WriteString(" ")
			b.WriteString(headerStyle.Render(cell))
			b.WriteString(" ")
			b.WriteString(borderStyle.Render("│"))
		}
		b.WriteRune('\n')

		// Header separator: ├───┼───┤
		b.WriteString(borderStyle.Render("├"))
		for i, w := range colWidths {
			sep := "─"
			if i < len(align) && align[i] == "center" {
				sep = "─" // could use :─: but standard is fine
			}
			b.WriteString(borderStyle.Render(strings.Repeat(sep, w+2)))
			if i < len(colWidths)-1 {
				b.WriteString(borderStyle.Render("┼"))
			}
		}
		b.WriteString(borderStyle.Render("┤"))
		b.WriteRune('\n')
	}

	// Data rows
	for _, row := range props.Rows {
		b.WriteString(borderStyle.Render("│"))
		for i := 0; i < colCount; i++ {
			cell := ""
			if i < len(row) {
				cell = truncate(row[i], colWidths[i])
			}
			cell = padCell(cell, colWidths[i], align[i])
			b.WriteString(" ")
			b.WriteString(cellStyle.Render(cell))
			b.WriteString(" ")
			b.WriteString(borderStyle.Render("│"))
		}
		b.WriteRune('\n')
	}

	// Bottom border: └───┴───┘
	b.WriteString(borderStyle.Render("└"))
	for i, w := range colWidths {
		b.WriteString(borderStyle.Render(strings.Repeat("─", w+2)))
		if i < len(colWidths)-1 {
			b.WriteString(borderStyle.Render("┴"))
		}
	}
	b.WriteString(borderStyle.Render("┘"))

	return b.String()
}

// padCell pads cell text to the specified width with proper alignment.
// Uses displayWidth for correct multi-byte / emoji handling.
func padCell(text string, width int, align string) string {
	dw := DisplayWidth(text)
	if dw >= width {
		return text
	}
	padding := width - dw
	switch align {
	case "right":
		return strings.Repeat(" ", padding) + text
	case "center":
		left := padding / 2
		right := padding - left
		return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
	default: // left
		return text + strings.Repeat(" ", padding)
	}
}

// truncate shortens text to maxWidth, adding "..." if truncated.
func truncate(text string, maxWidth int) string {
	runes := []rune(text)
	if len(runes) <= maxWidth {
		return text
	}
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-3]) + "..."
}

// displayWidth returns the visual width of a string.
// Accounts for CJK, fullwidth forms, and emoji that occupy 2 columns.
// DisplayWidth returns the visual width of a string.
// Exported for use by other packages (e.g., model for panel padding).
func DisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		switch {
		case r >= 0x4E00 && r <= 0x9FFF, // CJK Unified
			r >= 0x3000 && r <= 0x303F,   // CJK Symbols
			r >= 0xFF00 && r <= 0xFFEF,   // Fullwidth Forms
			r >= 0x1F300 && r <= 0x1F9FF, // Misc Symbols, Emoticons, Emoji
			r >= 0x200D,                   // Zero-width joiner (ZWJ emoji sequences)
			r == 0x231A || r == 0x231B,    // Watch, Hourglass
			r == 0x23F0 || r == 0x23F3,    // Alarm, Hourglass with flowing sand
			r >= 0x2600 && r <= 0x27BF,   // Misc symbols, Dingbats
			r >= 0xFE00 && r <= 0xFE0F:   // Variation selectors (emoji presentation)
			width += 2
		default:
			width++
		}
	}
	return width
}

// ParseSimpleTable parses a TS table component's props into TableRenderProps.
func ParseSimpleTable(props map[string]interface{}) protocol.TableRenderProps {
	r := protocol.TableRenderProps{
		TabReplace: "    ",
	}

	if headers, ok := props["headers"].([]interface{}); ok {
		for _, h := range headers {
			if s, ok := h.(string); ok {
				r.Headers = append(r.Headers, s)
			}
		}
	}
	if rows, ok := props["rows"].([]interface{}); ok {
		for _, row := range rows {
			if rArr, ok := row.([]interface{}); ok {
				var rowStr []string
				for _, cell := range rArr {
					if s, ok := cell.(string); ok {
						rowStr = append(rowStr, s)
					}
				}
				if len(rowStr) > 0 {
					r.Rows = append(r.Rows, rowStr)
				}
			}
		}
	}
	if align, ok := props["alignment"].([]interface{}); ok {
		for _, a := range align {
			if s, ok := a.(string); ok {
				r.Alignment = append(r.Alignment, s)
			}
		}
	}
	if widths, ok := props["columnWidths"].([]interface{}); ok {
		for _, w := range widths {
			if f, ok := w.(float64); ok {
				r.ColumnWidths = append(r.ColumnWidths, int(f))
			}
		}
	}
	return r
}
