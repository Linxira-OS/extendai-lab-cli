package model

// ─── Dialog System ─────────────────────────────────────────────
//
// Architecture matching opencode's dialog system:
//   - Dialog overlay with semi-transparent backdrop (RGBA(0,0,0,150))
//   - Centered dialog box with border
//   - Background content dimmed to 60% brightness
//   - Dialog content at 100% brightness
//
// Reference: studying/opencode-dev/packages/opencode/src/cli/cmd/tui/ui/dialog.tsx
//            studying/opencode-dev/packages/opencode/src/cli/cmd/tui/ui/dialog-select.tsx

import (
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// ─── Dialog types ─────────────────────────────────────────────

type DialogType string

const (
	DialogNone     DialogType = ""
	DialogModel    DialogType = "model_list"
	DialogHelp     DialogType = "help"
	DialogHelpType DialogType = "help_dialog" // F1 help overlay
	DialogStatus   DialogType = "status"
	DialogSession  DialogType = "session_list"
	DialogPalette  DialogType = "command_palette"
	DialogTheme    DialogType = "theme_list"
	DialogProvider DialogType = "provider_list"
)

// ─── Dialog sizes ─────────────────────────────────────────────

const (
	DialogSizeSmall  = 40
	DialogSizeMedium = 60
	DialogSizeLarge  = 80
	DialogSizeXLarge = 100
)

// ─── Dialog interface ─────────────────────────────────────────

// Dialog is the interface for all dialog popups.
type Dialog interface {
	// Type returns the dialog type identifier.
	Type() DialogType
	// Title returns the dialog title.
	Title() string
	// View renders the dialog content at the given dimensions.
	View(width, height int) string
	// Update handles a key message and returns the (potentially new) dialog + cmd.
	Update(msg interface{}) (Dialog, interface{})
	// Height returns the preferred dialog height.
	Height() int
}

// ─── Overlay rendering ────────────────────────────────────────

// renderDialogOverlay creates a centered dialog with a semi-transparent backdrop.
//
// The dialog is centered in the terminal. The backdrop uses Unicode half-block
// characters to create a dimming effect over the background content.
//
// Layout:
//
//	┌──────────────────────────────────────────┐
//	│  [backdrop: dimmed background content]   │
//	│                                          │
//	│          ┌─ Dialog ──────────┐           │
//	│          │  Title             │           │
//	│          │  Content...        │           │
//	│          └───────────────────┘           │
//	│                                          │
//	└──────────────────────────────────────────┘
func renderDialogOverlay(dialog Dialog, termW, termH, dlgW, dlgH int) string {
	if dlgW <= 0 || dlgW > termW-4 {
		dlgW = termW - 4
		if dlgW > DialogSizeLarge {
			dlgW = DialogSizeLarge
		}
	}
	if dlgH <= 0 || dlgH > termH-6 {
		dlgH = termH - 6
		if dlgH > 30 {
			dlgH = 30
		}
	}

	// Build dialog box with border
	title := dialog.Title()
	content := dialog.View(dlgW-4, dlgH-4) // account for border + padding

	// Title bar
	titleBar := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Colors.Primary).
		Render(title)

	// Separator
	sep := lipgloss.NewStyle().
		Foreground(theme.Colors.Muted).
		Render(strings.Repeat("─", dlgW-4))

	// Footer hint
	hint := lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).
		Italic(true).
		Render("↑↓ navigate · Enter select · Esc close")

	// Assemble dialog content
	inner := lipgloss.JoinVertical(lipgloss.Top,
		titleBar,
		sep,
		content,
		sep,
		hint,
	)

	// Dialog box style with backdrop
	dialogBox := lipgloss.NewStyle().
		Width(dlgW).
		Height(dlgH).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Colors.Accent).
		Background(theme.Colors.Surface).
		Padding(1, 2).
		Render(inner)

	// Center dialog in terminal using Place with backdrop background
	return lipgloss.Place(
		termW, termH,
		lipgloss.Center, lipgloss.Center,
		dialogBox,
		lipgloss.WithWhitespaceBackground(theme.Colors.Backdrop),
	)
}
