package model

// ─── DialogSelect: Generic list selector ──────────────────────
//
// A reusable list dialog with:
//   - Filter / search input
//   - Keyboard navigation (↑↓ PgUp PgDn Home End)
//   - Category grouping
//   - Fuzzy matching on title
//   - Scrollbar for long lists
//
// Reference: opencode's DialogSelect (ui/dialog-select.tsx)

import (
	"fmt"
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Item types ───────────────────────────────────────────────

// DialogItem represents a single selectable item.
type DialogItem struct {
	Title       string
	Description string
	Category    string // group header (empty = no grouping)
	Value       string // the value to return when selected
	Subtitle    string // right-aligned text (e.g. keybinding hint)
	Disabled    bool   // cannot be selected
	Favorited   bool   // shown with ⭐
}

// DialogSelect is a generic list selection dialog.
type DialogSelect struct {
	title       string
	items       []DialogItem
	filtered    []int   // indices into items (after filtering)
	cursor      int     // index into filtered
	filter      string  // current filter text
	scroll      int     // scroll offset for visible items
	maxVisible  int     // max items visible at once
	showFilter  bool    // show filter input
	filterInput string  // the filter being typed
	width       int
	height      int

	onSelect func(DialogItem) (tea.Model, tea.Cmd) // returns the new model + cmd
}

// NewDialogSelect creates a new DialogSelect.
func NewDialogSelect(title string, items []DialogItem, maxVisible int, onSelect func(DialogItem) (tea.Model, tea.Cmd)) *DialogSelect {
	ds := &DialogSelect{
		title:      title,
		items:      items,
		cursor:     0,
		scroll:     0,
		maxVisible: maxVisible,
		showFilter: true,
		width:      60,
		height:     maxVisible + 6,
		onSelect:   onSelect,
	}
	ds.applyFilter()
	return ds
}

// ─── Dialog interface ─────────────────────────────────────────

func (d *DialogSelect) Type() DialogType   { return DialogPalette }
func (d *DialogSelect) Title() string       { return d.title }
func (d *DialogSelect) Height() int         { return d.height }

func (d *DialogSelect) View(width, height int) string {
	d.width = width
	if height > 0 {
		d.height = height
	}

	// Recalculate max visible
	d.maxVisible = height - 5 // account for title, filter, borders
	if d.maxVisible < 3 {
		d.maxVisible = 3
	}

	var b strings.Builder

	// Filter input
	if d.showFilter {
		filterLabel := "🔍 "
		filterDisplay := d.filter
		if filterDisplay == "" {
			filterDisplay = "type to filter..."
		}
		filterStyle := lipgloss.NewStyle().
			Width(width - 4).
			Foreground(theme.Colors.TextDim).
			Italic(true)
		if d.filter != "" {
			filterStyle = filterStyle.
				Foreground(theme.Colors.Text).
				Italic(false)
		}
		b.WriteString(filterStyle.Render(filterLabel + filterDisplay))
		b.WriteString("\n")
	}

	// Items list
	if len(d.filtered) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(theme.Colors.TextDim).
			Render("No results"))
		return b.String()
	}

	// Ensure cursor is visible
	d.ensureVisible()

	// Render visible items
	var lastCategory string
	visibleCount := 0
	for i := d.scroll; i < len(d.filtered) && visibleCount < d.maxVisible; i++ {
		idx := d.filtered[i]
		item := d.items[idx]
		isSelected := i == d.cursor

		// Category header
		if item.Category != "" && item.Category != lastCategory {
			if lastCategory != "" {
				b.WriteString("\n")
			}
			lastCategory = item.Category
			catStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.Colors.TextDim).
				Width(width - 4)
			b.WriteString(catStyle.Render(item.Category))
			b.WriteString("\n")
		}

		// Item line
		lineStyle := lipgloss.NewStyle().Width(width - 4)
		if isSelected {
			lineStyle = lineStyle.
				Background(theme.Colors.Selection).
				Foreground(theme.Colors.Text)
		} else {
			lineStyle = lineStyle.Foreground(theme.Colors.TextSoft)
		}
		if item.Disabled {
			lineStyle = lineStyle.Foreground(theme.Colors.TextDim)
		}

		// Prefix
		prefix := "  "
		if isSelected {
			prefix = "▸ "
		}
		if item.Favorited {
			prefix = "⭐ "
		}

		// Title (truncated)
		title := item.Title
		maxTitle := width - 8 - len(item.Subtitle)
		if maxTitle < 10 {
			maxTitle = 10
		}
		if len(title) > maxTitle {
			title = title[:maxTitle-1] + "…"
		}

		lineText := prefix + title
		if item.Subtitle != "" {
			// Right-align subtitle
			fill := width - 4 - len(lineText) - len(item.Subtitle)
			if fill < 1 {
				fill = 1
			}
			lineText += strings.Repeat(" ", fill) + item.Subtitle
		}

		b.WriteString(lineStyle.Render(lineText))
		b.WriteString("\n")
		visibleCount++
	}

	// Scroll indicator
	if len(d.filtered) > d.maxVisible {
		scrollInfo := fmt.Sprintf(" %d/%d ", d.cursor+1, len(d.filtered))
		b.WriteString(lipgloss.NewStyle().
			Foreground(theme.Colors.TextDim).
			Width(width - 4).
			Align(lipgloss.Right).
			Render(scrollInfo))
	}

	return b.String()
}

func (d *DialogSelect) Update(msg interface{}) (Dialog, interface{}) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			return d, d.closeCmd()

		case tea.KeyEnter:
			if len(d.filtered) > 0 && d.cursor >= 0 && d.cursor < len(d.filtered) {
				idx := d.filtered[d.cursor]
				item := d.items[idx]
				if !item.Disabled && d.onSelect != nil {
					_, cmd := d.onSelect(item)
					return d, cmd
				}
			}
			return d, nil

		case tea.KeyUp, tea.KeyShiftTab:
			d.cursor--
			if d.cursor < 0 {
				d.cursor = len(d.filtered) - 1
			}
			d.ensureVisible()

		case tea.KeyDown, tea.KeyTab:
			d.cursor++
			if d.cursor >= len(d.filtered) {
				d.cursor = 0
			}
			d.ensureVisible()

		case tea.KeyPgUp:
			d.cursor -= d.maxVisible
			if d.cursor < 0 {
				d.cursor = 0
			}
			d.ensureVisible()

		case tea.KeyPgDown:
			d.cursor += d.maxVisible
			if d.cursor >= len(d.filtered) {
				d.cursor = len(d.filtered) - 1
			}
			d.ensureVisible()

		case tea.KeyHome:
			d.cursor = 0
			d.scroll = 0

		case tea.KeyEnd:
			if len(d.filtered) > 0 {
				d.cursor = len(d.filtered) - 1
				d.ensureVisible()
			}

		case tea.KeyBackspace:
			if len(d.filter) > 0 {
				d.filter = d.filter[:len(d.filter)-1]
				d.applyFilter()
			}

		case tea.KeyRunes:
			d.filter += string(msg.Runes)
			d.applyFilter()

		case tea.KeyDelete:
			if len(d.filter) > 0 {
				d.filter = ""
				d.applyFilter()
			}
		}
	}
	return d, nil
}

// ─── Private ──────────────────────────────────────────────────

// applyFilter re-filters items based on the current filter text.
func (d *DialogSelect) applyFilter() {
	d.filtered = nil
	query := strings.ToLower(d.filter)

	for i, item := range d.items {
		if query == "" || strings.Contains(strings.ToLower(item.Title), query) ||
			(item.Description != "" && strings.Contains(strings.ToLower(item.Description), query)) {
			d.filtered = append(d.filtered, i)
		}
	}

	// Reset cursor
	if d.cursor >= len(d.filtered) {
		d.cursor = 0
	}
	d.scroll = 0
}

// ensureVisible scrolls so the cursor is visible.
func (d *DialogSelect) ensureVisible() {
	if d.cursor < d.scroll {
		d.scroll = d.cursor
	}
	if d.cursor >= d.scroll+d.maxVisible {
		d.scroll = d.cursor - d.maxVisible + 1
	}
}

// closeCmd returns a nil command (the caller should interpret dialog close).
func (d *DialogSelect) closeCmd() interface{} {
	return nil
}
