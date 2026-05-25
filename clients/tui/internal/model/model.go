package model

import (
	"fmt"
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/ipc"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/renderer"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Messages ───────────────────────────────────────────────

type MsgServerStatus struct {
	Connected bool
	Error     string
}

type MsgRenderCommand struct {
	Command *protocol.RenderCommand
}

type MsgError struct {
	Err string
}

// ─── Model ──────────────────────────────────────────────────

type Model struct {
	// Render state from TS
	renderCmd *protocol.RenderCommand

	// Local input state (for low-latency typing)
	input string
	mode  Mode

	// Server connection
	serverConnected bool
	serverError     string

	// Viewport for scrolling
	viewport viewport.Model

	// IPC client
	client *ipc.Client

	// Ready flag
	ready bool
}

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
)

// ─── Constructor ────────────────────────────────────────────

func New(client *ipc.Client) Model {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

	return Model{
		mode:     ModeNormal,
		viewport: vp,
		client:   client,
	}
}

// ─── Init ────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return nil
}

// ─── Update ──────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		headerHeight := 1
		footerHeight := 1
		inputHeight := 3
		vpHeight := msg.Height - headerHeight - footerHeight - inputHeight - 2
		if vpHeight < 10 {
			vpHeight = 10
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = vpHeight
		theme.TermWidth = msg.Width
		theme.TermHeight = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case ModeNormal:
			return m.handleNormalMode(msg)
		case ModeInsert:
			return m.handleInsertMode(msg)
		case ModeCommand:
			return m.handleCommandMode(msg)
		}

	case MsgServerStatus:
		m.serverConnected = msg.Connected
		m.serverError = msg.Error
		// Apply theme overrides if present in render command
		if m.renderCmd != nil && m.renderCmd.Theme != nil && m.renderCmd.Theme.Colors != nil {
			theme.ApplyThemeOverrides(m.renderCmd.Theme.Colors)
		}
		m.viewport.GotoBottom()
		return m, nil

	case MsgRenderCommand:
		m.renderCmd = msg.Command
		// Apply theme overrides
		if msg.Command.Theme != nil && msg.Command.Theme.Colors != nil {
			theme.ApplyThemeOverrides(msg.Command.Theme.Colors)
		}
		// Auto-scroll to bottom for new content
		m.viewport.GotoBottom()
		return m, nil

	case MsgError:
		m.renderCmd = &protocol.RenderCommand{
			Components: []protocol.Component{
				{
					Type: "message",
					Props: map[string]interface{}{
						"role":    "error",
						"content": msg.Err,
					},
				},
			},
		}
		m.viewport.GotoBottom()
		return m, nil
	}

	return m, nil
}

// ─── Mode Handlers ──────────────────────────────────────────

func (m Model) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "i", "I":
		m.mode = ModeInsert
	case ":":
		m.mode = ModeCommand
		m.input = ""
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.viewport.LineDown(1)
	case "k", "up":
		m.viewport.LineUp(1)
	case "g":
		m.viewport.GotoTop()
	case "G":
		m.viewport.GotoBottom()
	case "ctrl+d":
		m.viewport.HalfViewDown()
	case "ctrl+u":
		m.viewport.HalfViewUp()
	}
	return m, nil
}

func (m Model) handleInsertMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.mode = ModeNormal
	case tea.KeyEnter:
		input := strings.TrimSpace(m.input)
		if input == "" {
			return m, nil
		}
		// Send to TS via IPC
		if m.client != nil {
			m.client.SendMessage(input)
		}
		m.input = ""
		m.mode = ModeNormal
		m.viewport.GotoBottom()
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	case tea.KeyRunes:
		m.input += string(msg.Runes)
	}
	return m, nil
}

func (m Model) handleCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.mode = ModeNormal
		m.input = ""
	case tea.KeyEnter:
		cmd := strings.TrimSpace(strings.TrimPrefix(m.input, "/"))
		result := m.execCommand(cmd)
		// Show command result as system message via render command
		m.renderCmd = &protocol.RenderCommand{
			Components: []protocol.Component{
				{
					Type: "message",
					Props: map[string]interface{}{
						"role":    "system",
						"content": result,
					},
				},
			},
		}
		m.input = ""
		m.mode = ModeNormal
		m.viewport.GotoBottom()
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	case tea.KeyRunes:
		m.input += string(msg.Runes)
	}
	return m, nil
}

// ─── Commands ────────────────────────────────────────────────

func (m Model) execCommand(cmd string) string {
	args := strings.Fields(cmd)
	if len(args) == 0 {
		return "Available commands: help, clear, model, exit"
	}
	switch args[0] {
	case "help":
		return `Commands:
  /help       Show this help
  /clear      Clear conversation
  /model      Show/set model
  /exit       Quit (or :q)

Modes:
  i           Insert mode (type your message)
  Esc         Normal mode
  j/k         Scroll up/down
  g/G         Top/bottom
  :           Command mode`
	case "clear":
		m.renderCmd = nil
		return "Conversation cleared."
	case "model":
		if len(args) > 1 {
			return "Model set to: " + args[1]
		}
		return "Current model: free:QwQ-32B"
	case "exit", "quit":
		return "Use :q or Ctrl+C to quit."
	default:
		return "Unknown command: " + args[0] + ". Type /help"
	}
}

// ─── View ────────────────────────────────────────────────────

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	width := theme.TermWidth
	if width < 40 {
		width = 40
	}

	// Start with the RenderCommand rendering as the base
	// Then inject local interactive elements (input bar, mode indicator, status bar)

	var b strings.Builder

	// ── Header ──
	statusIcon := lipgloss.NewStyle().Foreground(theme.Colors.Success).Render("●")
	if !m.serverConnected {
		statusIcon = lipgloss.NewStyle().Foreground(theme.Colors.Error).Render("●")
	}
	headerText := fmt.Sprintf("%s ExtendAI Lab", statusIcon)
	b.WriteString(theme.HeaderStyle.Width(width).Render(headerText))
	b.WriteString("\n")

	// ── Mode badge ──
	modeText := "NORMAL"
	modeColor := theme.Colors.Primary
	switch m.mode {
	case ModeInsert:
		modeText = "INSERT"
		modeColor = theme.Colors.Secondary
	case ModeCommand:
		modeText = "CMD"
		modeColor = theme.Colors.Accent
	}
	modeBadge := lipgloss.NewStyle().
		Background(modeColor).
		Foreground(theme.Colors.Base).
		Padding(0, 1).
		Render(modeText)
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.Muted).
		Render(strings.Repeat("─", 2)))
	b.WriteString(modeBadge)
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.Muted).
		Render(strings.Repeat("─", width-2-len(modeText)-2)))
	b.WriteString("\n")

	// ── Content area (render TS components) ──
	if m.renderCmd != nil && len(m.renderCmd.Components) > 0 {
		rendered := renderer.RenderAllComponents(m.renderCmd, m.viewport.Width)
		rendered = renderer.RenderMarkdown(rendered, m.viewport.Width-2)
		m.viewport.SetContent(rendered)
	} else {
		m.viewport.SetContent(theme.StatusStyle.Render("Welcome to ExtendAI Lab. Type /help for commands."))
	}
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// ── Input bar ──
	inputPlaceholder := "Press 'i' to type, ':' for commands, ESC for normal"
	inputContent := m.input
	if m.mode == ModeNormal && inputContent == "" {
		inputContent = inputPlaceholder
	}
	inputBox := theme.InputStyle.Width(width - 4)
	if m.mode == ModeInsert {
		inputBox = inputBox.BorderForeground(theme.Colors.Secondary)
	} else if m.mode == ModeCommand {
		inputBox = inputBox.BorderForeground(theme.Colors.Accent)
	}
	prompt := "> "
	if m.mode == ModeCommand {
		prompt = ":"
	}
	b.WriteString(inputBox.Render(prompt + inputContent))
	b.WriteString("\n")

	// ── Footer status bar ──
	modelInfo := "free:QwQ-32B"
	msgCount := "? msgs"
	statusRight := "connected"
	if !m.serverConnected {
		statusRight = "disconnected"
	}
	footerText := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Width(width/3).Align(lipgloss.Left).Render(modelInfo),
		lipgloss.NewStyle().Width(width/3).Align(lipgloss.Center).Render(msgCount),
		lipgloss.NewStyle().Width(width/3).Align(lipgloss.Right).Render(statusRight),
	)
	b.WriteString(theme.FooterStyle.Width(width).Render(footerText))

	return b.String()
}
