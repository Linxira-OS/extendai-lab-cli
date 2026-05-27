package model

import (
	"fmt"
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// ─── Multi-panel Sidebar (CodeWhale-style) ───────────────────

// SidebarPanel represents a sidebar panel type.
type SidebarPanel int

const (
	SidebarWork SidebarPanel = iota
	SidebarTasks
	SidebarAgents
	SidebarContext
)

func (p SidebarPanel) String() string {
	switch p {
	case SidebarWork:
		return "Work"
	case SidebarTasks:
		return "Tasks"
	case SidebarAgents:
		return "Agents"
	case SidebarContext:
		return "Context"
	default:
		return "Unknown"
	}
}

func (p SidebarPanel) Icon() string {
	switch p {
	case SidebarWork:
		return "📋"
	case SidebarTasks:
		return "✅"
	case SidebarAgents:
		return "🤖"
	case SidebarContext:
		return "📊"
	default:
		return "•"
	}
}

// SidebarLayout manages the multi-panel sidebar layout.
type SidebarLayout struct {
	Panels       []SidebarPanel
	ActivePanel  int
	AutoMode     bool // auto-collapse empty panels
	PanelStates  map[SidebarPanel]bool // expanded/collapsed
}

// NewSidebarLayout creates a new sidebar layout.
func NewSidebarLayout() *SidebarLayout {
	return &SidebarLayout{
		Panels: []SidebarPanel{
			SidebarWork,
			SidebarTasks,
			SidebarAgents,
			SidebarContext,
		},
		ActivePanel: 0,
		AutoMode:    true,
		PanelStates: map[SidebarPanel]bool{
			SidebarWork:    true,
			SidebarTasks:   true,
			SidebarAgents:  true,
			SidebarContext: true,
		},
	}
}

// CyclePanel moves to the next panel.
func (l *SidebarLayout) CyclePanel(direction int) {
	if len(l.Panels) == 0 {
		return
	}
	l.ActivePanel = (l.ActivePanel + direction + len(l.Panels)) % len(l.Panels)
}

// TogglePanel expands/collapses the current panel.
func (l *SidebarLayout) TogglePanel() {
	if l.ActivePanel < len(l.Panels) {
		panel := l.Panels[l.ActivePanel]
		l.PanelStates[panel] = !l.PanelStates[panel]
	}
}

// ─── Auto-layout logic (CodeWhale-style) ─────────────────────

// AutoSidebarState holds the state for auto-layout decisions.
type AutoSidebarState struct {
	WorkHasContent bool
	TasksEmpty     bool
	AgentsEmpty    bool
	ContextEnabled bool
}

// VisiblePanels returns which panels should be visible in auto mode.
func VisiblePanels(state AutoSidebarState) []SidebarPanel {
	nothingElseActive := state.TasksEmpty && state.AgentsEmpty && !state.ContextEnabled
	var visible []SidebarPanel

	if state.WorkHasContent || nothingElseActive {
		visible = append(visible, SidebarWork)
	}
	if !state.TasksEmpty {
		visible = append(visible, SidebarTasks)
	}
	if !state.AgentsEmpty {
		visible = append(visible, SidebarAgents)
	}
	if state.ContextEnabled {
		visible = append(visible, SidebarContext)
	}

	return visible
}

// ─── Render sidebar panels ───────────────────────────────────

// RenderSidebarPanel renders a single sidebar panel.
func (m *Model) RenderSidebarPanel(panel SidebarPanel, width int) string {
	switch panel {
	case SidebarWork:
		return m.renderWorkPanel(width)
	case SidebarTasks:
		return m.renderTasksPanel(width)
	case SidebarAgents:
		return m.renderAgentsPanel(width)
	case SidebarContext:
		return m.renderContextSidebar(width)
	default:
		return ""
	}
}

// renderWorkPanel renders the Work panel.
func (m *Model) renderWorkPanel(width int) string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Primary).
		Bold(true)
	b.WriteString(headerStyle.Render("📋 Work"))
	b.WriteString("\n")

	// Show current goal/objective
	if m.session != nil {
		msgs := m.session.GetMessages()
		if len(msgs) > 0 {
			// Show last user message as current goal
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Role == RoleUser {
					goalStyle := lipgloss.NewStyle().
						Foreground(theme.Colors.Text)
					b.WriteString(goalStyle.Render("Goal:"))
					b.WriteString("\n")

					// Truncate goal to fit
					goal := msgs[i].Content
					if len(goal) > width-4 {
						goal = goal[:width-7] + "..."
					}
					b.WriteString("  ")
					b.WriteString(goalStyle.Render(goal))
					b.WriteString("\n")
					break
				}
			}
		}
	}

	// Show message count
	if m.session != nil {
		count := len(m.session.GetMessages())
		dimStyle := lipgloss.NewStyle().Foreground(theme.Colors.TextDim)
		b.WriteString(dimStyle.Render(fmt.Sprintf("\n%d messages in session", count)))
	}

	return b.String()
}

// renderTasksPanel renders the Tasks panel.
func (m *Model) renderTasksPanel(width int) string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Primary).
		Bold(true)
	b.WriteString(headerStyle.Render("✅ Tasks"))
	b.WriteString("\n")

	// Show tasks from session
	if m.session != nil {
		msgs := m.session.GetMessages()
		taskCount := 0
		for _, msg := range msgs {
			if msg.Role == RoleAssistant && strings.Contains(strings.ToLower(msg.Content), "task") {
				taskCount++
			}
		}

		dimStyle := lipgloss.NewStyle().Foreground(theme.Colors.TextDim)
		if taskCount > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("%d tasks identified", taskCount)))
		} else {
			b.WriteString(dimStyle.Render("No tasks yet"))
		}
	}

	return b.String()
}

// renderAgentsPanel renders the Agents panel.
func (m *Model) renderAgentsPanel(width int) string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Primary).
		Bold(true)
	b.WriteString(headerStyle.Render("🤖 Agents"))
	b.WriteString("\n")

	// Show agent status
	dimStyle := lipgloss.NewStyle().Foreground(theme.Colors.TextDim)
	b.WriteString(dimStyle.Render("No active agents"))

	return b.String()
}

// ─── Render full sidebar with auto-layout ────────────────────

// RenderSidebar renders the full sidebar with auto-layout.
func (m *Model) RenderSidebar(width int) string {
	if width < 24 {
		return ""
	}

	var b strings.Builder

	// Tab bar
	tabBar := m.renderSidebarTabs(width)
	b.WriteString(tabBar)
	b.WriteString("\n")

	// Active panel content
	if m.sidebarLayout != nil && m.sidebarLayout.ActivePanel < len(m.sidebarLayout.Panels) {
		panel := m.sidebarLayout.Panels[m.sidebarLayout.ActivePanel]
		content := m.RenderSidebarPanel(panel, width-4) // padding
		b.WriteString(content)
	}

	return b.String()
}

// renderSidebarTabs renders the tab bar for the sidebar.
func (m *Model) renderSidebarTabs(width int) string {
	if m.sidebarLayout == nil {
		return ""
	}

	var tabs []string
	for i, panel := range m.sidebarLayout.Panels {
		isActive := i == m.sidebarLayout.ActivePanel

		tabStyle := lipgloss.NewStyle().
			Padding(0, 1)

		if isActive {
			tabStyle = tabStyle.
				Foreground(theme.Colors.Primary).
				Background(theme.Colors.Surface).
				Bold(true)
		} else {
			tabStyle = tabStyle.
				Foreground(theme.Colors.TextDim)
		}

		tabs = append(tabs, tabStyle.Render(panel.String()))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}
