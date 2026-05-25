package renderer

import (
	"fmt"
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
)

// RenderContext carries state for a single render pass.
type RenderContext struct {
	Width    int
	Theme    *theme.ThemeColors
	TabWidth int
}

// ComponentRenderer renders a single component to a string.
type ComponentRenderer func(c protocol.Component, ctx *RenderContext) string

// Registry maps component types to renderers.
type Registry struct {
	renderers map[string]ComponentRenderer
}

// NewRegistry creates the default registry with all built-in renderers.
func NewRegistry() *Registry {
	r := &Registry{renderers: make(map[string]ComponentRenderer)}
	r.registerBuiltins()
	return r
}

// Register adds a component renderer.
func (r *Registry) Register(typ string, fn ComponentRenderer) {
	r.renderers[typ] = fn
}

// Render dispatches a component to its registered renderer.
func (r *Registry) Render(c protocol.Component, ctx *RenderContext) string {
	fn, ok := r.renderers[c.Type]
	if !ok {
		return fmt.Sprintf("[unknown component: %s]", c.Type)
	}
	return fn(c, ctx)
}

// RenderAll renders a slice of components and joins them.
func (r *Registry) RenderAll(components []protocol.Component, ctx *RenderContext) string {
	var b strings.Builder
	for i, c := range components {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(r.Render(c, ctx))
	}
	return b.String()
}

func (r *Registry) registerBuiltins() {
	r.renderers[protocol.CompHeader] = renderHeader
	r.renderers[protocol.CompMessageList] = renderMessageList
	r.renderers[protocol.CompMessage] = renderMessage
	r.renderers[protocol.CompMarkdown] = renderMarkdown
	r.renderers[protocol.CompTable] = renderTable
	r.renderers[protocol.CompStatusBar] = renderStatusBar
	r.renderers[protocol.CompInput] = renderInput
	r.renderers[protocol.CompPanel] = renderPanel
	r.renderers[protocol.CompProgress] = renderProgress
	r.renderers[protocol.CompSpinner] = renderSpinner
	r.renderers[protocol.CompToolOutput] = renderToolOutput
	r.renderers[protocol.CompDiff] = renderDiff
	r.renderers[protocol.CompTree] = renderTree
}
