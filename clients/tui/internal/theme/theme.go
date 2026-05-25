package theme

import (
	"os"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// ─── Terminal Capabilities ────────────────────────────────────
//
// Terminals CAN render:
//   ✓ Bold / Italic / Underline / Strikethrough
//   ✓ 256 colors / True color / Background
//   ✓ Inverse / Reverse
//
// Terminals CANNOT render:
//   ✗ Font size changes (H1-H6 same physical size)
//   ✗ Different fonts (monospace only)
//   ✗ Images / Line height / Letter spacing

// ─── ThemeColors ─────────────────────────────────────────────

type ThemeColors struct {
	// Brand
	Primary   lipgloss.Color // main brand (blueWHALE blue)
	Secondary lipgloss.Color // complementary brand (cyan)
	Accent    lipgloss.Color // accent highlights (amber/gold)

	// Semantic
	Success lipgloss.Color // green
	Error   lipgloss.Color // red
	Warning lipgloss.Color // orange
	Muted   lipgloss.Color // border / subtle

	// Surfaces
	Surface   lipgloss.Color // panel / card background
	Base      lipgloss.Color // root background
	Backdrop  lipgloss.Color // dialog overlay / dimming
	CodeBg    lipgloss.Color // code block background
	Selection lipgloss.Color // selection highlight

	// Text hierarchy (inspired by CodeWhale)
	Text     lipgloss.Color // body text (brightest)
	TextSoft lipgloss.Color // slightly dimmer for secondary content
	TextDim  lipgloss.Color // hints / meta / timestamps
	TextUser lipgloss.Color // user messages (green accent)
}

// ─── Theme presets ───────────────────────────────────────────

var Themes = map[string]struct {
	Name string
	Fn   func() ThemeColors
}{
	"bluewhale": {"Blue Whale", BlueWhaleColors},
	"whale":     {"Whale", WhaleColors},
	"catppuccin": {"Catppuccin Mocha", CatppuccinColors},
}

func BlueWhaleColors() ThemeColors {
	return ThemeColors{
		Primary:   lipgloss.Color("#3578E5"), // blueWHALE blue
		Secondary: lipgloss.Color("#06B6D4"), // cyan
		Accent:    lipgloss.Color("#F59E0B"), // gold
		Success:   lipgloss.Color("#10B981"), // emerald
		Error:     lipgloss.Color("#EF4444"), // red
		Warning:   lipgloss.Color("#F97316"), // orange
		Muted:     lipgloss.Color("#475569"), // slate-600

		Surface:   lipgloss.Color("#1E293B"), // slate-800
		Base:      lipgloss.Color("#0F172A"), // slate-900
		Backdrop:  lipgloss.Color("#0a0e1a"), // darker slate for dimming
		CodeBg:    lipgloss.Color("#1E293B"), // same as surface
		Selection: lipgloss.Color("#334155"), // slate-700

		Text:     lipgloss.Color("#E2E8F0"), // slate-200
		TextSoft: lipgloss.Color("#94A3B8"), // slate-400
		TextDim:  lipgloss.Color("#64748B"), // slate-500
		TextUser: lipgloss.Color("#34D399"), // emerald-400
	}
}

func WhaleColors() ThemeColors {
	return ThemeColors{
		Primary:   lipgloss.Color("#1D4ED8"), // ocean blue
		Secondary: lipgloss.Color("#06B6D4"), // cyan
		Accent:    lipgloss.Color("#F59E0B"), // amber
		Success:   lipgloss.Color("#10B981"), // emerald
		Error:     lipgloss.Color("#EF4444"), // red
		Warning:   lipgloss.Color("#F97316"), // orange
		Muted:     lipgloss.Color("#6B7280"), // gray

		Surface:   lipgloss.Color("#1F2937"),
		Base:      lipgloss.Color("#111827"),
		Backdrop:  lipgloss.Color("#080c14"),
		CodeBg:    lipgloss.Color("#2D3748"),
		Selection: lipgloss.Color("#374151"),

		Text:     lipgloss.Color("#F3F4F6"),
		TextSoft: lipgloss.Color("#D1D5DB"),
		TextDim:  lipgloss.Color("#9CA3AF"),
		TextUser: lipgloss.Color("#34D399"),
	}
}

func CatppuccinColors() ThemeColors {
	return ThemeColors{
		Primary:   lipgloss.Color("#89B4FA"), // blue
		Secondary: lipgloss.Color("#94E2D5"), // teal
		Accent:    lipgloss.Color("#F9E2AF"), // yellow
		Success:   lipgloss.Color("#A6E3A1"), // green
		Error:     lipgloss.Color("#F38BA8"), // red
		Warning:   lipgloss.Color("#FAB387"), // peach
		Muted:     lipgloss.Color("#585B70"), // surface2

		Surface:   lipgloss.Color("#313244"), // surface0
		Base:      lipgloss.Color("#1E1E2E"), // base
		Backdrop:  lipgloss.Color("#0c0c18"),
		CodeBg:    lipgloss.Color("#181825"), // mantle
		Selection: lipgloss.Color("#45475A"), // surface1

		Text:     lipgloss.Color("#CDD6F4"), // text
		TextSoft: lipgloss.Color("#BAC2DE"), // subtext1
		TextDim:  lipgloss.Color("#A6ADC8"), // subtext0
		TextUser: lipgloss.Color("#A6E3A1"), // green
	}
}

// ─── Current theme ───────────────────────────────────────────

var (
	CurrentTheme string = "bluewhale"
	Colors       ThemeColors

	TermWidth  int
	TermHeight int

	// ── Layout styles ──
	HeaderStyle   lipgloss.Style
	FooterStyle   lipgloss.Style
	InputStyle    lipgloss.Style
	PromptStyle   lipgloss.Style
	DividerStyle  lipgloss.Style

	// ── Message styles ──
	UserStyle       lipgloss.Style
	AssistantStyle  lipgloss.Style
	SystemStyle     lipgloss.Style
	ErrorStyle      lipgloss.Style
	ToolOutputStyle lipgloss.Style
	CodeStyle       lipgloss.Style

	// ── Heading styles ──
	Heading1Style lipgloss.Style
	Heading2Style lipgloss.Style
	Heading3Style lipgloss.Style
	Heading4Style lipgloss.Style
	Heading5Style lipgloss.Style
	Heading6Style lipgloss.Style

	// ── Table ──
	TableStyle lipgloss.Style

	// ── Panel / Tab ──
	PanelStyle    lipgloss.Style
	TabBarStyle   lipgloss.Style
	TabActiveStyle lipgloss.Style
	TabInactiveStyle lipgloss.Style
	ScrollbarTrackStyle lipgloss.Style
	ScrollbarThumbStyle lipgloss.Style
)

// ─── Theme names for /theme command ──────────────────────────

var ThemeNames = []string{"bluewhale", "whale", "catppuccin"}

func ThemeDisplayName(id string) string {
	t, ok := Themes[id]
	if !ok {
		return id
	}
	return t.Name
}

// ─── Init ────────────────────────────────────────────────────

func InitTerm() {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, height = 120, 40
	}
	TermWidth = width
	TermHeight = height
	SetTheme("bluewhale")
}

func SetTheme(id string) bool {
	t, ok := Themes[id]
	if !ok {
		return false
	}
	CurrentTheme = id
	Colors = t.Fn()
	recalcStyles()
	return true
}

// ApplyThemeOverrides applies theme overrides from TS RenderCommand.
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

	// ── Layout ──
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
		BorderForeground(Colors.Muted).
		Padding(0, 1).
		Width(w - 4)

	PromptStyle = lipgloss.NewStyle().
		Foreground(Colors.Primary).
		Bold(true)

	DividerStyle = lipgloss.NewStyle().
		Foreground(Colors.Muted).
		Width(w).
		Padding(0, 2)

	// ── Messages ──
	UserStyle = lipgloss.NewStyle().
		Foreground(Colors.TextUser).
		Bold(true)

	AssistantStyle = lipgloss.NewStyle().
		Foreground(Colors.Text)

	SystemStyle = lipgloss.NewStyle().
		Foreground(Colors.TextDim).
		Italic(true)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(Colors.Error).
		Bold(true)

	ToolOutputStyle = lipgloss.NewStyle().
		Background(Colors.Surface).
		Padding(0, 1).
		Width(w - 2)

	CodeStyle = lipgloss.NewStyle().
		Background(Colors.CodeBg).
		Foreground(Colors.TextSoft).
		Padding(0, 1)

	TableStyle = lipgloss.NewStyle().
		Foreground(Colors.Text)

	// ── Panel / Tab ──
	PanelStyle = lipgloss.NewStyle().
		Background(Colors.Surface).
		Padding(0, 1)

	TabBarStyle = lipgloss.NewStyle().
		Background(Colors.Surface).
		Foreground(Colors.Muted)

	TabActiveStyle = lipgloss.NewStyle().
		Background(Colors.Primary).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		Bold(true)

	TabInactiveStyle = lipgloss.NewStyle().
		Background(Colors.Surface).
		Foreground(Colors.TextDim).
		Padding(0, 1)

	ScrollbarTrackStyle = lipgloss.NewStyle().
		Foreground(Colors.Muted)

	ScrollbarThumbStyle = lipgloss.NewStyle().
		Foreground(Colors.TextDim).
		Bold(true)

	// ── Headings ──
	Heading1Style = lipgloss.NewStyle().
		Foreground(Colors.Primary).
		Bold(true).
		Underline(true)

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
