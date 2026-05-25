package model

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/api"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/ipc"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/renderer"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	tuiView "github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/view"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Focus Zone ──────────────────────────────────────────────

type FocusZone int

const (
	FocusMain  FocusZone = iota // browsing / scrolling main content
	FocusPanel                  // browsing / scrolling right panel
	FocusInput                  // editing input field
)

// ─── AI State ────────────────────────────────────────────────

type AIState struct {
	Status   protocol.AIStatus // idle | thinking | streaming | error
	Label    string            // human-readable label
	Model    string            // model name
	Content  string            // accumulated streaming content
	ErrorMsg string            // last error message
}

func (s AIState) IsWorking() bool {
	return s.Status == protocol.AIThinking || s.Status == protocol.AIStreaming
}

func (s AIState) SpinnerFrame() string {
	if !s.IsWorking() {
		return ""
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[time.Now().UnixMilli()/80%int64(len(frames))]
}

// ─── Messages ───────────────────────────────────────────────

type MsgServerStatus struct {
	Connected bool
	Error     string
}
type MsgRenderCommand struct {
	Command *protocol.RenderCommand
}
type MsgAIStatus struct {
	Status protocol.AIStatus
	Model  string
	Label  string
}
type MsgStreamChunk struct {
	Content string
	Done    bool
	Error   string
}
type MsgStatusUpdate struct {
	Update protocol.StatusUpdate
}
type MsgError struct {
	Err string
}

// ─── Model ──────────────────────────────────────────────────

type Model struct {
	// Render state from TS
	renderCmd  *protocol.RenderCommand
	cachedMain string // cached rendered main content

	// Input state
	input        string
	prompt       string   // ">" or "/"
	inputFocused bool

	// Focus navigation
	focusZone FocusZone

	// AI state machine
	ai AIState

	// Environment status (from TS or local)
	serverConnected bool
	serverError     string
	lspCount        int
	mcpCount        int
	mcpError        bool
	currentDir      string
	modelName       string
	permissionCount int
	lspList         []protocol.LSPStatus
	mcpList         []protocol.MCPStatus

	// Main viewport
	viewport viewport.Model

	// Session tree (persistent conversation storage)
	session *Session

	// Request timing tracking (for assistant message metadata)
	reqStart      time.Time
	reqFirstToken time.Time
	reqTokensOut  int

	// Right panel
	panelMode          string // "auto" (hide when narrow) or "hide" (always hidden)
	activeRightTab     string
	rightTabIDs        []string
	rightPanelScroll   int
	rightPanelHeight   int
	rightPanelContent  string
	panelRenderCmd     *protocol.Component // the side-panel component from TS
	panelSections      map[string]bool    // section name -> expanded (mcp/lsp/todo/files)

	// IPC client (TS server)
	client *ipc.Client

	// Standalone API client (direct OpenAI-compatible, e.g. LM Studio)
	apiClient *api.Client

	// Streaming channel for standalone API client
	streamCh <-chan api.StreamEvent

	// Model info for header/status display
	modelURL string // API base URL (truncated for display)

	// Dialog system (overlay popups)
	activeDialog Dialog
	dialogOpen   bool

	// Ready flag
	ready bool
}

func New(client *ipc.Client, aiClient *api.Client) Model {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

	m := Model{
		prompt:   ">",
		ready:    true,
		viewport: vp,
		client:   client,
		apiClient: aiClient,
		rightTabIDs: []string{"lsp", "files", "todo"},
		activeRightTab: "lsp",
		panelSections: map[string]bool{
			"mcp":   true,  // expanded by default
			"lsp":   true,
			"todo":  false,
			"files": false,
		},
	}

	// In standalone mode, set the model name from the API client
	if aiClient != nil {
		m.serverConnected = true
		m.modelName = aiClient.Model()
		m.modelURL = aiClient.BaseURL()
	}

	// Initialize session (load most recent or create new)
	dir := m.currentDir
	if dir == "" {
		dir, _ = os.Getwd()
		if dir == "" {
			dir = "~"
		}
	}
	m.session = loadOrCreateSession(dir)

	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

// ─── Update ──────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// ── When a dialog is active, route all messages to the dialog ──
	if m.dialogOpen && m.activeDialog != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			newDlg, cmd := m.activeDialog.Update(msg)
			m.activeDialog = newDlg
			if dlg, ok := cmd.(tea.Cmd); ok && dlg != nil {
				return m, dlg
			}
			// Close dialog on Escape or Ctrl+C
			if msg.Type == tea.KeyEscape || msg.Type == tea.KeyCtrlC {
				m.dialogOpen = false
				m.activeDialog = nil
				m.invalidateCache()
				return m, nil
			}
			// Re-query dialog state: if it returned a close signal via its own logic
			m.invalidateCache()
			return m, nil
		case tea.WindowSizeMsg:
			// Pass resize through so dialogs can react
			// but don't break the overlay
			m.handleResize(msg)
			return m, nil
		default:
			return m, nil
		}
	}

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		return m.handleResize(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case MsgServerStatus:
		m.serverConnected = msg.Connected
		m.serverError = msg.Error
		if msg.Connected && m.renderCmd != nil && m.renderCmd.Theme != nil && m.renderCmd.Theme.Colors != nil {
			theme.ApplyThemeOverrides(m.renderCmd.Theme.Colors)
		}
		m.invalidateCache()
		m.viewport.GotoBottom()
		return m, nil

	case MsgRenderCommand:
		m.renderCmd = msg.Command
		if msg.Command.Theme != nil && msg.Command.Theme.Colors != nil {
			theme.ApplyThemeOverrides(msg.Command.Theme.Colors)
		}
		// Extract side-panel component from render command
		m.extractPanelData()
		// If AI was streaming and we got a full render, mark as idle
		if m.ai.Status == protocol.AIStreaming {
			m.ai.Status = protocol.AIIdle
			// Accumulate last streaming content into session
			if m.ai.Content != "" {
				msg := NewAssistantMessage(m.ai.Content, 0, 0, 0, len(m.ai.Content)/4, 0)
				if m.session != nil {
					m.session.AppendMessage(msg)
				}
				m.ai.Content = ""
			}
		}
		m.invalidateCache()
		m.viewport.GotoBottom()
		return m, nil

	case MsgAIStatus:
		m.ai.Status = msg.Status
		m.ai.Label = msg.Label
		if msg.Model != "" {
			m.ai.Model = msg.Model
		}
		if msg.Status == protocol.AIIdle && m.ai.Content != "" {
			// Compute timing for assistant message
			dur := time.Duration(0)
			ttft := time.Duration(0)
			if !m.reqStart.IsZero() {
				dur = time.Since(m.reqStart)
			}
			if !m.reqFirstToken.IsZero() {
				ttft = m.reqFirstToken.Sub(m.reqStart)
			}
			asm := NewAssistantMessage(m.ai.Content, dur, ttft, 0, m.reqTokensOut, 0)
			if m.session != nil {
				m.session.AppendMessage(asm)
			}
			m.ai.Content = ""
			m.reqStart = time.Time{}
			m.reqFirstToken = time.Time{}
			m.reqTokensOut = 0
		}
		if msg.Status == protocol.AIError {
			m.ai.ErrorMsg = msg.Label
		}
		m.invalidateCache()
		return m, nil

	case MsgStreamChunk:
		if msg.Error != "" {
			m.ai.Status = protocol.AIError
			m.ai.ErrorMsg = msg.Error
			if m.session != nil {
				m.session.AppendMessage(NewErrorMessage(msg.Error))
			}
			m.invalidateCache()
			m.streamCh = nil
			return m, nil
		}

		// Track first token timing
		if m.ai.Status != protocol.AIStreaming && m.reqFirstToken.IsZero() {
			m.reqFirstToken = time.Now()
		}

		m.ai.Status = protocol.AIStreaming
		m.ai.Content += msg.Content
		m.reqTokensOut += len(msg.Content) / 4 // rough token estimate

		if msg.Done {
			m.ai.Status = protocol.AIIdle
			if m.ai.Content != "" {
				// Compute timing
				dur := time.Duration(0)
				ttft := time.Duration(0)
				if !m.reqStart.IsZero() {
					dur = time.Since(m.reqStart)
				}
				if !m.reqFirstToken.IsZero() {
					ttft = m.reqFirstToken.Sub(m.reqStart)
				}
				asm := NewAssistantMessage(m.ai.Content, dur, ttft, 0, m.reqTokensOut, 0)
				if m.session != nil {
					m.session.AppendMessage(asm)
					m.session.Save()
				}
			}
			m.ai.Content = ""
			m.reqStart = time.Time{}
			m.reqFirstToken = time.Time{}
			m.reqTokensOut = 0
			m.invalidateCache()
			m.streamCh = nil
			return m, nil
		}
		m.invalidateCache()
		m.viewport.GotoBottom()
		// Continue streaming if we have an active channel
		if m.streamCh != nil {
			return m, m.apiNextChunk()
		}
		return m, nil

	case MsgStatusUpdate:
		u := msg.Update
		m.serverConnected = u.Connected
		m.lspCount = u.LSPCount
		m.mcpCount = u.MCPCount
		m.mcpError = u.MCPError
		m.modelName = u.Model
		m.permissionCount = u.Permissions
		if u.Directory != "" {
			m.currentDir = u.Directory
		}
		if u.LSPList != nil {
			m.lspList = u.LSPList
		}
		if u.MCPList != nil {
			m.mcpList = u.MCPList
		}
		m.invalidateCache()
		return m, nil

	case MsgError:
		m.ai.Status = protocol.AIError
		m.ai.ErrorMsg = msg.Err
		if m.session != nil {
			m.session.AppendMessage(NewErrorMessage(msg.Err))
		}
		m.invalidateCache()
		m.viewport.GotoBottom()
		return m, nil
	}

	return m, nil
}

// ─── Resize ──────────────────────────────────────────────────

func (m *Model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	theme.TermWidth = msg.Width
	theme.TermHeight = msg.Height

	vpHeight := m.bodyHeight(msg.Height)
	showPanel := m.panelVisible(msg.Width)

	var mainW int
	if showPanel {
		panelW := panelWidthMax
		if panelW > msg.Width-60 {
			panelW = msg.Width - 60
		}
		if panelW < panelWidthMin {
			panelW = panelWidthMin
		}
		mainW = msg.Width - panelW - 1
		m.rightPanelHeight = vpHeight
	} else {
		mainW = msg.Width
		m.rightPanelHeight = 0
	}

	m.viewport.Width = mainW
	m.viewport.Height = vpHeight
	m.ready = true
	m.invalidateCache()
	return m, nil
}

// ─── Key Handler ─────────────────────────────────────────────

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Tab: cycle focus zones (unless in slash command where Tab does completion) ──
	if msg.Type == tea.KeyTab {
		if m.focusZone == FocusInput && m.prompt == "/" {
			return m.handleCommandTab()
		}
		m.cycleFocus()
		return m, nil
	}

	// ── Handle based on current focus zone ──
	switch m.focusZone {
	case FocusInput:
		return m.handleInputKey(msg)
	case FocusPanel:
		return m.handlePanelKey(msg)
	default:
		return m.handleMainKey(msg)
	}
}

func (m *Model) cycleFocus() {
	switch m.focusZone {
	case FocusMain:
		if m.panelMode != "hide" {
			// In auto mode, check if panel would actually be visible
			if m.panelVisible(theme.TermWidth) {
				m.focusZone = FocusPanel
			} else {
				m.focusZone = FocusInput
				m.inputFocused = true
			}
		} else {
			m.focusZone = FocusInput
			m.inputFocused = true
		}
	case FocusPanel:
		m.focusZone = FocusInput
		m.inputFocused = true
	case FocusInput:
		m.focusZone = FocusMain
		m.inputFocused = false
	}
}

// ─── Main zone keys (browse/scroll) ─────────────────────────

func (m Model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Enter in browse mode → focus input
		m.focusZone = FocusInput
		m.inputFocused = true
		return m, nil

	case tea.KeyEscape:
		return m, nil

	case tea.KeyCtrlC:
		// Ctrl+C: clear input and switch to input focus
		m.focusZone = FocusInput
		m.inputFocused = true
		m.input = ""
		m.prompt = ">"
		m.invalidateCache()
		return m, nil

	case tea.KeyCtrlD:
		if m.session != nil {
			m.session.Save() // best-effort save before exit
		}
		return m, tea.Quit

	case tea.KeyCtrlP:
		// Ctrl+P: open command palette dialog
		return m.openCommandPalette()

	case tea.KeyCtrlF:
		// Ctrl+F: open model favorites/list dialog
		return m.openModelDialog()

	case tea.KeyRunes:
		// Only '/' auto-focuses input for command entry
		if string(msg.Runes) == "/" {
			m.focusZone = FocusInput
			m.inputFocused = true
			m.input = ""
			m.prompt = "/"
			return m, nil
		}
		// All other keys: no-op in browse mode
		return m, nil

	case tea.KeyBackspace, tea.KeyDelete:
		return m, nil

	default:
		// Scroll keys for main viewport
		m.handleViewportScroll(msg)
		return m, nil
	}
}

// ─── Panel zone keys (browse right panel) ───────────────────

func (m Model) handlePanelKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.focusZone = FocusInput
		m.inputFocused = true
		return m, nil

	case tea.KeyEscape:
		m.focusZone = FocusMain
		return m, nil

	case tea.KeyCtrlP:
		// Ctrl+P: open command palette from panel zone
		return m.openCommandPalette()

	case tea.KeyCtrlF:
		// Ctrl+F: open model dialog from panel zone
		return m.openModelDialog()

	case tea.KeyCtrlC:
		// Ctrl+C: focus main
		m.focusZone = FocusMain
		m.invalidateCache()
		return m, nil

	case tea.KeyCtrlD:
		if m.session != nil {
			m.session.Save()
		}
		return m, tea.Quit

	case tea.KeySpace:
		// Toggle expand/collapse of the active section
		m.toggleSection(m.activeRightTab)
		return m, nil

	case tea.KeyRunes:
		if string(msg.Runes) == "/" {
			m.focusZone = FocusInput
			m.inputFocused = true
			m.input = ""
			m.prompt = "/"
			return m, nil
		}
		return m, nil

	case tea.KeyBackspace, tea.KeyDelete:
		return m, nil

	default:
		// 'h' or left → previous section
		// 'l' or right → next section
		switch msg.String() {
		case "h", "left":
			m.cycleSection(-1)
		case "l", "right":
			m.cycleSection(1)
		default:
			// j/k scroll the panel
			switch {
			case msg.Type == tea.KeyDown || msg.String() == "j":
				if m.rightPanelScroll < m.rightPanelHeight {
					m.rightPanelScroll++
				}
			case msg.Type == tea.KeyUp || msg.String() == "k":
				if m.rightPanelScroll > 0 {
					m.rightPanelScroll--
				}
			case msg.String() == "g":
				m.rightPanelScroll = 0
			case msg.String() == "G":
				m.rightPanelScroll = m.rightPanelHeight
			}
		}
		return m, nil
	}
}

// ─── Input zone keys (text editing) ─────────────────────────

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {

	case tea.KeyEnter:
		return m.handleSubmit()

	case tea.KeyEscape:
		// Unfocus input → back to browse mode
		m.focusZone = FocusMain
		m.inputFocused = false
		if m.prompt == "/" || m.input != "" {
			m.input = ""
			m.prompt = ">"
		}
		return m, nil

	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil

	case tea.KeyCtrlP:
		// Ctrl+P: open command palette from input zone
		return m.openCommandPalette()

	case tea.KeyCtrlF:
		// Ctrl+F: open model dialog from input zone
		return m.openModelDialog()

	case tea.KeyCtrlC:
		// Ctrl+C: clear input
		m.input = ""
		m.prompt = ">"
		m.invalidateCache()
		return m, nil

	case tea.KeyCtrlD:
		if m.session != nil {
			m.session.Save()
		}
		return m, tea.Quit

	case tea.KeyRunes:
		text := string(msg.Runes)
		if len(m.input) == 0 && text == "/" {
			m.prompt = "/"
			return m, nil
		}
		m.input += text
		return m, nil

	default:
		return m, nil
	}
}

// ─── Viewport scroll (shared for main) ──────────────────────

func (m *Model) handleViewportScroll(msg tea.KeyMsg) {
	switch {
	case msg.Type == tea.KeyDown || msg.String() == "j":
		m.viewport.LineDown(1)
	case msg.Type == tea.KeyUp || msg.String() == "k":
		m.viewport.LineUp(1)
	case msg.String() == "g":
		m.viewport.GotoTop()
	case msg.String() == "G":
		m.viewport.GotoBottom()
	case msg.String() == "ctrl+d":
		m.viewport.HalfViewDown()
	case msg.String() == "ctrl+u":
		m.viewport.HalfViewUp()
	case msg.String() == "pgdown":
		m.viewport.HalfViewDown()
	case msg.String() == "pgup":
		m.viewport.HalfViewUp()
	}
}

// ─── Mouse Handler ───────────────────────────────────────────

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.MouseWheelUp:
		if m.focusZone == FocusPanel {
			if m.rightPanelScroll > 0 {
				m.rightPanelScroll -= 3
			}
		} else {
			m.viewport.LineUp(3)
		}
		return m, nil

	case tea.MouseWheelDown:
		if m.focusZone == FocusPanel {
			m.rightPanelScroll += 3
		} else {
			m.viewport.LineDown(3)
		}
		return m, nil

	case tea.MouseLeft:
		// Click detection for input focus
		// Input is at the bottom of the screen
		// Layout: header(1) + sep(1) + viewport(vpHeight) + input(3) + footer(1)
		headerHeight := 1
		footerHeight := 1
		inputHeight := 3
		sepLines := 1
		vpHeight := theme.TermHeight - headerHeight - footerHeight - inputHeight - sepLines - 2

		inputStartY := headerHeight + sepLines + vpHeight + 1 // +1 for spacing

		if msg.Y >= inputStartY && msg.Y < inputStartY+inputHeight {
			// Click on input area → focus input
			m.focusZone = FocusInput
			m.inputFocused = true
			return m, nil
		}

		// Click on right panel area
		if m.panelVisible(theme.TermWidth) {
			panelLeft := theme.TermWidth - panelWidthMax
			if msg.X >= panelLeft && msg.Y >= headerHeight+sepLines && msg.Y < headerHeight+sepLines+vpHeight {
				m.focusZone = FocusPanel
				return m, nil
			}
		}

		// Click on main content area
		m.focusZone = FocusMain
		return m, nil
	}

	return m, nil
}

// ─── Section cycling ─────────────────────────────────────────

var sectionOrder = []string{"mcp", "lsp", "todo", "files"}

func (m *Model) toggleSection(id string) {
	if _, ok := m.panelSections[id]; ok {
		m.panelSections[id] = !m.panelSections[id]
		m.rightPanelScroll = 0
	}
}

func (m *Model) cycleSection(dir int) {
	current := -1
	for i, id := range sectionOrder {
		if id == m.activeRightTab {
			current = i
			break
		}
	}
	if current < 0 {
		m.activeRightTab = sectionOrder[0]
		return
	}
	next := (current + dir + len(sectionOrder)) % len(sectionOrder)
	m.activeRightTab = sectionOrder[next]
	m.rightPanelScroll = 0
}

// ─── Tab switching ───────────────────────────────────────────

func (m *Model) switchTab(dir int) {
	if len(m.rightTabIDs) == 0 {
		return
	}
	current := -1
	for i, id := range m.rightTabIDs {
		if id == m.activeRightTab {
			current = i
			break
		}
	}
	if current < 0 {
		m.activeRightTab = m.rightTabIDs[0]
		return
	}
	next := (current + dir + len(m.rightTabIDs)) % len(m.rightTabIDs)
	m.activeRightTab = m.rightTabIDs[next]
	m.rightPanelScroll = 0
}

// ─── Extract panel data from render command ──────────────────

func (m *Model) extractPanelData() {
	if m.renderCmd == nil {
		return
	}
	for _, comp := range m.renderCmd.Components {
		if comp.Type == protocol.CompSidePanel {
			// Auto-enable panel only if user hasn't explicitly hidden it
			if m.panelMode == "" {
				m.panelMode = "auto"
			}
			m.panelRenderCmd = &comp

			// Extract tab IDs
			if tabsRaw, ok := comp.Props["tabs"].([]interface{}); ok {
				var ids []string
				for _, t := range tabsRaw {
					if tab, ok := t.(map[string]interface{}); ok {
						if id, ok := tab["id"].(string); ok {
							ids = append(ids, id)
						}
					}
				}
				m.rightTabIDs = ids
				// Set default active tab
				if m.activeRightTab == "" && len(ids) > 0 {
					m.activeRightTab = ids[0]
				}
				// Validate active tab still exists
				valid := false
				for _, id := range ids {
					if id == m.activeRightTab {
						valid = true
						break
					}
				}
				if !valid && len(ids) > 0 {
					m.activeRightTab = ids[0]
				}
			}
			return
		}
	}
	// No side-panel found → hide it
	if m.panelMode == "" {
		m.panelMode = "hide"
	}
}

// ─── Command tab completion ────────────────────────────────

// commandNames lists all valid slash commands (without / prefix).
var commandNames = []string{"help", "clear", "model", "theme", "panel", "exit", "quit", "q"}

func (m Model) handleCommandTab() (tea.Model, tea.Cmd) {
	if m.input == "" {
		// If empty, insert "help" as default completion
		m.input = "help "
		return m, nil
	}

	// Split input: first word is command, rest are arguments
	parts := splitInput(m.input)

	if len(parts) == 1 {
		// Completing command name
		prefix := parts[0]
		var matches []string
		for _, cmd := range commandNames {
			if strings.HasPrefix(cmd, prefix) {
				matches = append(matches, cmd)
			}
		}
		if len(matches) == 1 {
			// Unique match — complete with trailing space
			m.input = matches[0] + " "
		} else if len(matches) > 0 && prefix != "" {
			// Multiple matches — find common prefix
			common := longestCommonPrefix(matches)
			m.input = common
		}
		// If no matches, leave as-is
		return m, nil
	}

	// Has argument(s) — try argument completion
	// For now: if command is "theme" and we have partial arg name, complete it
	cmd := parts[0]
	argPrefix := parts[len(parts)-1]

	switch cmd {
	case "theme":
		var matches []string
		for _, t := range theme.ThemeNames {
			if strings.HasPrefix(t, argPrefix) {
				matches = append(matches, t)
			}
		}
		if len(matches) == 1 {
			// Build input with completed arg
			completion := matches[0]
			m.input = cmd + " " + completion + " "
		} else if len(matches) > 0 && argPrefix != "" {
			common := longestCommonPrefix(matches)
			m.input = cmd + " " + common
		}
	case "model":
		// Model name completion would query API — leave for future
	}

	return m, nil
}

// splitInput splits input by spaces, preserving content.
func splitInput(s string) []string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return nil
	}
	return parts
}

// longestCommonPrefix returns the longest common prefix among strings.
func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}
	prefix := strs[0]
	for i := 1; i < len(strs); i++ {
		j := 0
		for j < len(prefix) && j < len(strs[i]) && prefix[j] == strs[i][j] {
			j++
		}
		prefix = prefix[:j]
	}
	return prefix
}

// ─── Invalidate cache ───────────────────────────────────────

func (m *Model) invalidateCache() {
	m.cachedMain = ""
	m.rightPanelContent = ""
}

// ─── Submit ──────────────────────────────────────────────────

func (m Model) handleSubmit() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.input)

	// Block empty and whitespace-only messages
	if raw == "" {
		m.prompt = ">"
		m.invalidateCache()
		return m, nil
	}

	if m.prompt == "/" || strings.HasPrefix(raw, "/") {
		cmdLine := strings.TrimPrefix(raw, "/")
		cmdArgs := strings.Fields(cmdLine)
		result := m.execCommand(cmdLine)

		// Handle session-affecting commands
		if len(cmdArgs) > 0 && cmdArgs[0] == "clear" {
			dir := m.currentDir
			if dir == "" {
				dir, _ = os.Getwd()
				if dir == "" {
					dir = "~"
				}
			}
			m.session = NewSession(dir)
		}

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
		m.prompt = ">"
		m.invalidateCache()
		m.viewport.GotoBottom()
		return m, nil
	}

	if m.client != nil {
		m.client.SendMessage(raw)
		m.input = ""
		m.prompt = ">"
		m.invalidateCache()
		m.viewport.GotoBottom()
		return m, nil
	}

	// Standalone mode with API client
	if m.apiClient != nil {
		m.input = ""
		m.prompt = ">"

		// Append user message to session
		if m.session != nil {
			m.session.AppendMessage(NewUserMessage(raw))
		}

		m.ai.Status = protocol.AIThinking
		m.ai.Label = "thinking"
		m.reqStart = time.Now()
		m.reqFirstToken = time.Time{}
		m.reqTokensOut = 0
		m.invalidateCache()
		m.viewport.GotoBottom()

		// Start the streaming API call immediately (stores the channel on m)
		ch, err := m.apiClient.SendMessage(context.Background(), raw)
		if err != nil {
			m.ai.Status = protocol.AIError
			m.ai.ErrorMsg = err.Error()
			if m.session != nil {
				m.session.AppendMessage(NewErrorMessage(err.Error()))
			}
			m.invalidateCache()
			return m, nil
		}
		m.streamCh = ch
		// Return a command that reads the first chunk
		return m, m.apiNextChunk()
	}

	m.input = ""
	m.prompt = ">"
	m.invalidateCache()
	m.viewport.GotoBottom()
	return m, nil
}

// ─── Standalone API streaming helpers ────────────────────────

// apiStartStream starts an API call and returns a Cmd that reads the first event.
func (m Model) apiStartStream(userMsg string) tea.Cmd {
	return func() tea.Msg {
		ch, err := m.apiClient.SendMessage(context.Background(), userMsg)
		if err != nil {
			return MsgError{Err: err.Error()}
		}
		evt, ok := <-ch
		if !ok {
			return MsgStreamChunk{Done: true}
		}
		if evt.Error != nil {
			return MsgError{Err: evt.Error.Error()}
		}
		if evt.Done {
			return MsgStreamChunk{Done: true}
		}
		m.streamCh = ch
		return MsgStreamChunk{Content: evt.Content}
	}
}

// apiNextChunk reads the next event from streamCh and returns a Msg.
// Must only be called when streamCh is non-nil.
func (m Model) apiNextChunk() tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-m.streamCh
		if !ok {
			return MsgStreamChunk{Done: true}
		}
		if evt.Error != nil {
			return MsgError{Err: evt.Error.Error()}
		}
		if evt.Done {
			return MsgStreamChunk{Done: true}
		}
		return MsgStreamChunk{Content: evt.Content}
	}
}

// ─── Commands ────────────────────────────────────────────────

func (m Model) execCommand(cmd string) string {
	args := strings.Fields(cmd)
	if len(args) == 0 {
		return "Commands: /help /clear /model /theme /exit /panel"
	}
	switch args[0] {
	case "help":
		return `Commands:
  /help       Show this help
  /clear      Clear conversation
  /model      Show/set model
  /theme      Switch theme: /theme (list) or /theme <name>
  /panel      Toggle right panel (auto/hide)
  /exit       Quit

Navigation (browse mode):
  j / ↓       Scroll down
  k / ↑       Scroll up
  g / G       Top / bottom
  Ctrl+D/U    Half page scroll
  Mouse wheel Scroll
  Tab         Cycle focus: main → panel → input

Right panel (when panel focused):
  h / ←       Previous tab
  l / →       Next tab
  j / ↓       Scroll panel down
  k / ↑       Scroll panel up

Focus:
  Click input  Edit input
  /            Auto-focus input for command
  Enter        Focus input (from browse mode)
  Esc          Unfocus input`
	case "clear":
		m.renderCmd = nil
		m.invalidateCache()
		return "Conversation cleared."
	case "panel":
		if m.panelMode == "hide" {
			m.panelMode = "auto"
			m.invalidateCache()
			return "Right panel: auto mode (shows when wide)."
		}
		m.panelMode = "hide"
		m.invalidateCache()
		return "Right panel hidden."
	case "model":
		if len(args) > 1 {
			return "Model set to: " + args[1]
		}
		return "Current model: free:QwQ-32B"
	case "theme":
		if len(args) > 1 {
			if theme.SetTheme(args[1]) {
				return "Theme switched to: " + theme.ThemeDisplayName(args[1])
			}
			return "Available themes: " + strings.Join(theme.ThemeNames, ", ")
		}
		return "Current theme: " + theme.ThemeDisplayName(theme.CurrentTheme) + "\nAvailable: " + strings.Join(theme.ThemeNames, ", ")
	case "exit", "quit", "q":
		return "Press Ctrl+C to quit."
	default:
		return "Unknown command: /" + args[0] + ". Type /help"
	}
}

// ─── Layout constants ───────────────────────────────────────

const (
	panelWidthMax  = 42 // fixed right panel width (matching opencode-dev)
	panelWidthMin  = 28
	wideThreshold  = 120 // min terminal width for right panel auto-show
	inputBoxHeight = 3
	sepHeight      = 1
)

// panelVisible returns true if the right panel should be shown,
// based on terminal width and the panel mode.
func (m *Model) panelVisible(width int) bool {
	if m.panelMode == "hide" {
		return false
	}
	if m.panelMode == "auto" {
		// Hide if terminal is too narrow
		return width >= wideThreshold
	}
	// Default: show when there's panel data
	return width >= wideThreshold
}

// bodyHeight calculates the height available for the main content area.
func (m *Model) bodyHeight(height int) int {
	// header(1) + sep(1) + body + input(3) + statusLine(1) + sep(1) + footer(1)
	h := height - 1 - sepHeight - inputBoxHeight - 1 - sepHeight - 1
	if h < 5 {
		h = 5
	}
	return h
}

// ─── View ────────────────────────────────────────────────────

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	width := theme.TermWidth
	height := theme.TermHeight
	if width < 40 {
		width = 40
	}
	if height < 10 {
		height = 10
	}

	vpHeight := m.bodyHeight(height)

	// Panel visibility based on available width
	showPanel := m.panelVisible(width)

	var panelW, mainW int
	if showPanel {
		panelW = panelWidthMax
		if panelW > width-60 {
			panelW = width - 60
		}
		if panelW < panelWidthMin {
			panelW = panelWidthMin
		}
		mainW = width - panelW - 1 // -1 for vertical divider
	} else {
		panelW = 0
		mainW = width
	}

	var b strings.Builder

	// ── Header bar (1 line) ──
	b.WriteString(m.renderHeader(mainW, showPanel))
	b.WriteString("\n")

	// ── Separator ──
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.Muted).
		Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	// ── Scrollbar for main content area ──
	totalLines := max(m.viewport.TotalLineCount(), vpHeight)
	sb := tuiView.RenderStyledScrollbar(
		m.viewport.YOffset,
		totalLines,
		vpHeight,
		theme.ScrollbarTrackStyle,
		theme.ScrollbarThumbStyle,
	)
	sbLines := strings.Split(sb, "\n")
	for len(sbLines) < vpHeight {
		sbLines = append(sbLines, " ")
	}
	if len(sbLines) > vpHeight {
		sbLines = sbLines[:vpHeight]
	}

	// ── Body: main content + right panel ──
	if showPanel {
		mainRendered := m.renderMainContent(mainW)
		mainLines := strings.Split(mainRendered, "\n")

		panelRendered := m.renderRightPanel(panelW)
		panelLines := strings.Split(panelRendered, "\n")

		// Normalize both sides to vpHeight
		for len(mainLines) < vpHeight {
			mainLines = append(mainLines, "")
		}
		for len(panelLines) < vpHeight {
			panelLines = append(panelLines, "")
		}
		if len(mainLines) > vpHeight {
			mainLines = mainLines[:vpHeight]
		}
		if len(panelLines) > vpHeight {
			panelLines = panelLines[:vpHeight]
		}

		divider := lipgloss.NewStyle().
			Foreground(theme.Colors.Muted).
			Render("│")
		for i := 0; i < vpHeight; i++ {
			leftLine := mainLines[i]
			// Pad to mainW-1, scrollbar takes the last column of left area
			padded := lipgloss.NewStyle().Width(mainW - 1).Render(leftLine)
			rightLine := panelLines[i]
			if len(rightLine) < panelW {
				rightLine += strings.Repeat(" ", panelW-len(rightLine))
			}
			b.WriteString(padded)
			b.WriteString(sbLines[i])
			b.WriteString(divider)
			b.WriteString(rightLine)
			b.WriteString("\n")
		}
	} else {
		// Full-width main content + scrollbar at right edge
		content := m.renderMainContent(mainW)

		contentLines := strings.Split(content, "\n")
		for len(contentLines) < vpHeight {
			contentLines = append(contentLines, "")
		}
		if len(contentLines) > vpHeight {
			contentLines = contentLines[:vpHeight]
		}

		for i := 0; i < vpHeight; i++ {
			line := contentLines[i]
			// Pad to mainW-1, scrollbar fixed at rightmost column
			padded := lipgloss.NewStyle().Width(mainW - 1).Render(line)
			b.WriteString(padded)
			b.WriteString(sbLines[i])
			b.WriteString("\n")
		}
	}

	// ── Input bar ──
	promptSymbol := theme.PromptStyle.Render(m.prompt)
	displayText := m.input
	if m.input == "" && m.prompt == ">" {
		if m.inputFocused {
			displayText = ""
		} else {
			displayText = "Type a message... (/help)"
		}
	}

	inputBox := theme.InputStyle.Width(width - 4)
	if m.focusZone == FocusInput {
		inputBox = inputBox.BorderForeground(theme.Colors.Primary)
	} else {
		inputBox = inputBox.BorderForeground(theme.Colors.Muted)
	}
	b.WriteString(inputBox.Render(promptSymbol + " " + displayText))
	b.WriteString("\n")

	// ── Status line (model, tokens, MCP/LSP/agent counts) ──
	b.WriteString(m.renderStatusLine(width))
	b.WriteString("\n")

	// ── Separator before footer ──
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.Muted).
		Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	// ── Footer ──
	b.WriteString(m.renderFooter(width))

	finalView := replaceTabs(b.String())

	// ── Dialog overlay (rendered on top of everything) ──
	if m.dialogOpen && m.activeDialog != nil {
		dlg := m.activeDialog
		dlgH := dlg.Height()
		if dlgH < 8 {
			dlgH = 8
		}
		finalView = renderDialogOverlay(dlg, width, height, 0, dlgH)
	}

	return finalView
}

// ─── Render header ──────────────────────────────────────────

func (m Model) renderHeader(mainWidth int, hasPanel bool) string {
	// Connection indicator (always visible)
	connColor := theme.Colors.Success
	connText := "●"
	if !m.serverConnected {
		connColor = theme.Colors.Error
		connText = "○"
	}

	var leftParts []string
	leftParts = append(leftParts, lipgloss.NewStyle().Foreground(connColor).Render(connText))
	leftParts = append(leftParts, " ExtendAI Lab")

	// AI status indicator (right after title)
	if m.ai.IsWorking() {
		aiColor := theme.Colors.Accent
		if m.ai.Status == protocol.AIStreaming {
			aiColor = theme.Colors.Primary
		}
		spinner := m.ai.SpinnerFrame()
		label := m.ai.Label
		if label == "" {
			label = string(m.ai.Status)
		}
		leftParts = append(leftParts,
			lipgloss.NewStyle().Foreground(aiColor).Render(" "+spinner+" "+label))
	} else if m.ai.Status == protocol.AIError {
		leftParts = append(leftParts,
			lipgloss.NewStyle().Foreground(theme.Colors.Error).Render(" ✗ error"))
	}

	// Date/time (always visible, for fullscreen use)
	now := time.Now()
	timeStr := now.Format("15:04")
	dateStr := now.Format("2006-01-02")
	leftParts = append(leftParts,
		lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(" ─ "+dateStr+" "+timeStr))

	leftText := strings.Join(leftParts, "")

	// Right side: model name + API URL + focus zone
	var rightParts []string
	if m.modelName != "" {
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render(m.modelName))
	}
	if m.modelURL != "" {
		// Truncate URL to prevent overflow
		urlStr := m.modelURL
		if len(urlStr) > 40 {
			urlStr = urlStr[:37] + "..."
		}
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(urlStr))
	}
	if hasPanel || m.focusZone != FocusMain {
		zoneLabel := "[main]"
		switch m.focusZone {
		case FocusInput:
			zoneLabel = "[input]"
		case FocusPanel:
			zoneLabel = "[panel]"
		}
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(zoneLabel))
	}

	rightText := strings.Join(rightParts, "  ")

	// Compute fill: HeaderStyle has Padding(0,2) = 4 chars consumed
	avail := theme.TermWidth - 4
	if avail < 10 {
		avail = 10
	}
	fillLen := avail - len(leftText) - len(rightText)
	if fillLen < 1 {
		fillLen = 1
	}
	fill := strings.Repeat(" ", fillLen)

	return theme.HeaderStyle.Render(leftText + fill + rightText)
}

// ─── Render status line ──────────────────────────────────

func (m Model) renderStatusLine(width int) string {
	var leftParts, rightParts []string

	// Left: model name + conversation count
	if m.modelName != "" {
		leftParts = append(leftParts,
			lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render("agent:"),
			lipgloss.NewStyle().Foreground(theme.Colors.Text).Render(m.modelName))
	}
	// Message count
	if m.session != nil {
		msgCount := m.session.GetMessageCount()
		if msgCount > 0 {
			leftParts = append(leftParts,
				lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(fmt.Sprintf(" msgs:%d", msgCount)))
		}
	}

	// Right: MCP / LSP counts
	if m.mcpCount > 0 {
		mcpColor := theme.Colors.Success
		if m.mcpError {
			mcpColor = theme.Colors.Error
		}
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(mcpColor).Render(fmt.Sprintf("MCP:%d", m.mcpCount)))
	}
	if m.lspCount > 0 {
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(theme.Colors.Success).Render(fmt.Sprintf("LSP:%d", m.lspCount)))
	}

	leftText := strings.Join(leftParts, " ")
	rightText := strings.Join(rightParts, "  ")

	avail := width - 4
	if avail < 10 {
		avail = 10
	}
	fillLen := avail - len(leftText) - len(rightText)
	if fillLen < 1 {
		fillLen = 1
	}
	fill := strings.Repeat(" ", fillLen)

	return lipgloss.NewStyle().
		Width(width).
		Foreground(theme.Colors.TextDim).
		Render(leftText + fill + rightText)
}

// ─── Render footer ─────────────────────────────────────────

func (m Model) renderFooter(width int) string {
	// ── Left: directory ──
	dir := m.currentDir
	if dir == "" {
		dir = "~"
	}
	leftText := lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).
		Render(dir)

	// ── Center: LSP + MCP + permissions ──
	var centerParts []string
	lspColor := theme.Colors.TextDim
	if m.lspCount > 0 {
		lspColor = theme.Colors.Success
	}
	centerParts = append(centerParts,
		lipgloss.NewStyle().Foreground(lspColor).Render("●"),
		fmt.Sprintf(" %d", m.lspCount))

	if m.mcpCount > 0 {
		mcpColor := theme.Colors.Success
		if m.mcpError {
			mcpColor = theme.Colors.Error
		}
		centerParts = append(centerParts, " ",
			lipgloss.NewStyle().Foreground(mcpColor).Render("⊙"),
			fmt.Sprintf(" %d", m.mcpCount))
	}
	if m.permissionCount > 0 {
		centerParts = append(centerParts, " ",
			lipgloss.NewStyle().Foreground(theme.Colors.Warning).Render("△"),
			fmt.Sprintf("%d", m.permissionCount))
	}

	centerText := strings.Join(centerParts, "")

	// ── Right: theme name (dot color already shows connected/disconnected) ──
	dotColor := theme.Colors.Success
	if !m.serverConnected {
		dotColor = theme.Colors.Error
	}
	themeName := theme.ThemeDisplayName(theme.CurrentTheme)
	rightText := lipgloss.NewStyle().Foreground(dotColor).Render("●") +
		" " +
		lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(themeName)

	// ── Layout: FooterStyle has Padding(0,1) = 2 chars consumed ──
	avail := width - 2 // content area inside footer padding

	// Compute proportional widths that sum to avail
	// Left 25%, right 30%, center 45%
	leftW := avail * 25 / 100
	rightW := avail * 30 / 100
	centerW := avail - leftW - rightW

	// Minimum safe widths (prevent word wrap)
	const minLeft = 10
	const minCenter = 16
	const minRight = 16

	if leftW < minLeft {
		leftW = minLeft
	}
	if rightW < minRight {
		rightW = minRight
	}
	// Recalc center if we stole from it
	centerW = avail - leftW - rightW
	if centerW < minCenter && avail >= minLeft+minRight+minCenter {
		// Steal from left first
		leftW = avail - rightW - minCenter
		if leftW < minLeft {
			leftW = minLeft
		}
		centerW = avail - leftW - rightW
	}
	if centerW < 4 {
		centerW = 4
	}

	// Render each piece at exact width so nothing bleeds
	lBlock := lipgloss.NewStyle().Width(leftW).MaxWidth(leftW).Align(lipgloss.Left).
		Foreground(theme.Colors.TextDim).
		Render(m.truncStr(leftText, leftW))
	cBlock := lipgloss.NewStyle().Width(centerW).MaxWidth(centerW).Align(lipgloss.Center).
		Render(m.truncStr(centerText, centerW))
	rBlock := lipgloss.NewStyle().Width(rightW).MaxWidth(rightW).Align(lipgloss.Right).
		Render(m.truncStr(rightText, rightW))

	return theme.FooterStyle.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, lBlock, cBlock, rBlock),
	)
}

// truncStr truncates s to at most maxLen runes, appending "…" if cut.
func (m Model) truncStr(s string, maxLen int) string {
	if maxLen < 2 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

// ─── Render main content ────────────────────────────────────

func (m *Model) renderMainContent(width int) string {
	if m.cachedMain != "" && !m.ai.IsWorking() {
		return m.cachedMain
	}

	m.viewport.Width = width

	var contentLines []string

	// 1. Session messages (persistent conversation)
	if m.session != nil {
		msgs := m.session.GetMessages()
		for i, msg := range msgs {
			// Determine brightness style
			brightStyle := lipgloss.NewStyle()
			if msg.Brightness == BrightnessDim {
				brightStyle = brightStyle.Faint(true)
			}

			switch msg.Role {
			case RoleUser:
				contentLines = append(contentLines, theme.UserStyle.Render("You"))
				contentLines = append(contentLines, msg.Content)

			case RoleAssistant:
				contentLines = append(contentLines, theme.AssistantStyle.Render("Assistant"))
				mdWidth := width - 4
				if mdWidth < 20 {
					mdWidth = 20
				}
				rendered := renderer.RenderMarkdown(msg.Content, mdWidth)
				// Apply brightness
				if msg.Brightness == BrightnessDim {
					rendered = brightStyle.Render(rendered)
				}
				contentLines = append(contentLines, rendered)

				// Timing footer for assistant messages
				widthFull := width - 4
				if widthFull > 80 {
					// Full mode
					timing := msg.TimingFull()
					if timing != "" {
						contentLines = append(contentLines,
							lipgloss.NewStyle().
								Foreground(theme.Colors.TextDim).
								Italic(true).
								Width(widthFull).
								Render(timing))
					}
				} else {
					// Compact mode
					timing := msg.TimingShort()
					if timing != "" {
						contentLines = append(contentLines,
							lipgloss.NewStyle().
								Foreground(theme.Colors.TextDim).
								Italic(true).
								Width(widthFull).
								Render(timing))
					}
				}

			case RoleTool:
				// Tool calls: dimmer display
				toolHeader := lipgloss.NewStyle().
					Foreground(theme.Colors.Muted).
					Render("Tool: " + msg.ToolName)
				contentLines = append(contentLines, brightStyle.Render(toolHeader))
				mdWidth := width - 4
				if mdWidth < 20 {
					mdWidth = 20
				}
				rendered := renderer.RenderMarkdown(msg.Content, mdWidth)
				contentLines = append(contentLines, brightStyle.Render(rendered))

			case RoleError:
				contentLines = append(contentLines, theme.ErrorStyle.Render(msg.Content))

			case RoleSystem:
				contentLines = append(contentLines, theme.SystemStyle.Render(msg.Content))
			}

			// Separator between messages (except last)
			if i < len(msgs)-1 {
				contentLines = append(contentLines, "")
			}
		}
	}

	// 2. TS-rendered components (filter out side-panel)
	if m.renderCmd != nil && len(m.renderCmd.Components) > 0 {
		var mainComps []protocol.Component
		for _, comp := range m.renderCmd.Components {
			if comp.Type != protocol.CompSidePanel {
				mainComps = append(mainComps, comp)
			}
		}
		if len(mainComps) > 0 {
			filteredCmd := &protocol.RenderCommand{
				Components: mainComps,
			}
			tsContent := renderer.RenderAllComponents(filteredCmd, width)
			if tsContent != "" {
				contentLines = append(contentLines, tsContent)
			}
		}
	}

	// 3. Streaming content (AI response in progress)
	if m.ai.Status == protocol.AIStreaming && m.ai.Content != "" {
		mdWidth := width - 4
		if mdWidth < 20 {
			mdWidth = 20
		}
		rendered := renderer.RenderMarkdown(m.ai.Content, mdWidth)
		contentLines = append(contentLines,
			theme.AssistantStyle.Render("Assistant"), rendered)
	}

	// 4. Welcome message if nothing else
	if len(contentLines) == 0 {
		contentLines = append(contentLines,
			theme.SystemStyle.Render("Welcome to ExtendAI Lab. Type /help for commands."))
	}

	fullContent := strings.Join(contentLines, "\n")
	m.cachedMain = fullContent
	m.viewport.SetContent(fullContent)
	return fullContent
}

// ─── Render right panel ─────────────────────────────────────

func (m *Model) renderRightPanel(width int) string {
	contentWidth := width - 4 // padding on both sides

	var sectionLines []string

	// ── MCP section ──
	{
		expanded := m.panelSections["mcp"]
		var summary string
		if len(m.mcpList) > 0 {
			connected := 0
			for _, mcp := range m.mcpList {
				if mcp.Status == "connected" {
					connected++
				}
			}
			summary = fmt.Sprintf("(%d/%d) ", connected, len(m.mcpList))
		}
		header := m.renderSectionHeader("MCP", expanded, summary)
		sectionLines = append(sectionLines, header)
		if expanded {
			content := m.renderMCPTab(contentWidth)
			for _, line := range strings.Split(content, "\n") {
				sectionLines = append(sectionLines, theme.PanelStyle.Render(line))
			}
		}
	}

	// ── LSP section ──
	{
		expanded := m.panelSections["lsp"]
		var summary string
		if len(m.lspList) > 0 {
			errCount := 0
			for _, lsp := range m.lspList {
				if lsp.Status == "error" {
					errCount++
				}
			}
			summary = fmt.Sprintf("(%d err) ", errCount)
		}
		header := m.renderSectionHeader("LSP", expanded, summary)
		sectionLines = append(sectionLines, header)
		if expanded {
			content := m.renderLSPTab(contentWidth)
			for _, line := range strings.Split(content, "\n") {
				sectionLines = append(sectionLines, theme.PanelStyle.Render(line))
			}
		}
	}

	// ── TODO section ──
	{
		expanded := m.panelSections["todo"]
		header := m.renderSectionHeader("TODO", expanded, "")
		sectionLines = append(sectionLines, header)
		if expanded {
			content := m.renderTodoTab(contentWidth)
			for _, line := range strings.Split(content, "\n") {
				sectionLines = append(sectionLines, theme.PanelStyle.Render(line))
			}
		}
	}

	// ── Files section ──
	{
		expanded := m.panelSections["files"]
		header := m.renderSectionHeader("Files", expanded, "")
		sectionLines = append(sectionLines, header)
		if expanded {
			content := m.renderFilesTab(contentWidth)
			for _, line := range strings.Split(content, "\n") {
				sectionLines = append(sectionLines, theme.PanelStyle.Render(line))
			}
		}
	}

	// Apply scroll offset
	maxScroll := len(sectionLines) - m.rightPanelHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.rightPanelScroll > maxScroll {
		m.rightPanelScroll = maxScroll
	}
	start := m.rightPanelScroll
	visible := sectionLines[start:]
	if len(visible) > m.rightPanelHeight {
		visible = visible[:m.rightPanelHeight]
	}

	// Wrap in panel style
	return theme.PanelStyle.Width(width).Render(strings.Join(visible, "\n"))
}

// renderSectionHeader renders a collapsible section header for the right panel.
func (m *Model) renderSectionHeader(name string, expanded bool, summary string) string {
	toggle := "▶"
	if expanded {
		toggle = "▼"
	}
	color := theme.Colors.TextDim
	if m.activeRightTab == m.sectionIDByName(name) {
		color = theme.Colors.Accent
	}
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(color).
		Render(toggle + " " + name)
	dim := lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).
		Render(summary)
	return header + " " + dim
}

// sectionIDByName maps section display names to internal IDs.
func (m *Model) sectionIDByName(name string) string {
	switch name {
	case "MCP":
		return "mcp"
	case "LSP":
		return "lsp"
	case "TODO":
		return "todo"
	case "Files":
		return "files"
	}
	return ""
}

// renderPanelTabs renders the tab bar at the top of the right panel.
func (m *Model) renderPanelTabs(width int) string {
	if len(m.rightTabIDs) == 0 {
		return ""
	}

	tabNames := map[string]string{
		"lsp":   "LSP",
		"files": "Files",
		"todo":  "TODO",
		"mcp":   "MCP",
	}

	var tabStrs []string
	for _, id := range m.rightTabIDs {
		name := id
		if n, ok := tabNames[id]; ok {
			name = n
		}
		if id == m.activeRightTab {
			tabStrs = append(tabStrs, theme.TabActiveStyle.Render(name))
		} else {
			tabStrs = append(tabStrs, theme.TabInactiveStyle.Render(name))
		}
	}
	return theme.TabBarStyle.Width(width).Render(strings.Join(tabStrs, ""))
}

// renderActiveTabContent renders content for the currently active right-panel tab.
func (m *Model) renderActiveTabContent(width int) string {
	switch m.activeRightTab {
	case "lsp":
		return m.renderLSPTab(width)
	case "files":
		return m.renderFilesTab(width)
	case "todo":
		return m.renderTodoTab(width)
	case "mcp":
		return m.renderMCPTab(width)
	default:
		return ""
	}
}

// renderLSPTab shows language server status.
func (m *Model) renderLSPTab(width int) string {
	var b strings.Builder

	if len(m.lspList) > 0 {
		for _, lsp := range m.lspList {
			dotColor := theme.Colors.Success
			statusLabel := "connected"
			switch lsp.Status {
			case "starting":
				dotColor = theme.Colors.Warning
				statusLabel = "starting"
			case "error":
				dotColor = theme.Colors.Error
				statusLabel = "error"
			}
			b.WriteString(
				lipgloss.NewStyle().Foreground(dotColor).Render("●") +
					" " +
					lipgloss.NewStyle().Foreground(theme.Colors.Text).Render(lsp.ID) +
					" " +
					lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(statusLabel) +
					"\n",
			)
		}
	} else {
		b.WriteString(
			lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render("No LSPs active") + "\n" +
				lipgloss.NewStyle().Foreground(theme.Colors.Muted).Render("LSPs activate when files are read"),
		)
	}
	return b.String()
}

// renderFilesTab shows modified files (placeholder).
func (m *Model) renderFilesTab(width int) string {
	return lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).
		Render("No modified files")
}

// renderTodoTab shows todo items (placeholder).
func (m *Model) renderTodoTab(width int) string {
	return lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).
		Render("No todo items")
}

// renderMCPTab shows MCP server status.
func (m *Model) renderMCPTab(width int) string {
	var b strings.Builder
	if len(m.mcpList) > 0 {
		for _, mcp := range m.mcpList {
			dotColor := theme.Colors.Success
			if mcp.Status == "failed" {
				dotColor = theme.Colors.Error
			}
			b.WriteString(
				lipgloss.NewStyle().Foreground(dotColor).Render("⊙") +
					" " +
					lipgloss.NewStyle().Foreground(theme.Colors.Text).Render(mcp.Name) +
					"\n",
			)
		}
	} else {
		b.WriteString(
			lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render("No MCP servers"),
		)
	}
	return b.String()
}

// ─── Session helpers ──────────────────────────────────────────

// loadOrCreateSession loads the most recent session or creates a new one.
func loadOrCreateSession(cwd string) *Session {
	// Try to load most recent session
	sessions, err := ListSessions()
	if err == nil && len(sessions) > 0 {
		// Load the most recent session
		s, err := LoadSession(sessions[0])
		if err == nil {
			return s
		}
	}

	// Create new session
	return NewSession(cwd)
}

// ─── Tab fix ──────────────────────────────────────────────────

// replaceTabs replaces tab characters with spaces in the rendered output.
// This prevents alignment issues in terminals where tabs render at fixed stops.
func replaceTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

// ─── Helpers ─────────────────────────────────────────────────

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ─── Dialog helpers ─────────────────────────────────────────

// openCommandPalette opens the command palette dialog (Ctrl+P).
func (m Model) openCommandPalette() (tea.Model, tea.Cmd) {
	items := []DialogItem{
		{Title: "Clear conversation", Description: "/clear", Category: "Commands", Value: "/clear"},
		{Title: "Switch theme", Description: "Change visual theme", Category: "Commands", Value: "/theme"},
		{Title: "Toggle panel", Description: "Show/hide right panel", Category: "Commands", Value: "/panel"},
		{Title: "Show help", Description: "View all commands", Category: "Commands", Value: "/help"},
		{Title: "Exit", Description: "Quit ExtendAI Lab", Category: "Commands", Value: "/exit"},
		{Title: "Switch model", Description: "Choose a different AI model", Category: "Models", Value: "/model"},
	}
	ds := NewDialogSelect("Command Palette", items, 15,
		func(item DialogItem) (tea.Model, tea.Cmd) {
			m.input = item.Value
			m.prompt = "/"
			m.focusZone = FocusInput
			m.dialogOpen = false
			m.activeDialog = nil
			m.invalidateCache()
			return m, nil
		},
	)
	m.activeDialog = ds
	m.dialogOpen = true
	m.invalidateCache()
	return m, nil
}

// openModelDialog opens the model list dialog (Ctrl+F).
func (m Model) openModelDialog() (tea.Model, tea.Cmd) {
	items := []DialogItem{
		{Title: "QwQ-32B", Description: "Default reasoning model", Category: "Free", Value: "QwQ-32B"},
		{Title: "DeepSeek-R1", Description: "Advanced reasoning", Category: "Pro", Value: "DeepSeek-R1"},
		{Title: "GPT-4o", Description: "OpenAI flagship", Category: "Pro", Value: "GPT-4o"},
		{Title: "Claude 3.5 Sonnet", Description: "Anthropic flagship", Category: "Pro", Value: "claude-3.5-sonnet"},
		{Title: "Local (LM Studio)", Description: "Local inference", Category: "Local", Value: "local"},
	}
	ds := NewDialogSelect("Select Model", items, 15,
		func(item DialogItem) (tea.Model, tea.Cmd) {
			m.modelName = item.Value
			m.dialogOpen = false
			m.activeDialog = nil
			m.invalidateCache()
			return m, nil
		},
	)
	m.activeDialog = ds
	m.dialogOpen = true
	m.invalidateCache()
	return m, nil
}

// ─── Interface guards ────────────────────────────────────────

var _ tea.Model = (*Model)(nil)
