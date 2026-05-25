package theme

import (
	"os"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// ─── Terminal Rendering Capabilities ────────────────────────
//
// Terminals CAN render:
//   ✓ Bold            \033[1m
//   ✓ Italic          \033[3m
//   ✓ Underline       \033[4m
//   ✓ Strikethrough   \033[9m
//   ✓ 256 colors      \033[38;5;Nm
//   ✓ True color      \033[38;2;R;G;Bm
//   ✓ Background color
//   ✓ Inverse/reverse \033[7m
//
// Terminals CANNOT render:
//   ✗ Font size changes (H1-H6 same physical size)
//   ✗ Different fonts (monospace only)
//   ✗ Letter spacing
//   ✗ Line height control
//   ✗ Images (only ASCII art)
//
// Therefore: heading levels are distinguished ONLY by:
//   - Color (primary → secondary → muted progression)
//   - Weight (bold → normal → italic progression)
//   - Decoration (optional underline for H1)

// ─── ThemeColors (can be overridden by TS RenderCommand) ────

type ThemeColors struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Success   lipgloss.Color
	Error     lipgloss.Color
	Warning   lipgloss.Color
	Muted     lipgloss.Color
	Surface   lipgloss.Color
	Base      lipgloss.Color
	Text      lipgloss.Color
	TextDim   lipgloss.Color
	CodeBg    lipgloss.Color // background for inline code
	Selection lipgloss.Color
}

// DefaultColors returns the default dark theme.
func DefaultColors() ThemeColors {
	return ThemeColors{
		Primary:   lipgloss.Color("#7C3AED"), // violet
		Secondary: lipgloss.Color("#06B6D4"), // cyan
		Accent:    lipgloss.Color("#F59E0B"), // amber
		Success:   lipgloss.Color("#10B981"), // emerald
		Error:     lipgloss.Color("#EF4444"), // red
		Warning:   lipgloss.Color("#F97316"), // orange
		Muted:     lipgloss.Color("#6B7280"), // gray
		Surface:   lipgloss.Color("#1F2937"), // dark gray
		Base:      lipgloss.Color("#111827"), // near black
		Text:      lipgloss.Color("#F3F4F6"), // light gray
		TextDim:   lipgloss.Color("#9CA3AF"), // dim gray
		CodeBg:    lipgloss.Color("#2D3748"), // code background
		Selection: lipgloss.Color("#374151"),
	}
}

// Global state
var (
	TermWidth  int
	TermHeight int
	Colors     ThemeColors

	// ── Component styles ──────────────────────────────────
	HeaderStyle   lipgloss.Style
	FooterStyle   lipgloss.Style
	InputStyle    lipgloss.Style
	StatusStyle   lipgloss.Style
	UserMsgStyle  lipgloss.Style
	AssistantMsgStyle lipgloss.Style
	ErrorStyle    lipgloss.Style
	HelpStyle     lipgloss.Style
	DividerStyle  lipgloss.Style
	CodeStyle     lipgloss.Style
	TableStyle    lipgloss.Style

	// ── Heading styles (terminal: no font size, only color + weight) ──
	//
	// H1: bold + primary color
	// H2: bold + secondary color
	// H3: bold only (no color change)
	// H4: italic + dim color
	// H5: italic
	// H6: dim only
	Heading1Style lipgloss.Style
	Heading2Style lipgloss.Style
	Heading3Style lipgloss.Style
	Heading4Style lipgloss.Style
	Heading5Style lipgloss.Style
	Heading6Style lipgloss.Style
)

// InitTerm initializes terminal and all styles.
// Call once at startup and on every WindowSizeMsg.
func InitTerm() {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, height = 120, 40
	}
	TermWidth = width
	TermHeight = height
	Colors = DefaultColors()
	recalcStyles()
}

// ApplyThemeOverrides applies theme overrides from a TS RenderCommand.
func ApplyThemeOverrides(overrides *protocol.ThemeColors) {
	if overrides == nil {
		return
	}
	if overrides.Primary != "" {
		Colors.Primary = lipgloss.Color(overrides.Primary)
	}
	if overrides.Secondary != "" {
		Colors.Secondary = lipgloss.Color(overrides.Secondary)
	}
	if overrides.Accent != "" {
		Colors.Accent = lipgloss.Color(overrides.Accent)
	}
	if overrides.Success != "" {
		Colors.Success = lipgloss.Color(overrides.Success)
	}
	if overrides.Error != "" {
		Colors.Error = lipgloss.Color(overrides.Error)
	}
	if overrides.Muted != "" {
		Colors.Muted = lipgloss.Color(overrides.Muted)
	}
	if overrides.Surface != "" {
		Colors.Surface = lipgloss.Color(overrides.Surface)
	}
	if overrides.Base != "" {
		Colors.Base = lipgloss.Color(overrides.Base)
	}
	if overrides.Text != "" {
		Colors.Text = lipgloss.Color(overrides.Text)
	}
	if overrides.TextDim != "" {
		Colors.TextDim = lipgloss.Color(overrides.TextDim)
	}
	recalcStyles()
}

func recalcStyles() {
	w := TermWidth

	HeaderStyle = lipgloss.NewStyle().
		Background(Colors.Primary).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 2).
		Bold(true)

	FooterStyle = lipgloss.NewStyle().
		Background(Colors.Surface).
		Foreground(Colors.TextDim).
		Padding(0, 1)

	InputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Primary).
		Padding(0, 1).
		Width(w - 4)

	StatusStyle = lipgloss.NewStyle().
		Foreground(Colors.TextDim).
		Italic(true)

	UserMsgStyle = lipgloss.NewStyle().
		Foreground(Colors.Secondary).
		Bold(true)

	AssistantMsgStyle = lipgloss.NewStyle().
		Foreground(Colors.Text)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(Colors.Error).
		Bold(true)

	HelpStyle = lipgloss.NewStyle().
		Foreground(Colors.TextDim).
		Italic(true).
		Padding(0, 2)

	DividerStyle = lipgloss.NewStyle().
		Foreground(Colors.Muted).
		Width(w).
		Padding(0, 2)

	CodeStyle = lipgloss.NewStyle().
		Background(Colors.CodeBg).
		Foreground(Colors.Text).
		Padding(0, 1)

	TableStyle = lipgloss.NewStyle().
		Foreground(Colors.Text)

	// Heading styles — terminal has NO font size control
	// Only color + bold/italic/underline available
	Heading1Style = lipgloss.NewStyle().
		Foreground(Colors.Primary).
		Bold(true).
		Underline(true).
		Padding(0, 0)

	Heading2Style = lipgloss.NewStyle().
		Foreground(Colors.Secondary).
		Bold(true)

	Heading3Style = lipgloss.NewStyle().
		Bold(true)

	Heading4Style = lipgloss.NewStyle().
		Italic(true).
		Foreground(Colors.TextDim)

	Heading5Style = lipgloss.NewStyle().
		Italic(true)

	Heading6Style = lipgloss.NewStyle().
		Foreground(Colors.TextDim)
}
