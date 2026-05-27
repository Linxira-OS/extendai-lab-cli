package model

import (
	"fmt"
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// ─── Command Palette (CodeWhale-style) ───────────────────────

// PaletteEntry represents a single command in the palette.
type PaletteEntry struct {
	Section     PaletteSection
	Label       string
	Description string
	Command     string
	Action      string // "execute" or "insert"
}

// PaletteSection categorizes palette entries.
type PaletteSection int

const (
	PaletteCommand PaletteSection = iota
	PaletteSkill
	PaletteTool
)

func (s PaletteSection) String() string {
	switch s {
	case PaletteCommand:
		return "Command"
	case PaletteSkill:
		return "Skill"
	case PaletteTool:
		return "Tool"
	default:
		return "Other"
	}
}

func (s PaletteSection) Color() lipgloss.Color {
	switch s {
	case PaletteCommand:
		return theme.Colors.Primary
	case PaletteSkill:
		return theme.Colors.Success
	case PaletteTool:
		return theme.Colors.Accent
	default:
		return theme.Colors.TextDim
	}
}

// CommandPalette holds the palette state.
type CommandPalette struct {
	Entries    []PaletteEntry
	Filtered   []int
	Query      string
	Selected   int
	ScrollOffset int
}

// NewCommandPalette creates a palette with all available commands.
func NewCommandPalette() *CommandPalette {
	entries := []PaletteEntry{
		// Commands
		{Section: PaletteCommand, Label: "/help", Description: "Show help", Command: "help", Action: "execute"},
		{Section: PaletteCommand, Label: "/clear", Description: "Clear conversation", Command: "clear", Action: "execute"},
		{Section: PaletteCommand, Label: "/model", Description: "Show/set model", Command: "model", Action: "insert"},
		{Section: PaletteCommand, Label: "/theme", Description: "Switch theme", Command: "theme", Action: "insert"},
		{Section: PaletteCommand, Label: "/panel", Description: "Cycle panel mode", Command: "panel", Action: "execute"},
		{Section: PaletteCommand, Label: "/compact", Description: "Compact context", Command: "compact", Action: "execute"},
		{Section: PaletteCommand, Label: "/context", Description: "Show context inspector", Command: "context", Action: "execute"},
		{Section: PaletteCommand, Label: "/session", Description: "Session management", Command: "session", Action: "insert"},
		{Section: PaletteCommand, Label: "/exit", Description: "Quit", Command: "exit", Action: "execute"},
	}

	p := &CommandPalette{
		Entries: entries,
	}
	p.updateFilter()
	return p
}

// updateFilter applies the current query to filter entries.
func (p *CommandPalette) updateFilter() {
	p.Filtered = nil
	query := strings.ToLower(strings.TrimSpace(p.Query))

	for i, entry := range p.Entries {
		if query == "" || fuzzyMatch(query, entry.Label) || fuzzyMatch(query, entry.Description) {
			p.Filtered = append(p.Filtered, i)
		}
	}

	// Reset selection if out of bounds
	if p.Selected >= len(p.Filtered) {
		p.Selected = 0
	}
	if p.Selected < 0 {
		p.Selected = 0
	}
}

// fuzzyMatch checks if query matches target (case-insensitive substring match).
func fuzzyMatch(query, target string) bool {
	return strings.Contains(strings.ToLower(target), strings.ToLower(query))
}

// HandleInput processes keyboard input for the palette.
func (p *CommandPalette) HandleInput(key string) (string, bool) {
	switch key {
	case "up", "ctrl+p":
		p.Selected--
		if p.Selected < 0 {
			p.Selected = len(p.Filtered) - 1
		}
		p.adjustScroll()
		return "", false
	case "down":
		p.Selected++
		if p.Selected >= len(p.Filtered) {
			p.Selected = 0
		}
		p.adjustScroll()
		return "", false
	case "enter":
		if len(p.Filtered) > 0 && p.Selected < len(p.Filtered) {
			entry := p.Entries[p.Filtered[p.Selected]]
			return entry.Command, entry.Action == "execute"
		}
		return "", false
	case "escape":
		return "", false
	case "backspace":
		if len(p.Query) > 0 {
			runes := []rune(p.Query)
			p.Query = string(runes[:len(runes)-1])
			p.updateFilter()
		}
		return "", false
	default:
		// Regular character input
		if len(key) == 1 {
			p.Query += key
			p.updateFilter()
		}
		return "", false
	}
}

// adjustScroll ensures the selected item is visible.
func (p *CommandPalette) adjustScroll() {
	maxVisible := 10 // typical palette height
	if p.Selected < p.ScrollOffset {
		p.ScrollOffset = p.Selected
	}
	if p.Selected >= p.ScrollOffset+maxVisible {
		p.ScrollOffset = p.Selected - maxVisible + 1
	}
}

// Render renders the command palette.
func (p *CommandPalette) Render(width, height int) string {
	if width < 30 || height < 8 {
		return ""
	}

	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Primary).
		Bold(true).
		Padding(0, 1)
	b.WriteString(titleStyle.Render("Command Palette"))
	b.WriteString("\n")

	// Search box
	searchStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Text).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Colors.Primary).
		Padding(0, 1).
		Width(width - 4)
	query := p.Query
	if query == "" {
		query = "Type to search..."
	}
	b.WriteString(searchStyle.Render(query))
	b.WriteString("\n\n")

	// Entries
	maxVisible := height - 6 // title + search + padding
	if maxVisible < 3 {
		maxVisible = 3
	}

	start := p.ScrollOffset
	end := start + maxVisible
	if end > len(p.Filtered) {
		end = len(p.Filtered)
	}

	for i := start; i < end; i++ {
		entry := p.Entries[p.Filtered[i]]
		isSelected := i == p.Selected

		// Section badge
		sectionBadge := lipgloss.NewStyle().
			Foreground(entry.Section.Color()).
			Render(fmt.Sprintf("[%s]", entry.Section))

		// Label
		labelStyle := lipgloss.NewStyle()
		if isSelected {
			labelStyle = labelStyle.Foreground(theme.Colors.Primary).Bold(true)
		} else {
			labelStyle = labelStyle.Foreground(theme.Colors.Text)
		}
		label := labelStyle.Render(entry.Label)

		// Description
		descStyle := lipgloss.NewStyle().Foreground(theme.Colors.TextDim)
		desc := descStyle.Render(entry.Description)

		// Selection indicator
		indicator := "  "
		if isSelected {
			indicator = lipgloss.NewStyle().Foreground(theme.Colors.Primary).Render("▸ ")
		}

		line := fmt.Sprintf("%s%s %s %s", indicator, sectionBadge, label, desc)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Footer hint
	b.WriteString("\n")
	hintStyle := lipgloss.NewStyle().Foreground(theme.Colors.TextDim)
	b.WriteString(hintStyle.Render("↑↓ navigate  Enter select  Esc close"))

	return b.String()
}
