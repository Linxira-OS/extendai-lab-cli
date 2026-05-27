package model

import (
	"sort"
)

// ─── Sidebar Plugin System ────────────────────────────────────
//
// Sidebar sections are registered globally via RegisterSidebarSection().
// Built-in sections register via init(); third-party plugins or extensions
// can call RegisterSidebarSection() at any time to add custom content
// to the right sidebar panel.
//
// Matching opencode's slot-based sidebar design: each section is a
// self-contained plugin with sortable ordering, a collapsible header,
// and a render function that returns formatted content.

// SidebarSection defines a collapsible section in the right sidebar.
type SidebarSection struct {
	// ID is the unique identifier (e.g., "context", "mcp", "lsp").
	ID string

	// Label is the display name shown in the section header.
	Label string

	// Order controls sort position (lower = nearer top).
	Order int

	// ExpandByDefault sets the initial expanded state.
	ExpandByDefault bool

	// Render returns the section content as a single string.
	// Called on each render cycle with the model and available content width.
	Render func(m *Model, width int) string
}

var (
	registeredSections []SidebarSection
)

// RegisterSidebarSection registers a sidebar section plugin.
// If a section with the same ID already exists, it is replaced (allowing
// plugins to override built-in sections). Sections are automatically
// sorted by Order after each registration.
func RegisterSidebarSection(s SidebarSection) {
	for i, existing := range registeredSections {
		if existing.ID == s.ID {
			registeredSections[i] = s
			return
		}
	}
	registeredSections = append(registeredSections, s)
	sort.Slice(registeredSections, func(i, j int) bool {
		return registeredSections[i].Order < registeredSections[j].Order
	})
}

// GetSidebarSections returns a sorted copy of registered sections.
func GetSidebarSections() []SidebarSection {
	result := make([]SidebarSection, len(registeredSections))
	copy(result, registeredSections)
	return result
}

// UnregisterSidebarSection removes a sidebar section by ID.
// Useful for tests or plugins that want to completely remove a section.
func UnregisterSidebarSection(id string) {
	for i, s := range registeredSections {
		if s.ID == id {
			registeredSections = append(registeredSections[:i], registeredSections[i+1:]...)
			return
		}
	}
}

// ─── Built-in section registrations ───────────────────────────

func init() {
	RegisterSidebarSection(SidebarSection{
		ID: "context", Label: "Context", Order: 50,
		ExpandByDefault: true,
		Render: func(m *Model, width int) string {
			return m.renderContextSidebar(width)
		},
	})

	RegisterSidebarSection(SidebarSection{
		ID: "mcp", Label: "MCP", Order: 100,
		ExpandByDefault: true,
		Render: func(m *Model, width int) string {
			return m.renderMCPTab(width)
		},
	})

	RegisterSidebarSection(SidebarSection{
		ID: "lsp", Label: "LSP", Order: 200,
		ExpandByDefault: true,
		Render: func(m *Model, width int) string {
			return m.renderLSPTab(width)
		},
	})

	RegisterSidebarSection(SidebarSection{
		ID: "todo", Label: "TODO", Order: 300,
		ExpandByDefault: false,
		Render: func(m *Model, width int) string {
			return m.renderTodoTab(width)
		},
	})

	RegisterSidebarSection(SidebarSection{
		ID: "files", Label: "Files", Order: 400,
		ExpandByDefault: false,
		Render: func(m *Model, width int) string {
			return m.renderFilesTab(width)
		},
	})

	RegisterSidebarSection(SidebarSection{
		ID: "session", Label: "Session", Order: 500,
		ExpandByDefault: true,
		Render: func(m *Model, width int) string {
			return m.renderSessionTab(width)
		},
	})
}
