package view

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Scrollbar style tokens ──────────────────────────────────

const (
	ScrollbarTrack   = "░"
	ScrollbarThumb   = "█"
	ScrollbarTrackV  = "│" // lighter vertical track
	ScrollbarThumbV  = "▌" // half-block thumb
)

// ─── VerticalScrollbar ───────────────────────────────────────

// RenderScrollbar renders a thin vertical scrollbar.
//   yOffset:    current scroll position (0-indexed)
//   totalLines: total number of content lines
//   height:     visible viewport height in rows
//   Returns a string of `height` rows, one character per row.
func RenderScrollbar(yOffset, totalLines, height int) string {
	if totalLines <= height || height < 2 {
		return "" // no scroll needed
	}

	thumbHeight := height * height / totalLines
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	if thumbHeight >= height {
		thumbHeight = height - 1
	}

	maxOffset := totalLines - height
	thumbPos := 0
	if maxOffset > 0 {
		thumbPos = (yOffset * (height - thumbHeight)) / maxOffset
	}
	if thumbPos+thumbHeight > height {
		thumbPos = height - thumbHeight
	}

	var b strings.Builder
	for i := 0; i < height; i++ {
		if i >= thumbPos && i < thumbPos+thumbHeight {
			b.WriteString(ScrollbarThumbV)
		} else {
			b.WriteString(ScrollbarTrack)
		}
	}
	return b.String()
}

// ─── Styled scrollbar ─────────────────────────────────────────

// RenderStyledScrollbar renders a scrollbar with styled track/thumb.
func RenderStyledScrollbar(yOffset, totalLines, height int, trackStyle, thumbStyle lipgloss.Style) string {
	raw := RenderScrollbar(yOffset, totalLines, height)
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "")
	for i, ch := range lines {
		if ch == ScrollbarThumbV {
			lines[i] = thumbStyle.Render(ch)
		} else {
			lines[i] = trackStyle.Render(ch)
		}
	}
	return strings.Join(lines, "\n")
}

// ─── Helpers ─────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
