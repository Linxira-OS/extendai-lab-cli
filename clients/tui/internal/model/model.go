package model

import (
	"fmt"
	"strings"

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

type MsgStreamChunk struct {
	Content string
	Done    bool
}

type MsgError struct {
	Err string
}

// ─── Model ──────────────────────────────────────────────────

type Model struct {
	// Chat state
	messages   []Message
	input      string
	streamBuf  strings.Builder

	// Server connection
	serverConnected bool
	serverError     string

	// Viewport for scrolling message history
	viewport viewport.Model

	// Mode
	mode Mode

	// Ready flag (after first render)
	ready bool
}

type Message struct {
	Role    string // "user" | "assistant" | "system" | "error"
	Content string
}

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
)

func New() Model {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

	return Model{
		messages: []Message{
			{
				Role:    "system",
				Content: "Welcome to ExtendAI Lab. Type /help for commands.",
			},
		},
		viewport: vp,
		mode:     ModeNormal,
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
		if !msg.Connected {
			m.messages = append(m.messages, Message{
				Role:    "error",
				Content: "Server not available: " + msg.Error,
			})
		} else {
			m.messages = append(m.messages, Message{
				Role:    "system",
				Content: "Connected to server.",
			})
		}
		m.viewport.GotoBottom()
		return m, nil

	case MsgStreamChunk:
		m.streamBuf.WriteString(msg.Content)
		if msg.Done {
			m.messages = append(m.messages, Message{
				Role:    "assistant",
				Content: m.streamBuf.String(),
			})
			m.streamBuf.Reset()
			m.viewport.GotoBottom()
		}
		return m, nil

	case MsgError:
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: msg.Err,
		})
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
		// Add user message
		m.messages = append(m.messages, Message{
			Role:    "user",
			Content: input,
		})
		m.input = ""
		m.mode = ModeNormal
		m.viewport.GotoBottom()
		// TODO: send to server via IPC
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
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: m.execCommand(cmd),
		})
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
		m.messages = []Message{}
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

	var b strings.Builder

	// Header bar
	statusIcon := "●"
	statusColor := theme.Error
	if m.serverConnected {
		statusIcon = "●"
		statusColor = theme.Success
	}
	headerText := lipgloss.NewStyle().Foreground(statusColor).Render(statusIcon) + " ExtendAI Lab"
	b.WriteString(theme.HeaderStyle.Width(theme.TermWidth).Render(headerText))
	b.WriteString("\n")

	// Mode indicator
	modeText := "NORMAL"
	modeColor := theme.Primary
	switch m.mode {
	case ModeInsert:
		modeText = "INSERT"
		modeColor = theme.Secondary
	case ModeCommand:
		modeText = "CMD"
		modeColor = theme.Accent
	}
	modeBadge := lipgloss.NewStyle().
		Background(modeColor).
		Foreground(lipgloss.Color("#000000")).
		Padding(0, 1).
		Render(modeText)
	b.WriteString(theme.DividerStyle.Render(modeBadge))
	b.WriteString("\n")

	// Chat content in viewport
	var content strings.Builder
	for i, msg := range m.messages {
		if i > 0 {
			content.WriteString("\n")
		}
		switch msg.Role {
		case "user":
			content.WriteString(theme.UserMsgStyle.Render("┌ You:"))
			content.WriteString("\n")
			content.WriteString(theme.ChatStyle.Render(msg.Content))
			content.WriteString("\n")
			content.WriteString(theme.UserMsgStyle.Render("└"))
		case "assistant":
			content.WriteString(theme.AssistantMsgStyle.Render("┌ Assistant:"))
			content.WriteString("\n")
			content.WriteString(theme.ChatStyle.Render(msg.Content))
			content.WriteString("\n")
			content.WriteString(theme.AssistantMsgStyle.Render("└"))
		case "error":
			content.WriteString(theme.ErrorStyle.Render("✗ " + msg.Content))
		case "system":
			content.WriteString(theme.StatusStyle.Render(msg.Content))
		}
	}
	m.viewport.SetContent(content.String())
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Input bar
	inputPlaceholder := "Press 'i' to type, ':' for commands, ESC for normal"
	inputContent := m.input
	if m.mode == ModeNormal && inputContent == "" {
		inputContent = inputPlaceholder
	}
	inputStyle := theme.InputStyle.
		Width(theme.TermWidth - 4)
	if m.mode == ModeInsert {
		inputStyle = inputStyle.BorderForeground(theme.Secondary)
	} else if m.mode == ModeCommand {
		inputStyle = inputStyle.BorderForeground(theme.Accent)
	}
	prompt := "> "
	if m.mode == ModeCommand {
		prompt = ":"
	}
	b.WriteString(inputStyle.Render(prompt + inputContent))
	b.WriteString("\n")

	// Footer
	modelInfo := "free:QwQ-32B"
	msgCount := len(m.messages)
	tokens := "-"
	footerText := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Width(theme.TermWidth/3).Align(lipgloss.Left).Render(modelInfo),
		lipgloss.NewStyle().Width(theme.TermWidth/3).Align(lipgloss.Center).Render(fmt.Sprintf("%d msgs", msgCount)),
		lipgloss.NewStyle().Width(theme.TermWidth/3).Align(lipgloss.Right).Render(tokens),
	)
	b.WriteString(theme.FooterStyle.Width(theme.TermWidth).Render(footerText))

	return b.String()
}
