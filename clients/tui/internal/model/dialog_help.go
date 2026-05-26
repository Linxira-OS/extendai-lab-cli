package model

// ─── Help Dialog ──────────────────────────────────────────────
//
// Comprehensive help dialog showing all keyboard shortcuts and commands.
// Triggered by: F1 key, /help command.

import (
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Help types ───────────────────────────────────────────────

type helpSection struct {
	title string
	rows  [][2]string // key, description pairs
}

// helpData defines all help sections and their keyboard rows.
var helpData = []helpSection{
	{
		title: "Navigation (Browse Mode)",
		rows: [][2]string{
			{"j / ↓", "Scroll down"},
			{"k / ↑", "Scroll up"},
			{"g", "Go to top"},
			{"G", "Go to bottom"},
			{"Ctrl+D", "Half page down"},
			{"Ctrl+U", "Half page up"},
			{"PgDown", "Half page down"},
			{"PgUp", "Half page up"},
			{"Enter", "Focus input"},
			{"Tab", "Cycle focus: main → panel → input"},
			{"Mouse scroll", "Scroll content"},
		},
	},
	{
		title: "Input Editing",
		rows: [][2]string{
			{"Enter", "Submit message"},
			{"Escape", "Unfocus input"},
			{"Ctrl+C", "Clear input"},
			{"Tab", "Command completion (in / mode)"},
			{"/", "Start slash command"},
			{"Backspace", "Delete character"},
		},
	},
	{
		title: "Right Panel (panel focused)",
		rows: [][2]string{
			{"h / ←", "Previous section"},
			{"l / →", "Next section"},
			{"j / ↓", "Scroll down"},
			{"k / ↑", "Scroll up"},
			{"Space", "Toggle section expand/collapse"},
			{"g", "Go to top"},
			{"G", "Go to bottom"},
			{"Esc", "Return to main"},
		},
	},
	{
		title: "Keyboard Shortcuts",
		rows: [][2]string{
			{"Ctrl+P", "Open command palette"},
			{"Ctrl+F", "Open model list"},
			{"F1", "Show this help"},
			{"Ctrl+C", "Clear / cancel"},
			{"Ctrl+D", "Quit (browse/panel mode)"},
			{"Tab", "Cycle focus zones"},
		},
	},
	{
		title: "Slash Commands",
		rows: [][2]string{
			{"/help", "Show help"},
			{"/clear", "Clear conversation"},
			{"/model", "List / set model"},
			{"/theme", "List / set theme"},
			{"/panel", "Toggle right panel (auto/hide)"},
			{"/exit", "Quit"},
		},
	},
}

// ─── HelpDialog ───────────────────────────────────────────────

// HelpDialog is a read-only help dialog with scrollable content.
type HelpDialog struct {
	content    string // pre-rendered content
	cachedW    int
	scroll     int
	maxVisible int
	height     int
}

// NewHelpDialog creates a pre-rendered help dialog.
func NewHelpDialog() *HelpDialog {
	return &HelpDialog{
		height: 30,
	}
}

// ─── Dialog interface ─────────────────────────────────────────

func (d *HelpDialog) Type() DialogType { return DialogHelpType }
func (d *HelpDialog) Title() string    { return "Help — ExtendAI Lab" }
func (d *HelpDialog) Height() int      { return d.height }

func (d *HelpDialog) View(width, height int) string {
	if height > 0 {
		d.height = height
	}
	d.maxVisible = height - 6 // borders + title + separators
	if d.maxVisible < 5 {
		d.maxVisible = 5
	}

	if d.content == "" || d.cachedW != width {
		d.content = d.renderContent(width - 6) // inner padding
		d.cachedW = width
	}

	lines := strings.Split(d.content, "\n")
	// Clamp scroll
	maxScroll := len(lines) - d.maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if d.scroll > maxScroll {
		d.scroll = maxScroll
	}
	if d.scroll < 0 {
		d.scroll = 0
	}

	end := d.scroll + d.maxVisible
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[d.scroll:end]

	return strings.Join(visible, "\n")
}

func (d *HelpDialog) Update(msg interface{}) (Dialog, interface{}) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			return d, nil
		case tea.KeyUp, tea.KeyShiftTab:
			d.scroll--
			if d.scroll < 0 {
				d.scroll = 0
			}
		case tea.KeyDown, tea.KeyTab:
			d.scroll++
		case tea.KeyPgUp:
			d.scroll -= d.maxVisible
			if d.scroll < 0 {
				d.scroll = 0
			}
		case tea.KeyPgDown:
			d.scroll += d.maxVisible
		case tea.KeyHome:
			d.scroll = 0
		case tea.KeyEnd:
			d.scroll = 99999
		}
	}
	return d, nil
}

// ─── Content rendering ───────────────────────────────────────

func (d *HelpDialog) renderContent(width int) string {
	keyStyle := lipgloss.NewStyle().Foreground(theme.Colors.Accent).Width(14)
	descStyle := lipgloss.NewStyle().Foreground(theme.Colors.TextSoft)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Colors.Primary)
	sepStyle := lipgloss.NewStyle().Foreground(theme.Colors.Muted)

	var b strings.Builder

	for i, section := range helpData {
		if i > 0 {
			b.WriteString("\n")
		}

		b.WriteString(sectionStyle.Render(section.title))
		b.WriteString("\n")
		b.WriteString(sepStyle.Render(strings.Repeat("─", width)))
		b.WriteString("\n")

		for _, row := range section.rows {
			b.WriteString(keyStyle.Render(row[0]))
			b.WriteString(descStyle.Render(row[1]))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(sepStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).Italic(true).
		Render("↑↓/PgUp/PgDn scroll · Esc close"))

	return b.String()
}
