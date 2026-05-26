package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

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
type MsgDialogSelect struct {
	DlgType DialogType
	Value   string
}
type MsgSessionSelect struct {
	Path  string
	New   bool
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
	// Startup session picker
	sessionPickerOpen bool

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
		panelMode: "auto", // always auto-show when wide enough
		rightTabIDs: []string{"lsp", "files", "todo", "session"},
		activeRightTab: "session",
		panelSections: map[string]bool{
			"mcp":     true,   // expanded by default
			"lsp":     true,
			"todo":    false,
			"files":   false,
			"session": true,   // expanded by default
		},
	}

	// In standalone mode, set the model name from the API client
	if aiClient != nil {
		m.serverConnected = true
		m.modelName = aiClient.Model()
		m.modelURL = aiClient.BaseURL()
	}

	// Initialize session (load most recent or prompt selection)
	dir := m.currentDir
	if dir == "" {
		dir, _ = os.Getwd()
		if dir == "" {
			dir = "~"
		}
	}
	if session, files := loadRecentSessionOrPrompt(dir); session != nil {
		m.session = session
	} else if len(files) > 0 {
		m.sessionPickerOpen = true
		m.activeDialog = newSessionPickerDialog(files, dir)
		m.dialogOpen = true
	} else {
		m.session = NewSession(dir)
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

// ─── Update ──────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// ── When a dialog is active, route keyboard to dialog,
	//    but DO NOT block background processing (stream chunks, AI status, etc.)
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
			m.invalidateCache()
			return m, nil
		case tea.WindowSizeMsg:
			// Pass resize through so dialogs can react
			m.handleResize(msg)
			return m, nil
		default:
			// CRITICAL: Non-keyboard messages (MsgStreamChunk, MsgAIStatus,
			// MsgRenderCommand, MsgServerStatus, MsgError, etc.) must still
			// be processed so background AI streaming continues.
			// Dialog is a visual overlay, NOT a processing blocker.
			break // fall through to normal processing
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

	case MsgDialogSelect:
		m.dialogOpen = false
		m.activeDialog = nil
		switch msg.DlgType {
		case DialogPalette:
			// Special-case /help: open help dialog instead of inserting text
			if msg.Value == "/help" {
				return m.openHelpDialog()
			}
			// Execute the selected command as slash command
			m.input = msg.Value
			m.prompt = "/"
			m.focusZone = FocusInput
			m.inputFocused = true
			m.invalidateCache()
			return m, nil
		case DialogModel:
			// Model selection
			m.modelName = msg.Value
			m.invalidateCache()
			return m, nil
		case DialogHelpType, DialogHelp:
			m.invalidateCache()
			return m, nil
		default:
			m.invalidateCache()
			return m, nil
		}

	case MsgSessionSelect:
		if msg.New {
			dir := m.currentDir
			if dir == "" {
				dir, _ = os.Getwd()
			}
			m.session = NewSession(dir)
			m.sessionPickerOpen = false
			m.dialogOpen = false
			m.activeDialog = nil
			return m, nil
		}
		loaded, err := LoadSession(msg.Path)
		if err != nil {
			m.ai.Status = protocol.AIError
			m.ai.ErrorMsg = err.Error()
			m.session = NewSession(m.currentDir)
		} else {
			m.session = loaded
		}
		m.sessionPickerOpen = false
		m.dialogOpen = false
		m.activeDialog = nil
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
	// ── Global keys (work in any focus zone) ──
	if msg.Type == tea.KeyF1 {
		return m.openHelpDialog()
	}

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
			_, size := utf8.DecodeLastRuneInString(m.input)
			m.input = m.input[:len(m.input)-size]
		}
		return m, nil

	case tea.KeyCtrlP:
		// Ctrl+P: open command palette from input zone
		return m.openCommandPalette()

	case tea.KeyCtrlF:
		// Ctrl+F: open model dialog from input zone
		return m.openModelDialog()

	case tea.KeyCtrlS:
		return m.openSessionPicker()

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
		// Layout: header(1) + sep(1) + viewport(vpHeight) + input(3) + footer(3)
		headerHeight := 1
		footerHeight := 3
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

var sectionOrder = []string{"mcp", "lsp", "todo", "files", "session"}

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
			m.panelRenderCmd = &comp

			// Extract tab IDs (from TS server side-panel component)
			if tabsRaw, ok := comp.Props["tabs"].([]interface{}); ok {
				var ids []string
				for _, t := range tabsRaw {
					if tab, ok := t.(map[string]interface{}); ok {
						if id, ok := tab["id"].(string); ok {
							ids = append(ids, id)
						}
					}
				}
				if len(ids) > 0 {
					m.rightTabIDs = ids
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
	// No side-panel from TS — that's fine, local sections still render.
	// Don't auto-hide the panel.
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
// Uses rune-aware comparison for correct UTF-8 handling.
func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}
	prefix := []rune(strs[0])
	for i := 1; i < len(strs); i++ {
		runes := []rune(strs[i])
		j := 0
		for j < len(prefix) && j < len(runes) && prefix[j] == runes[j] {
			j++
		}
		prefix = prefix[:j]
		if len(prefix) == 0 {
			return ""
		}
	}
	return string(prefix)
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
  /panel      Cycle panel mode: auto → show → hide
  /exit       Quit

Navigation (browse mode):
  j / ↓       Scroll down
  k / ↑       Scroll up
  g / G       Top / bottom
  Ctrl+D/U    Half page scroll
  Mouse wheel Scroll
  Tab         Cycle focus: main → panel → input

Right panel (when panel focused):
  h / ←       Previous section
  l / →       Next section
  Space       Expand/collapse current section
  j / ↓       Scroll panel down
  k / ↑       Scroll panel up

Dialogs:
  Ctrl+P      Command palette
  Ctrl+F      Model selection
  Ctrl+S      Session picker
  F1          Help
  Esc         Close dialog

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
		switch m.panelMode {
		case "hide":
			m.panelMode = "auto"
			m.invalidateCache()
			return "Right panel: auto mode (shows when wide, >100 cols)."
		case "show":
			m.panelMode = "hide"
			m.invalidateCache()
			return "Right panel hidden. /panel to show."
		default: // auto
			m.panelMode = "show"
			m.invalidateCache()
			return "Right panel: always visible. /panel to hide."
		}
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
	wideThreshold  = 100 // min terminal width for right panel auto-show
	inputBoxHeight = 3
	sepHeight      = 1
)

// panelVisible returns true if the right panel should be shown,
// based on terminal width and the panel mode.
func (m *Model) panelVisible(width int) bool {
	switch m.panelMode {
	case "hide":
		return false
	case "show":
		return true
	default: // "auto"
		return width >= wideThreshold
	}
}

// bodyHeight calculates the height available for the main content area.
func (m *Model) bodyHeight(height int) int {
	// header(1) + sep(1) + body + input(3) + statusLine(1) + sep(1) + footer(3)
	h := height - 1 - sepHeight - inputBoxHeight - 1 - sepHeight - 3
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
			// lipgloss.Width handles ANSI codes AND multi-byte characters correctly
			if rlw := lipgloss.Width(rightLine); rlw < panelW {
				rightLine += strings.Repeat(" ", panelW-rlw)
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
		if utf8.RuneCountInString(urlStr) > 40 {
			runes := []rune(urlStr)
			urlStr = string(runes[:37]) + "..."
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
	// Footer is a dedicated bottom panel: status row + context rows + legend row.
	// This matches the design doc's footer/context visualization intent.

	// Row 1: session / system status
	dir := m.currentDir
	if dir == "" {
		dir = "~"
	}
	dotColor := theme.Colors.Success
	if !m.serverConnected {
		dotColor = theme.Colors.Error
	}
	themeName := theme.ThemeDisplayName(theme.CurrentTheme)
	left := lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(dir)
	center := lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(
		fmt.Sprintf("MCP:%d  LSP:%d  P:%d", m.mcpCount, m.lspCount, m.permissionCount),
	)
	right := lipgloss.NewStyle().Foreground(dotColor).Render("●") + " " +
		lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(themeName)
	row1 := theme.FooterStyle.Render(m.footerThreeCols(width, left, center, right))

	// Row 2: multi-bar context visualization (role-based usage)
	row2 := m.renderContextUsagePanel(width)

	// Row 3: legend / totals
	row3 := m.renderContextLegend(width)

	return strings.Join([]string{row1, row2, row3}, "\n")
}

func (m Model) footerThreeCols(width int, left, center, right string) string {
	avail := width - 2
	if avail < 30 {
		avail = 30
	}
	leftW := avail * 26 / 100
	rightW := avail * 24 / 100
	centerW := avail - leftW - rightW
	if leftW < 10 {
		leftW = 10
	}
	if rightW < 10 {
		rightW = 10
	}
	if centerW < 10 {
		centerW = 10
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(leftW).MaxWidth(leftW).Render(m.truncStr(left, leftW)),
		lipgloss.NewStyle().Width(centerW).MaxWidth(centerW).Align(lipgloss.Center).Render(m.truncStr(center, centerW)),
		lipgloss.NewStyle().Width(rightW).MaxWidth(rightW).Align(lipgloss.Right).Render(m.truncStr(right, rightW)),
	)
}

func (m *Model) renderContextUsagePanel(width int) string {
	if m.session == nil {
		return theme.FooterStyle.Render("")
	}

	type stat struct {
		role  string
		name  string
		color lipgloss.Color
		count int
		chars int
	}

	stats := []stat{
		{role: RoleSystem, name: "system", color: theme.Colors.Primary},
		{role: RoleUser, name: "user", color: theme.Colors.Success},
		{role: RoleAssistant, name: "assistant", color: theme.Colors.Accent},
		{role: RoleTool, name: "tool", color: theme.Colors.Warning},
		{role: RoleError, name: "error", color: theme.Colors.Error},
	}
	idx := map[string]int{}
	for i := range stats {
		idx[stats[i].role] = i
	}
	for _, msg := range m.session.GetMessages() {
		if i, ok := idx[msg.Role]; ok {
			stats[i].count++
			stats[i].chars += len(msg.Content)
		}
	}
	total := 0
	for _, s := range stats {
		total += s.chars
	}
	if total == 0 {
		return theme.FooterStyle.Render(m.footerPanelLine(width, "Context", "empty", ""))
	}

	barWidth := width - 18
	if barWidth < 20 {
		barWidth = 20
	}
	var parts []string
	used := 0
	for i, s := range stats {
		if s.chars == 0 {
			continue
		}
		segW := s.chars * barWidth / total
		if segW < 1 {
			segW = 1
		}
		if i == len(stats)-1 {
			segW = barWidth - used
		}
		if segW < 1 {
			segW = 1
		}
		used += segW
		bar := lipgloss.NewStyle().Foreground(s.color).Render(strings.Repeat("█", segW))
		pct := (s.chars * 100) / total
		label := lipgloss.NewStyle().Foreground(s.color).Render(fmt.Sprintf(" %s %d%%", s.name, pct))
		parts = append(parts, bar+label)
	}
	line := strings.Join(parts, "  ")
	return theme.FooterStyle.Render(m.truncStr(line, width-2))
}

func (m *Model) renderContextLegend(width int) string {
	if m.session == nil {
		return theme.FooterStyle.Render("")
	}
	total := 0
	var sys, usr, asst, tool, other int
	for _, msg := range m.session.GetMessages() {
		n := len(msg.Content)
		total += n
		switch msg.Role {
		case RoleSystem:
			sys += n
		case RoleUser:
			usr += n
		case RoleAssistant:
			asst += n
		case RoleTool:
			tool += n
		default:
			other += n
		}
	}
	if total == 0 {
		return theme.FooterStyle.Render("")
	}
	msg := fmt.Sprintf("sys %d%% · usr %d%% · asst %d%% · tool %d%% · other %d%% · total %d",
		sys*100/total, usr*100/total, asst*100/total, tool*100/total, other*100/total, total)
	return theme.FooterStyle.Render(lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(m.truncStr(msg, width-2)))
}

func (m *Model) footerPanelLine(width int, left, center, right string) string {
	return m.footerThreeCols(width, left, center, right)
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

	// ── Tab bar at top (shows all sections, highlights active) ──
	sectionLines = append(sectionLines, m.renderPanelTabs(width-2))

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

	// ── Session section ──
	{
		expanded := m.panelSections["session"]
		header := m.renderSectionHeader("Session", expanded, "")
		sectionLines = append(sectionLines, header)
		if expanded {
			content := m.renderSessionTab(contentWidth)
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
	case "Session":
		return "session"
	}
	return ""
}

// renderPanelTabs renders the tab bar at the top of the right panel.
func (m *Model) renderPanelTabs(width int) string {
	if len(m.rightTabIDs) == 0 {
		return ""
	}

	tabNames := map[string]string{
		"lsp":     "LSP",
		"files":   "Files",
		"todo":    "TODO",
		"mcp":     "MCP",
		"session": "Session",
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
	case "session":
		return m.renderSessionTab(width)
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

// renderSessionTab shows session details: ID, messages, timing, context stats.
func (m *Model) renderSessionTab(width int) string {
	if m.session == nil {
		return lipgloss.NewStyle().
			Foreground(theme.Colors.TextDim).
			Render("No active session")
	}

	msg := m.session.GetMessages()
	var b strings.Builder

	// Session ID (truncated)
	sid := m.session.ID
	if utf8.RuneCountInString(sid) > 16 {
		runes := []rune(sid)
		sid = string(runes[:16]) + "…"
	}
	b.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Colors.Text).
		Render("Session"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).
		Render(sid))
	b.WriteString("\n\n")

	// CWD
	if m.session.CWD != "" {
		b.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Colors.Text).
			Render("Directory"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Foreground(theme.Colors.TextDim).
			Render(m.truncStr(m.session.CWD, width-4)))
		b.WriteString("\n\n")
	}

	// Message counts
	var userCt, asstCt, sysCt, toolCt, errCt int
	var totalChars int
	for _, m := range msg {
		totalChars += len(m.Content)
		switch m.Role {
		case RoleUser:
			userCt++
		case RoleAssistant:
			asstCt++
		case RoleSystem:
			sysCt++
		case RoleTool:
			toolCt++
		case RoleError:
			errCt++
		}
	}
	b.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Colors.Text).
		Render("Messages"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.Success).
		Render(fmt.Sprintf("  user:      %d", userCt)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.Accent).
		Render(fmt.Sprintf("  assistant: %d", asstCt)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.Primary).
		Render(fmt.Sprintf("  system:    %d", sysCt)))
	b.WriteString("\n")
	if toolCt > 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(theme.Colors.Warning).
			Render(fmt.Sprintf("  tool:      %d", toolCt)))
		b.WriteString("\n")
	}
	if errCt > 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(theme.Colors.Error).
			Render(fmt.Sprintf("  error:     %d", errCt)))
		b.WriteString("\n")
	}
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).
		Render(fmt.Sprintf("  total:     %d", len(msg))))
	b.WriteString("\n\n")

	// Context size
	b.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Colors.Text).
		Render("Context"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim).
		Render(fmt.Sprintf("  chars: %d", totalChars)))
	b.WriteString("\n")
	if totalChars > 0 {
		// Estimate tokens (rough: 4 chars per token)
		tokEst := totalChars / 4
		b.WriteString(lipgloss.NewStyle().
			Foreground(theme.Colors.TextDim).
			Render(fmt.Sprintf("  ~%dK tokens", tokEst/1000)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Last activity
	if m.session.path != "" {
		if fi, err := os.Stat(m.session.path); err == nil {
			b.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.Colors.Text).
				Render("Last saved"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().
				Foreground(theme.Colors.TextDim).
				Render(fi.ModTime().Format("15:04 Jan 02")))
			b.WriteString("\n")
		}
	}

	// Parent session (fork)
	if m.session.ParentSession != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Italic(true).
			Foreground(theme.Colors.TextDim).
			Render("Forked session"))
	}

	return b.String()
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
	ds := NewDialogSelect(DialogPalette, "Command Palette", items, 15,
		func(item DialogItem) tea.Cmd {
			return func() tea.Msg {
				return MsgDialogSelect{DlgType: DialogPalette, Value: item.Value}
			}
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
	ds := NewDialogSelect(DialogModel, "Select Model", items, 15,
		func(item DialogItem) tea.Cmd {
			return func() tea.Msg {
				return MsgDialogSelect{DlgType: DialogModel, Value: item.Value}
			}
		},
	)
	m.activeDialog = ds
	m.dialogOpen = true
	m.invalidateCache()
	return m, nil
}

// openHelpDialog opens the Help dialog (F1).
func (m Model) openHelpDialog() (tea.Model, tea.Cmd) {
	hdlg := NewHelpDialog()
	hdlg.height = m.bodyHeight(theme.TermHeight) // use most of the body height
	m.activeDialog = hdlg
	m.dialogOpen = true
	m.invalidateCache()
	return m, nil
}

// openSessionPicker opens a dialog for selecting an existing session (Ctrl+S).
// Always shows the picker when sessions exist, never auto-loads.
func (m Model) openSessionPicker() (tea.Model, tea.Cmd) {
	dir := m.currentDir
	if dir == "" {
		dir, _ = os.Getwd()
	}
	files, err := ListSessions()
	if err != nil || len(files) == 0 {
		// No sessions exist — create a new one
		m.session = NewSession(dir)
		m.invalidateCache()
		return m, nil
	}
	m.sessionPickerOpen = true
	m.activeDialog = newSessionPickerDialog(files, dir)
	m.dialogOpen = true
	m.invalidateCache()
	return m, nil
}

func loadRecentSessionOrPrompt(cwd string) (*Session, []string) {
	files, err := ListSessions()
	if err != nil || len(files) == 0 {
		return nil, files
	}
	latest, err := LoadSession(files[0])
	if err != nil {
		return nil, files
	}
	if latest.CWD != "" && cwd != "" && latest.CWD == cwd {
		return latest, nil
	}
	return nil, files
}

func newSessionPickerDialog(files []string, cwd string) Dialog {
	items := []DialogItem{{Title: "New Session", Description: "Start fresh", Category: "Actions", Value: "__new__"}}
	for _, path := range files {
		label := filepath.Base(path)
		desc := cwd
		if sess, err := LoadSession(path); err == nil {
			if sess.CWD != "" {
				desc = sess.CWD
			}
			if sess.ParentSession != "" {
				desc += " · forked"
			}
		}
		items = append(items, DialogItem{Title: label, Description: desc, Category: "History", Value: path})
	}
	return NewDialogSelect(DialogSession, "Session History", items, 14, func(item DialogItem) tea.Cmd {
		return func() tea.Msg {
			if item.Value == "__new__" {
				return MsgSessionSelect{New: true}
			}
			return MsgSessionSelect{Path: item.Value}
		}
	})
}

// ─── Interface guards ────────────────────────────────────────

var _ tea.Model = (*Model)(nil)
