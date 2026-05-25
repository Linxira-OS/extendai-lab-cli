package theme

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Global state
var (
	TermWidth  int
	TermHeight int

	// Colors
	Primary   = lipgloss.Color("#7C3AED") // violet
	Secondary = lipgloss.Color("#06B6D4") // cyan
	Accent    = lipgloss.Color("#F59E0B") // amber
	Success   = lipgloss.Color("#10B981") // emerald
	Error     = lipgloss.Color("#EF4444") // red
	Muted     = lipgloss.Color("#6B7280") // gray
	Surface   = lipgloss.Color("#1F2937") // dark gray
	Base      = lipgloss.Color("#111827") // near black
	Text      = lipgloss.Color("#F3F4F6") // light gray
	TextDim   = lipgloss.Color("#9CA3AF") // dim gray

	// Styles
	HeaderStyle   lipgloss.Style
	FooterStyle   lipgloss.Style
	ChatStyle     lipgloss.Style
	InputStyle    lipgloss.Style
	StatusStyle   lipgloss.Style
	UserMsgStyle  lipgloss.Style
	AssistantMsgStyle lipgloss.Style
	ErrorStyle    lipgloss.Style
	HelpStyle     lipgloss.Style
	DividerStyle  lipgloss.Style
)

func InitTerm() {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, height = 120, 40
	}
	TermWidth = width
	TermHeight = height

	HeaderStyle = lipgloss.NewStyle().
		Background(Primary).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 2).
		Bold(true)

	FooterStyle = lipgloss.NewStyle().
		Background(Surface).
		Foreground(TextDim).
		Padding(0, 1)

	ChatStyle = lipgloss.NewStyle().
		Padding(0, 2)

	InputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(0, 1).
		Width(width - 4)

	StatusStyle = lipgloss.NewStyle().
		Foreground(TextDim).
		Italic(true)

	UserMsgStyle = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true)

	AssistantMsgStyle = lipgloss.NewStyle().
		Foreground(Text)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(Error).
		Bold(true)

	HelpStyle = lipgloss.NewStyle().
		Foreground(TextDim).
		Italic(true).
		Padding(0, 2)

	DividerStyle = lipgloss.NewStyle().
		Foreground(Muted).
		Width(width).
		Padding(0, 2)
}
