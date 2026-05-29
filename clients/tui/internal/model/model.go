package model

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	Status           protocol.AIStatus // idle | thinking | streaming | error
	Label            string            // human-readable label
	Model            string            // model name
	Content          string            // accumulated streaming content
	ReasoningContent string            // accumulated reasoning/thinking content
	ErrorMsg         string            // last error message
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
	Content          string
	ReasoningContent string
	ToolCalls        []api.ToolCall
	Done             bool
	Error            string
	Usage            *api.UsageInfo
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
type MsgJobComplete struct {
	JobID     string
	Command   string
	ExitCode  int
	Output    string
	Error     string
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

	// Sidebar layout
	sidebarLayout *SidebarLayout

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
	panelSections      map[string]bool    // section name -> expanded (mcp/lsp/todo/files/context/session)
	panelHeaderAtLine  map[int]string     // unscrolled line index -> section ID for mouse-click detection

	// Background job manager
	jobManager *JobManager

	// IPC client (TS server)
	client *ipc.Client

	// Standalone API client (direct OpenAI-compatible, e.g. LM Studio)
	apiClient *api.Client

	// Tool registry for function calling
	toolRegistry *api.ToolRegistry

	// Context compaction configuration
	compactionConfig CompactionConfig

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
		prompt:           ">",
		ready:            true,
		viewport:         vp,
		client:           client,
		apiClient:        aiClient,
		panelMode:        "auto", // always auto-show when wide enough
		jobManager:       NewJobManager(),
		compactionConfig: DefaultCompactionConfig(),
	}

	m.panelHeaderAtLine = make(map[int]string)
	m.sidebarLayout = NewSidebarLayout()

	// Derive rightTabIDs and panelSections from registered sidebar sections.
	sections := GetSidebarSections()
	m.rightTabIDs = make([]string, len(sections))
	m.panelSections = make(map[string]bool, len(sections))
	for i, s := range sections {
		m.rightTabIDs[i] = s.ID
		m.panelSections[s.ID] = s.ExpandByDefault
	}
	if len(sections) > 0 {
		m.activeRightTab = sections[0].ID
	}

	// In standalone mode, set the model name from the API client
	if aiClient != nil {
		m.serverConnected = true
		m.modelName = aiClient.Model()
		m.modelURL = aiClient.BaseURL()
		// Initialize tool registry
		cwd, _ := os.Getwd()
		m.toolRegistry = api.NewToolRegistry(cwd)
		// Set job manager for background task support
		m.toolRegistry.SetJobManager(m.jobManager)
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
		// Sync API client history with session messages
		if aiClient != nil {
			syncAPIHistory(aiClient, session)
		}
	} else if len(files) > 0 {
		m.sessionPickerOpen = true
		m.activeDialog = newSessionPickerDialog(files, dir)
		m.dialogOpen = true
	} else {
		m.session = NewSession(dir)
	}

	return m
}

// syncAPIHistory syncs the API client's conversation history with the session's messages.
func syncAPIHistory(client *api.Client, session *Session) {
	msgs := session.GetMessages()
	if len(msgs) == 0 {
		return
	}

	// Convert session messages to API messages
	apiMsgs := []api.Message{
		{Role: "system", Content: "You are a helpful AI assistant."},
	}
	for _, msg := range msgs {
		role := msg.Role
		if role == "error" || role == "tool" {
			continue // skip error and tool messages
		}
		apiMsgs = append(apiMsgs, api.Message{
			Role:    role,
			Content: msg.Content,
		})
	}
	client.SetHistory(apiMsgs)
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.checkJobsCmd(),
	)
}

// checkJobsCmd returns a tea.Cmd that checks for completed jobs after a delay.
func (m Model) checkJobsCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return MsgCheckJobs{}
	})
}

type MsgCheckJobs struct{}

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

		// Track reasoning content separately
		if msg.ReasoningContent != "" {
			m.ai.ReasoningContent += msg.ReasoningContent
		}

		if msg.Content != "" {
			m.ai.Status = protocol.AIStreaming
			m.ai.Content += msg.Content
			m.reqTokensOut += len(msg.Content) / 4 // rough token estimate
		} else if msg.ReasoningContent != "" {
			// Only reasoning so far — stay in thinking state
			if m.ai.Status != protocol.AIStreaming {
				m.ai.Status = protocol.AIThinking
				m.ai.Label = "thinking"
			}
		}

		if msg.Done {
			// Check if model returned tool_calls (agentic loop)
			if len(msg.ToolCalls) > 0 && m.toolRegistry != nil {
				// Execute each tool and feed results back to model
				for _, tc := range msg.ToolCalls {
					// Parse arguments
					var args map[string]interface{}
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						args = make(map[string]interface{})
					}

					// Execute tool
					result, err := m.toolRegistry.Execute(tc.Function.Name, args)
					isError := err != nil
					if isError {
						result = err.Error()
					}

					// Add tool result to API client history
					m.apiClient.AddToolResult(tc.ID, result, isError)

					// Add tool message to session for display
					if m.session != nil {
						toolMsg := NewToolMessage(tc.Function.Name, result)
						m.session.AppendMessage(toolMsg)
					}
				}

				// Continue the agentic loop — send next request to model
				// Phase 1: Pre-API — check if context needs compaction
				if m.ShouldCompact(m.compactionConfig) {
					m.ai.Label = "compacting context..."
					m.invalidateCache()

					compacted := m.ExecuteCompaction(m.compactionConfig)
					if compacted {
						m.session.AppendMessage(NewSystemMessage("Context compacted to preserve token budget."))
					}
				}

				m.ai.Status = protocol.AIThinking
				m.ai.Label = "calling tools..."
				m.invalidateCache()

				var tools []api.ToolDefinition
				if m.toolRegistry != nil {
					tools = m.toolRegistry.GetDefinitions()
				}
				ch, err := m.apiClient.SendMessage(context.Background(), "", tools)
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
				return m, m.apiNextChunk()
			}

			// No tool_calls — normal completion
			m.ai.Status = protocol.AIIdle
			// Build final content: reasoning + content
			finalContent := m.ai.Content
			if m.ai.ReasoningContent != "" && m.ai.Content != "" {
				finalContent = m.ai.ReasoningContent + "\n\n---\n\n" + m.ai.Content
			} else if m.ai.ReasoningContent != "" {
				finalContent = m.ai.ReasoningContent
			}
			if finalContent != "" {
				// Compute timing
				dur := time.Duration(0)
				ttft := time.Duration(0)
				if !m.reqStart.IsZero() {
					dur = time.Since(m.reqStart)
				}
				if !m.reqFirstToken.IsZero() {
					ttft = m.reqFirstToken.Sub(m.reqStart)
				}

				// Use API usage if available, otherwise estimate
				tokensIn := 0
				tokensOut := m.reqTokensOut
				if msg.Usage != nil {
					tokensIn = msg.Usage.PromptTokens
					tokensOut = msg.Usage.CompletionTokens
				}

				asm := NewAssistantMessage(finalContent, dur, ttft, tokensIn, tokensOut, 0)
				if m.session != nil {
					m.session.AppendMessage(asm)
					m.session.Save()
				}
			}
			m.ai.Content = ""
			m.ai.ReasoningContent = ""
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
			// Reset API history for new session
			if m.apiClient != nil {
				m.apiClient.Reset()
			}
			return m, nil
		}
		loaded, err := LoadSession(msg.Path)
		if err != nil {
			m.ai.Status = protocol.AIError
			m.ai.ErrorMsg = err.Error()
			m.session = NewSession(m.currentDir)
		} else {
			m.session = loaded
			// Sync API history with loaded session
			if m.apiClient != nil {
				syncAPIHistory(m.apiClient, loaded)
			}
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

	case MsgJobComplete:
		// Inject job completion notification as a system message
		notification := fmt.Sprintf("Background job %s completed (exit code %d): %s",
			msg.JobID, msg.ExitCode, msg.Command)
		if msg.Error != "" {
			notification += "\nError: " + msg.Error
		}
		if msg.Output != "" {
			output := msg.Output
			if len(output) > 500 {
				output = output[:500] + "... (truncated)"
			}
			notification += "\nOutput:\n" + output
		}
		if m.session != nil {
			m.session.AppendMessage(NewSystemMessage(notification))
		}
		m.invalidateCache()
		m.viewport.GotoBottom()
		return m, m.checkJobsCmd() // continue checking

	case MsgCheckJobs:
		// Check for completed jobs and send notifications
		jobs := m.jobManager.ListJobs()
		for _, job := range jobs {
			if job.Status == JobCompleted || job.Status == JobFailed {
				// Check if we already notified (by checking if job is in a notified set)
				// For simplicity, we'll just send the notification
				// In a real implementation, we'd track which jobs have been notified
			}
		}
		return m, m.checkJobsCmd() // continue checking
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

	// Content width = mainW - margins
	m.viewport.Width = mainW - marginW*2
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
		// Layout: header(1) + body(vpHeight) + separator(1) + prompt(3) + contextBar(1)
		vpHeight := m.bodyHeight(theme.TermHeight)
		bodyStartY := headerHeight                // body starts after header
		sepY := bodyStartY + vpHeight             // separator
		promptStartY := sepY + 1                  // prompt starts after separator
		ctxBarY := promptStartY + promptAreaHeight // context bar

		// Click on prompt area
		if msg.Y >= promptStartY && msg.Y < ctxBarY {
			m.focusZone = FocusInput
			m.inputFocused = true
			return m, nil
		}

		// Click on right panel area (within body region)
		if m.panelVisible(theme.TermWidth) {
			actualPanelW := panelWidthMax
			if actualPanelW > theme.TermWidth-60 {
				actualPanelW = theme.TermWidth - 60
			}
			if actualPanelW < panelWidthMin {
				actualPanelW = panelWidthMin
			}
			panelLeft := theme.TermWidth - actualPanelW
			if msg.X >= panelLeft && msg.Y >= bodyStartY && msg.Y < bodyStartY+vpHeight {
				// visualLine = click Y - body start (0-based within panel)
				visualLine := msg.Y - bodyStartY
				unscrolledLine := visualLine + m.rightPanelScroll
				if sectionID, ok := m.panelHeaderAtLine[unscrolledLine]; ok {
					m.toggleSection(sectionID)
					m.activeRightTab = sectionID
					m.invalidateCache()
					return m, nil
				}
				m.focusZone = FocusPanel
				return m, nil
			}
		}

		// Click on main content area (within body region)
		if msg.Y >= bodyStartY && msg.Y < bodyStartY+vpHeight {
			m.focusZone = FocusMain
		}
		return m, nil
	}

	return m, nil
}

// ─── Section cycling ─────────────────────────────────────────

func (m *Model) toggleSection(id string) {
	if _, ok := m.panelSections[id]; ok {
		m.panelSections[id] = !m.panelSections[id]
		m.rightPanelScroll = 0
	}
}

func (m *Model) cycleSection(dir int) {
	sections := GetSidebarSections()
	if len(sections) == 0 {
		return
	}
	current := -1
	for i, s := range sections {
		if s.ID == m.activeRightTab {
			current = i
			break
		}
	}
	if current < 0 {
		m.activeRightTab = sections[0].ID
		return
	}
	next := (current + dir + len(sections)) % len(sections)
	m.activeRightTab = sections[next].ID
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

		// Phase 1: Pre-API — check if context needs compaction
		if m.ShouldCompact(m.compactionConfig) {
			m.ai.Status = protocol.AIThinking
			m.ai.Label = "compacting context..."
			m.invalidateCache()

			// Perform compaction (synchronous for now)
			compacted := m.ExecuteCompaction(m.compactionConfig)
			if compacted {
				m.session.AppendMessage(NewSystemMessage("Context compacted to preserve token budget."))
			}
		}

		m.ai.Status = protocol.AIThinking
		m.ai.Label = "thinking"
		m.reqStart = time.Now()
		m.reqFirstToken = time.Time{}
		m.reqTokensOut = 0
		m.invalidateCache()
		m.viewport.GotoBottom()

		// Start the streaming API call with tools
		var tools []api.ToolDefinition
		if m.toolRegistry != nil {
			tools = m.toolRegistry.GetDefinitions()
		}
		ch, err := m.apiClient.SendMessage(context.Background(), raw, tools)
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
		var tools []api.ToolDefinition
		if m.toolRegistry != nil {
			tools = m.toolRegistry.GetDefinitions()
		}
		ch, err := m.apiClient.SendMessage(context.Background(), userMsg, tools)
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
			return MsgStreamChunk{Done: true, Usage: evt.Usage, ToolCalls: evt.ToolCalls}
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
			return MsgStreamChunk{Done: true, Usage: evt.Usage, ToolCalls: evt.ToolCalls}
		}
		return MsgStreamChunk{
			Content:          evt.Content,
			ReasoningContent: evt.ReasoningContent,
		}
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
  /bg <cmd>   Run command in background
  /jobs       List background jobs
  /kill <id>  Cancel a background job
  /effort     Set thinking intensity: /effort (low|medium|high)
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
	case "debug":
		var hdrLines []string
		for y, id := range m.panelHeaderAtLine {
			hdrLines = append(hdrLines, fmt.Sprintf("  y=%d → %s", y, id))
		}
		return fmt.Sprintf("Panel debug:\n  rightPanelHeight=%d\n  rightPanelScroll=%d\n  panelMode=%s\n  focusZone=%d\n  panelSections=%v\n  panelHeaderAtLine(%d entries):\n%s",
			m.rightPanelHeight, m.rightPanelScroll, m.panelMode, m.focusZone, m.panelSections, len(m.panelHeaderAtLine), strings.Join(hdrLines, "\n"))
	case "jobs":
		return m.jobManager.FormatJobList()
	case "bg":
		if len(args) < 2 {
			return "Usage: /bg <command> [description]"
		}
		command := args[1]
		desc := ""
		if len(args) > 2 {
			desc = strings.Join(args[2:], " ")
		}
		jobID, err := m.jobManager.StartJob(command, desc)
		if err != nil {
			return "Error starting job: " + err.Error()
		}
		return fmt.Sprintf("Started background job %s: %s", jobID, command)
	case "kill":
		if len(args) < 2 {
			return "Usage: /kill <job_id>"
		}
		if m.jobManager.CancelJob(args[1]) {
			return "Cancelled job: " + args[1]
		}
		return "Job not found or not running: " + args[1]
	case "effort":
		if m.apiClient == nil {
			return "No API client configured."
		}
		if len(args) < 2 {
			current := m.apiClient.ReasoningEffort
			if current == "" {
				current = "medium (default)"
			}
			return "Current thinking effort: " + current + "\nUsage: /effort low|medium|high"
		}
		level := strings.ToLower(args[1])
		if level != "low" && level != "medium" && level != "high" {
			return "Invalid effort level. Use: low, medium, or high"
		}
		m.apiClient.ReasoningEffort = level
		return "Thinking effort set to: " + level
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
	wideThreshold  = 120 // min terminal width for right panel auto-show (matching opencode)
	marginW        = 2   // left/right content margin (matching opencode)
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
// Layout: header(1) + body + prompt(3) + contextBar(1)
func (m *Model) bodyHeight(height int) int {
	h := height - headerHeight - promptAreaHeight - contextBarHeight
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
		mainW = width - panelW - 1
	} else {
		panelW = 0
		mainW = width
	}

	contentW := mainW - marginW*2
	if contentW < 20 {
		contentW = 20
	}

	margin := strings.Repeat(" ", marginW)
	thin := lipgloss.NewStyle().Foreground(theme.Colors.Muted)
	var b strings.Builder

	// ── Header bar ──
	b.WriteString(m.renderHeaderBar(width))
	b.WriteString("\n")

	// ── Body: main content + optional sidebar ──
	if showPanel {
		mainRendered := m.renderMainContent(contentW)
		allMainLines := strings.Split(mainRendered, "\n")

		panelRendered := m.renderRightPanel(panelW)
		panelLines := strings.Split(panelRendered, "\n")

		// Apply viewport scroll offset to main content
		scrollOffset := m.viewport.YOffset
		if scrollOffset < 0 {
			scrollOffset = 0
		}
		if scrollOffset > len(allMainLines)-vpHeight {
			scrollOffset = len(allMainLines) - vpHeight
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}
		mainLines := allMainLines[scrollOffset:]
		if len(mainLines) > vpHeight {
			mainLines = mainLines[:vpHeight]
		}
		for len(mainLines) < vpHeight {
			mainLines = append(mainLines, "")
		}

		for len(panelLines) < vpHeight {
			panelLines = append(panelLines, "")
		}
		if len(panelLines) > vpHeight {
			panelLines = panelLines[:vpHeight]
		}

		divider := thin.Render("│")
		for i := 0; i < vpHeight; i++ {
			leftLine := mainLines[i]
			leftLine = truncateToWidth(leftLine, contentW)
			padded := lipgloss.NewStyle().Width(contentW).Render(leftLine)
			rightLine := panelLines[i]
			if rlw := lipgloss.Width(rightLine); rlw < panelW {
				rightLine += strings.Repeat(" ", panelW-rlw)
			}
			b.WriteString(margin)
			b.WriteString(padded)
			b.WriteString(divider)
			b.WriteString(rightLine)
			b.WriteString("\n")
		}
	} else {
		content := m.renderMainContent(contentW)
		allLines := strings.Split(content, "\n")

		// Apply viewport scroll offset
		scrollOffset := m.viewport.YOffset
		if scrollOffset < 0 {
			scrollOffset = 0
		}
		if scrollOffset > len(allLines)-vpHeight {
			scrollOffset = len(allLines) - vpHeight
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}
		contentLines := allLines[scrollOffset:]
		if len(contentLines) > vpHeight {
			contentLines = contentLines[:vpHeight]
		}
		for len(contentLines) < vpHeight {
			contentLines = append(contentLines, "")
		}
		for i := 0; i < vpHeight; i++ {
			line := truncateToWidth(contentLines[i], contentW)
			padded := lipgloss.NewStyle().Width(contentW).Render(line)
			b.WriteString(margin)
			b.WriteString(padded)
			b.WriteString("\n")
		}
	}

	// ── Separator ──
	b.WriteString(thin.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	// ── Prompt area ──
	b.WriteString(m.renderPromptArea(width))

	// ── Context usage bar ──
	b.WriteString("\n")
	b.WriteString(m.renderContextBar(width))

	finalView := replaceTabs(b.String())

	// ── Dialog overlay ──
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

// promptAreaHeight is the fixed height of the prompt+status area at the bottom.
// 1 (spacer) + 1 (input) + 1 (status row)
const promptAreaHeight = 3

// contextBarHeight is the fixed height of the context usage bar at the very bottom.
const contextBarHeight = 1

// headerHeight is the fixed height of the header bar at the top.
const headerHeight = 1

// ─── Render header (old, kept for reference) ────────────────

func (m Model) renderHeader(hasPanel bool) string {
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
	fillLen := avail - lipgloss.Width(leftText) - lipgloss.Width(rightText)
	if fillLen < 1 {
		fillLen = 1
	}
	fill := strings.Repeat(" ", fillLen)

	return theme.HeaderStyle.Render(leftText + fill + rightText)
}

// ─── Render footer ─────────────────────────────────────────
// Footer rendering is in footer.go (FooterProps pattern)

// ─── Render context sidebar ────────────────────────────────
// Context sidebar rendering is in context_inspector.go

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

	// ── Top spacer (1 line, matching opencode) ──
	contentLines = append(contentLines, "")

	// 1. Session messages
	if m.session != nil {
		msgs := m.session.GetMessages()
		for i, msg := range msgs {
			dimStyle := lipgloss.NewStyle()
			if msg.Brightness == BrightnessDim {
				dimStyle = dimStyle.Italic(true).Foreground(theme.Colors.TextDim)
			}

			switch msg.Role {
		case RoleUser:
			// ┃ User message (left border accent, matching opencode)
			borderChar := lipgloss.NewStyle().Foreground(theme.Colors.Primary).Render("┃")
			// Wrap long user messages
			mdWidth := width - 3
			if mdWidth < 20 {
				mdWidth = 20
			}
			wrappedContent := wrapText(msg.Content, mdWidth)
			for _, line := range strings.Split(wrappedContent, "\n") {
				contentLines = append(contentLines,
					borderChar+"  "+lipgloss.NewStyle().Foreground(theme.Colors.TextUser).Bold(true).Render(line))
			}

			case RoleAssistant:
				// Assistant text (3-char indent, matching opencode)
				mdWidth := width - 3
				if mdWidth < 20 {
					mdWidth = 20
				}
				rendered := renderer.RenderMarkdown(msg.Content, mdWidth)
				if msg.Brightness == BrightnessDim {
					rendered = dimStyle.Render(rendered)
				}
				contentLines = append(contentLines, "   "+rendered)

				// Timing footer
				widthFull := width - 3
				if widthFull > 80 {
					timing := msg.TimingFull()
					if timing != "" {
						contentLines = append(contentLines,
							lipgloss.NewStyle().
								Foreground(theme.Colors.TextDim).
								Italic(true).
								Render("   "+timing))
					}
				} else {
					timing := msg.TimingShort()
					if timing != "" {
						contentLines = append(contentLines,
							lipgloss.NewStyle().
								Foreground(theme.Colors.TextDim).
								Italic(true).
								Render("   "+timing))
					}
				}

			case RoleTool:
				// Tool card with ╭│╰ rail (CodeWhale style)
				familyGlyph := toolFamilyGlyph(msg.ToolName)
				toolColor := theme.Colors.Muted
				if msg.Brightness != BrightnessDim {
					toolColor = theme.Colors.Warning
				}
				// Card header: ╭ ▷ tool_name
				headerLine := lipgloss.NewStyle().Foreground(toolColor).Render(
					"╭ " + familyGlyph + " " + msg.ToolName)
				contentLines = append(contentLines, headerLine)
				// Card body: │ content
				mdWidth := width - 5
				if mdWidth < 20 {
					mdWidth = 20
				}
				rendered := renderer.RenderMarkdown(msg.Content, mdWidth)
				if msg.Brightness == BrightnessDim {
					rendered = dimStyle.Render(rendered)
				}
				for _, line := range strings.Split(rendered, "\n") {
					contentLines = append(contentLines,
						lipgloss.NewStyle().Foreground(toolColor).Render("│ ") + line)
				}
				// Card footer: ╰ summary
				contentLines = append(contentLines,
					lipgloss.NewStyle().Foreground(toolColor).Render("╰"))

			case RoleError:
				contentLines = append(contentLines,
					lipgloss.NewStyle().Foreground(theme.Colors.Error).Render("   ✗ "+msg.Content))

			case RoleSystem:
				contentLines = append(contentLines,
					lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Italic(true).Render("   "+msg.Content))
			}

			// Spacer between messages
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
			filteredCmd := &protocol.RenderCommand{Components: mainComps}
			tsContent := renderer.RenderAllComponents(filteredCmd, width)
			if tsContent != "" {
				contentLines = append(contentLines, "   "+tsContent)
			}
		}
	}

	// 3. Streaming content (AI response in progress)
	if m.ai.Status == protocol.AIStreaming || m.ai.Status == protocol.AIThinking {
		mdWidth := width - 3
		if mdWidth < 20 {
			mdWidth = 20
		}

		// Show reasoning/thinking content with dim styling
		if m.ai.ReasoningContent != "" {
			borderChar := lipgloss.NewStyle().Foreground(theme.Colors.Muted).Render("┃")
			contentLines = append(contentLines,
				borderChar+"  "+lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Italic(true).Render("Thinking"))
			reasoning := m.ai.ReasoningContent
			if len(reasoning) > 2000 {
				reasoning = "..." + reasoning[len(reasoning)-2000:]
			}
			reasoningRendered := renderer.RenderMarkdown(reasoning, mdWidth)
			dimStyle := lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Italic(true)
			contentLines = append(contentLines, "   "+dimStyle.Render(reasoningRendered))
		}

		// Show actual content
		if m.ai.Content != "" {
			rendered := renderer.RenderMarkdown(m.ai.Content, mdWidth)
			contentLines = append(contentLines, "   "+rendered)
		} else if m.ai.Status == protocol.AIThinking && m.ai.ReasoningContent == "" {
			contentLines = append(contentLines,
				lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render("   ⠋ thinking..."))
		}
	}

	// 4. Welcome message if nothing else
	if len(contentLines) <= 1 { // only the spacer
		contentLines = append(contentLines,
			lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Italic(true).
				Render("   Welcome to ExtendAI Lab. Type /help for commands."))
	}

	fullContent := strings.Join(contentLines, "\n")
	m.cachedMain = fullContent
	m.viewport.SetContent(fullContent)

	// Auto-scroll to bottom when AI is working (streaming/thinking)
	if m.ai.IsWorking() {
		totalLines := len(contentLines)
		if m.viewport.YOffset < totalLines-m.viewport.Height {
			m.viewport.YOffset = totalLines - m.viewport.Height
		}
		if m.viewport.YOffset < 0 {
			m.viewport.YOffset = 0
		}
	}

	return fullContent
}

// ─── Render prompt area (OpenCode style) ────────────────────
// Replaces the old header + input + separator + footer with a single
// prompt area at the bottom: spacer + input box + status row.

func (m Model) renderPromptArea(width int) string {
	margin := strings.Repeat(" ", marginW)
	var b strings.Builder

	// ── Spacer (1 line) ──
	b.WriteString("\n")

	// ── Input box with ┃ left border (matching opencode) ──
	promptSymbol := theme.PromptStyle.Render(m.prompt)
	displayText := m.input
	if m.input == "" && m.prompt == ">" {
		if m.inputFocused {
			displayText = ""
		} else {
			displayText = lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render("Type a message... (/help)")
		}
	}

	// Input line with left border accent
	inputColor := theme.Colors.Muted
	if m.focusZone == FocusInput {
		inputColor = theme.Colors.Primary
	}
	borderChar := lipgloss.NewStyle().Foreground(inputColor).Render("┃")
	inputLine := borderChar + "  " + promptSymbol + " " + displayText
	b.WriteString(margin)
	b.WriteString(lipgloss.NewStyle().Width(width - marginW*2).Render(inputLine))
	b.WriteString("\n")

	// ── Status row (model · context% · cost) ──
	statusParts := []string{}

	// AI working indicator
	if m.ai.IsWorking() {
		spinner := m.ai.SpinnerFrame()
		label := m.ai.Label
		if label == "" {
			label = string(m.ai.Status)
		}
		statusParts = append(statusParts,
			lipgloss.NewStyle().Foreground(theme.Colors.Accent).Render(spinner+" "+label))
	} else if m.ai.Status == protocol.AIError {
		statusParts = append(statusParts,
			lipgloss.NewStyle().Foreground(theme.Colors.Error).Render("✗ error"))
	}

	// Model name (right side)
	rightParts := []string{}
	if m.modelName != "" {
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(m.modelName))
	}

	// Focus zone indicator
	zoneLabel := ""
	switch m.focusZone {
	case FocusInput:
		zoneLabel = "input"
	case FocusPanel:
		zoneLabel = "panel"
	}
	if zoneLabel != "" {
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(zoneLabel))
	}

	// Build status row: left status | right info
	leftStatus := strings.Join(statusParts, "  ")
	rightInfo := strings.Join(rightParts, "  ")
	avail := width - marginW*2

	// Water-spout wave animation (CodeWhale style) when AI is working
	if m.ai.IsWorking() {
		waveWidth := avail - lipgloss.Width(leftStatus) - lipgloss.Width(rightInfo) - 4
		if waveWidth > 10 {
			wave := renderWaterSpout(waveWidth)
			statusRow := leftStatus + "  " + wave + "  " + rightInfo
			b.WriteString(margin)
			b.WriteString(lipgloss.NewStyle().Width(avail).Render(statusRow))
		} else {
			fillLen := avail - lipgloss.Width(leftStatus) - lipgloss.Width(rightInfo)
			if fillLen < 1 {
				fillLen = 1
			}
			statusRow := leftStatus + strings.Repeat(" ", fillLen) + rightInfo
			b.WriteString(margin)
			b.WriteString(lipgloss.NewStyle().Width(avail).Render(statusRow))
		}
	} else {
		fillLen := avail - lipgloss.Width(leftStatus) - lipgloss.Width(rightInfo)
		if fillLen < 1 {
			fillLen = 1
		}
		statusRow := leftStatus + strings.Repeat(" ", fillLen) + rightInfo
		b.WriteString(margin)
		b.WriteString(lipgloss.NewStyle().Width(avail).Render(statusRow))
	}

	return b.String()
}

// ─── Render header bar (CodeWhale style) ────────────────────
// Shows: ● ExtendAI Lab ─ session-id ─ [status]    model · ctx% · ● Live

func (m Model) renderHeaderBar(width int) string {
	thin := lipgloss.NewStyle().Foreground(theme.Colors.Muted)
	dim := lipgloss.NewStyle().Foreground(theme.Colors.TextDim)
	sep := thin.Render(" · ")

	// Left cluster: connection dot + title + session ID
	connColor := theme.Colors.Success
	connText := "●"
	if !m.serverConnected {
		connColor = theme.Colors.Error
		connText = "○"
	}
	left := lipgloss.NewStyle().Foreground(connColor).Render(connText) +
		" " + lipgloss.NewStyle().Bold(true).Render("ExtendAI Lab")

	if m.session != nil && m.session.ID != "" {
		sid := m.session.ID
		if utf8.RuneCountInString(sid) > 12 {
			runes := []rune(sid)
			sid = string(runes[:12]) + "…"
		}
		left += thin.Render(" ─ ") + dim.Render(sid)
	}

	// Right cluster: model · ctx% · status chips
	rightParts := []string{}

	// Model name chip
	if m.modelName != "" {
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(theme.Colors.TextDim).Render(m.modelName))
	}

	// Context usage chip (from session)
	if m.session != nil {
		totalChars := 0
		for _, msg := range m.session.GetMessages() {
			totalChars += utf8.RuneCountInString(msg.Content)
		}
		maxTokens := 128000
		if m.apiClient != nil && m.apiClient.Info() != nil && m.apiClient.Info().ContextLength > 0 {
			maxTokens = m.apiClient.Info().ContextLength
		}
		pct := float64(totalChars/4) / float64(maxTokens) * 100
		if pct > 100 {
			pct = 100
		}
		pctColor := theme.Colors.Success
		if pct >= 85 {
			pctColor = theme.Colors.Warning
		}
		if pct >= 95 {
			pctColor = theme.Colors.Error
		}
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(pctColor).Render(fmt.Sprintf("%.0f%%", pct)))
	}

	// Live indicator (when AI is working)
	if m.ai.IsWorking() {
		spinner := m.ai.SpinnerFrame()
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Bold(true).Render(
				spinner+" Live"))
	}

	right := strings.Join(rightParts, sep)

	// Layout: left | fill | right
	avail := width - marginW*2
	fillLen := avail - lipgloss.Width(left) - lipgloss.Width(right)
	if fillLen < 1 {
		fillLen = 1
	}

	margin := strings.Repeat(" ", marginW)
	return margin + lipgloss.NewStyle().Width(avail).Render(
		left+strings.Repeat(" ", fillLen)+right)
}

// ─── Render context usage bar ────────────────────────────────
// Shows token usage by category with different colors:
//   user 320 · asst 1.2K · tool 450 · sys 80   12% · 1.5K

func (m Model) renderContextBar(width int) string {
	if m.session == nil {
		return ""
	}

	margin := strings.Repeat(" ", marginW)
	msgs := m.session.GetMessages()

	// Count tokens by role
	var userTok, asstTok, toolTok, sysTok int
	for _, msg := range msgs {
		toks := utf8.RuneCountInString(msg.Content) / 4 // rough estimate
		switch msg.Role {
		case RoleUser:
			userTok += toks
		case RoleAssistant:
			asstTok += toks
		case RoleTool:
			toolTok += toks
		case RoleSystem, RoleError:
			sysTok += toks
		}
	}
	totalTok := userTok + asstTok + toolTok + sysTok

	// Max tokens
	maxTokens := 128000
	if m.apiClient != nil && m.apiClient.Info() != nil && m.apiClient.Info().ContextLength > 0 {
		maxTokens = m.apiClient.Info().ContextLength
	}
	pct := float64(totalTok) / float64(maxTokens) * 100
	if pct > 100 {
		pct = 100
	}

	// Build left side: colored token counts
	dim := lipgloss.NewStyle().Foreground(theme.Colors.TextDim)
	parts := []string{}
	if userTok > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.Colors.TextUser).Render(
			fmt.Sprintf("user %s", formatTokenK(userTok))))
	}
	if asstTok > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.Colors.Text).Render(
			fmt.Sprintf("asst %s", formatTokenK(asstTok))))
	}
	if toolTok > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.Colors.Warning).Render(
			fmt.Sprintf("tool %s", formatTokenK(toolTok))))
	}
	if sysTok > 0 {
		parts = append(parts, dim.Render(
			fmt.Sprintf("sys %s", formatTokenK(sysTok))))
	}
	left := strings.Join(parts, dim.Render(" · "))

	// Build right side: percentage + total
	pctColor := theme.Colors.Success
	if pct >= 85 {
		pctColor = theme.Colors.Warning
	}
	if pct >= 95 {
		pctColor = theme.Colors.Error
	}
	right := lipgloss.NewStyle().Foreground(pctColor).Render(
		fmt.Sprintf("%.0f%%", pct)) +
		dim.Render(" · ") +
		dim.Render(formatTokenK(totalTok))

	// Layout
	avail := width - marginW*2
	fillLen := avail - lipgloss.Width(left) - lipgloss.Width(right)
	if fillLen < 1 {
		fillLen = 1
	}
	row := left + strings.Repeat(" ", fillLen) + right
	return margin + lipgloss.NewStyle().Width(avail).Render(row)
}

// ─── Render right panel ─────────────────────────────────────

func (m *Model) renderRightPanel(width int) string {
	contentWidth := width - 4 // padding on both sides

	var sectionLines []string
	// IMPORTANT: Clear map in-place, do NOT reassign.
	// View() has a value receiver; reassigning changes only the copy's pointer.
	for k := range m.panelHeaderAtLine {
		delete(m.panelHeaderAtLine, k)
	}

	y := 0

	// ── Iterate over registered sidebar sections (no tab bar) ──
	for _, s := range GetSidebarSections() {
		expanded := m.panelSections[s.ID]
		header := m.renderSectionHeader(s.Label, expanded, "")
		m.panelHeaderAtLine[y] = s.ID
		sectionLines = append(sectionLines, header)
		y++
		if expanded {
			content := s.Render(m, contentWidth)
			for _, line := range strings.Split(content, "\n") {
				sectionLines = append(sectionLines, line)
				y++
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

	// Wrap in panel style (Padding(0,1) adds one space left + right)
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
	if summary != "" {
		dim := lipgloss.NewStyle().
			Foreground(theme.Colors.TextDim).
			Render(summary)
		return header + " " + dim
	}
	return header
}

// sectionIDByName maps section display names to internal IDs.
func (m *Model) sectionIDByName(name string) string {
	for _, s := range GetSidebarSections() {
		if s.Label == name {
			return s.ID
		}
	}
	return ""
}

// renderPanelTabs renders the tab bar at the top of the right panel.
func (m *Model) renderPanelTabs(width int) string {
	sections := GetSidebarSections()
	if len(sections) == 0 {
		return ""
	}

	var tabStrs []string
	for _, s := range sections {
		name := s.Label
		if s.ID == m.activeRightTab {
			tabStrs = append(tabStrs, theme.TabActiveStyle.Render(name))
		} else {
			tabStrs = append(tabStrs, theme.TabInactiveStyle.Render(name))
		}
	}
	return theme.TabBarStyle.Width(width).Render(strings.Join(tabStrs, ""))
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
// renderWaterSpout generates a CodeWhale-style wave animation.
// Uses ▁▂▃▄▅▆▇█ glyphs with a sine wave formula.
func renderWaterSpout(width int) string {
	if width < 1 {
		return ""
	}
	waveChars := []rune("▁▂▃▄▅▆▇█")
	t := float64(time.Now().UnixMilli()) / 1000.0 // time in seconds

	var b strings.Builder
	for i := 0; i < width; i++ {
		x := float64(i)
		// Sine wave formula (CodeWhale style)
		primary := 0.5 + 0.5*math.Sin(x*0.52-t*8.0)
		swell := 0.35 * math.Sin(x*0.18+t*3.1)
		shimmer := 0.12 * math.Sin(x*1.35-t*11.0)
		val := primary + swell + shimmer
		if val < 0 {
			val = 0
		}
		if val > 1 {
			val = 1
		}
		idx := int(val * float64(len(waveChars)-1))
		b.WriteRune(waveChars[idx])
	}
	return lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render(b.String())
}

// toolFamilyGlyph returns a CodeWhale-style family glyph for a tool name.
// ▷ read  ◆ patch  ▶ run  ⌕ find  ◐ delegate  ⋮⋮ fanout  • generic
func toolFamilyGlyph(toolName string) string {
	lower := strings.ToLower(toolName)
	switch {
	case strings.Contains(lower, "read") || strings.Contains(lower, "file_read"):
		return lipgloss.NewStyle().Foreground(theme.Colors.Primary).Render("▷")
	case strings.Contains(lower, "write") || strings.Contains(lower, "edit") || strings.Contains(lower, "patch"):
		return lipgloss.NewStyle().Foreground(theme.Colors.Accent).Render("◆")
	case strings.Contains(lower, "bash") || strings.Contains(lower, "shell") || strings.Contains(lower, "exec") || strings.Contains(lower, "run"):
		return lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render("▶")
	case strings.Contains(lower, "search") || strings.Contains(lower, "grep") || strings.Contains(lower, "glob") || strings.Contains(lower, "find"):
		return lipgloss.NewStyle().Foreground(theme.Colors.Warning).Render("⌕")
	case strings.Contains(lower, "agent") || strings.Contains(lower, "delegate") || strings.Contains(lower, "task"):
		return lipgloss.NewStyle().Foreground(theme.Colors.Secondary).Render("◐")
	default:
		return lipgloss.NewStyle().Foreground(theme.Colors.Muted).Render("•")
	}
}

func replaceTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

// truncateToWidth truncates a string to fit within maxW visual columns.
// Handles ANSI escape codes (zero visual width) and multi-byte characters.
// If the line fits, returns it unchanged. Otherwise truncates with "…".
func truncateToWidth(s string, maxW int) string {
	if maxW < 2 {
		return ""
	}
	// Fast path: if lipgloss.Width says it fits, return as-is
	if lipgloss.Width(s) <= maxW {
		return s
	}
	// Walk the string, tracking visual width.
	// Skip ANSI escape sequences (they have zero visual width).
	var result []rune
	vw := 0
	runes := []rune(s)
	inEsc := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		// Detect ANSI escape sequence: ESC[
		if r == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			inEsc = true
			result = append(result, r)
			continue
		}
		if inEsc {
			result = append(result, r)
			// End of escape: letter character terminates
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		// Normal character
		rw := lipgloss.Width(string(r))
		if vw+rw > maxW-1 {
			break
		}
		result = append(result, r)
		vw += rw
	}
	return string(result) + "…"
}

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

// ─── Text Wrapping Helper ─────────────────────────────────────

// wrapText wraps text to fit within the given width.
// Handles word boundaries and preserves existing newlines.
func wrapText(text string, width int) string {
	if width < 1 {
		return text
	}

	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		if lipgloss.Width(line) <= width {
			result = append(result, line)
			continue
		}

		// Wrap long lines
		wrapped := wrapLine(line, width)
		result = append(result, wrapped...)
	}

	return strings.Join(result, "\n")
}

// wrapLine wraps a single line to fit within the given width.
func wrapLine(line string, width int) []string {
	if width < 1 {
		return []string{line}
	}

	var result []string
	remaining := line

	for lipgloss.Width(remaining) > width {
		// Find a good break point
		cut := findBreakPointSimple(remaining, width)
		if cut <= 0 {
			cut = width
		}

		// Extract the line
		part := remaining[:cut]
		result = append(result, strings.TrimRight(part, " "))
		remaining = strings.TrimLeft(remaining[cut:], " ")
	}

	if remaining != "" {
		result = append(result, remaining)
	}

	return result
}

// findBreakPointSimple finds a good point to break a line, preferring word boundaries.
func findBreakPointSimple(s string, width int) int {
	visibleWidth := 0
	lastSpace := -1

	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])

		if r == ' ' {
			lastSpace = i
		}

		visibleWidth++
		if visibleWidth > width {
			if lastSpace > 0 {
				return lastSpace
			}
			return i
		}

		i += size
	}

	return len(s)
}
