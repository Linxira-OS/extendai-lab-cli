package model

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// ─── Cost Tracking (CodeWhale-style) ─────────────────────────

// CostEstimate holds cost information for a request.
type CostEstimate struct {
	USD float64
	CNY float64
}

// IsPositive returns true if the cost is greater than zero.
func (c CostEstimate) IsPositive() bool {
	return c.USD > 0 || c.CNY > 0
}

// String returns a formatted cost string.
func (c CostEstimate) String() string {
	if c.USD > 0 {
		return fmt.Sprintf("$%.4f", c.USD)
	}
	if c.CNY > 0 {
		return fmt.Sprintf("¥%.4f", c.CNY)
	}
	return "$0.00"
}

// ─── Global cost accumulator (side-channel pattern) ──────────

var (
	pendingCostMu sync.Mutex
	pendingCost   CostEstimate
)

// ReportCost adds cost to the pending pool.
// Called by background LLM callers after each request.
func ReportCost(model string, inputTokens, outputTokens int) {
	cost := EstimateCost(model, inputTokens, outputTokens)
	if !cost.IsPositive() {
		return
	}

	pendingCostMu.Lock()
	defer pendingCostMu.Unlock()
	pendingCost.USD += cost.USD
	pendingCost.CNY += cost.CNY
}

// DrainCost returns the accumulated cost and resets the pool.
// Called by the TUI render loop on each frame.
func DrainCost() CostEstimate {
	pendingCostMu.Lock()
	defer pendingCostMu.Unlock()
	cost := pendingCost
	pendingCost = CostEstimate{}
	return cost
}

// ─── Cost estimation ─────────────────────────────────────────

// EstimateCost calculates cost based on model and token counts.
// Uses simplified pricing; real implementation would query a pricing table.
func EstimateCost(model string, inputTokens, outputTokens int) CostEstimate {
	// Default pricing (GPT-4o-like)
	inputPricePer1K := 0.005  // $0.005 per 1K input tokens
	outputPricePer1K := 0.015 // $0.015 per 1K output tokens

	// Adjust pricing based on model
	switch {
	case contains(model, "gpt-4o"):
		// GPT-4o pricing
	case contains(model, "gpt-4"):
		inputPricePer1K = 0.03
		outputPricePer1K = 0.06
	case contains(model, "gpt-3.5"):
		inputPricePer1K = 0.0015
		outputPricePer1K = 0.002
	case contains(model, "claude"):
		inputPricePer1K = 0.015
		outputPricePer1K = 0.075
	case contains(model, "deepseek"):
		inputPricePer1K = 0.001
		outputPricePer1K = 0.002
	case contains(model, "llama") || contains(model, "qwen"):
		// Local models: zero cost
		return CostEstimate{}
	}

	usd := float64(inputTokens)/1000.0*inputPricePer1K +
		float64(outputTokens)/1000.0*outputPricePer1K

	// Approximate CNY conversion (1 USD ≈ 7.2 CNY)
	cny := usd * 7.2

	return CostEstimate{USD: usd, CNY: cny}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ─── Session cost tracking ───────────────────────────────────

// SessionCost tracks cumulative cost for a session.
type SessionCost struct {
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCost         CostEstimate
	RequestCount      int
}

// Accrue adds a request's tokens to the session cost.
func (sc *SessionCost) Accrue(model string, inputTokens, outputTokens int) {
	sc.TotalInputTokens += inputTokens
	sc.TotalOutputTokens += outputTokens
	cost := EstimateCost(model, inputTokens, outputTokens)
	sc.TotalCost.USD += cost.USD
	sc.TotalCost.CNY += cost.CNY
	sc.RequestCount++
}

// DrainPending adds any pending background costs.
func (sc *SessionCost) DrainPending() {
	cost := DrainCost()
	sc.TotalCost.USD += cost.USD
	sc.TotalCost.CNY += cost.CNY
}

// ─── Render cost in footer ───────────────────────────────────

// RenderCostFooter renders cost information for the footer.
func RenderCostFooter(cost SessionCost) string {
	if cost.RequestCount == 0 {
		return ""
	}

	dimStyle := lipgloss.NewStyle().Foreground(theme.Colors.TextDim)
	return dimStyle.Render(cost.TotalCost.String())
}

// ─── Render cost in sidebar ──────────────────────────────────

// RenderCostSidebar renders detailed cost information for the sidebar.
func RenderCostSidebar(cost SessionCost, width int) string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Primary).
		Bold(true)
	b.WriteString(headerStyle.Render("💰 Cost"))
	b.WriteString("\n\n")

	// Total cost
	totalStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Success).
		Bold(true)
	b.WriteString(totalStyle.Render(cost.TotalCost.String()))
	b.WriteString("\n\n")

	// Token breakdown
	dimStyle := lipgloss.NewStyle().Foreground(theme.Colors.TextDim)
	b.WriteString(dimStyle.Render(fmt.Sprintf("Input:  %d tokens", cost.TotalInputTokens)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Output: %d tokens", cost.TotalOutputTokens)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Requests: %d", cost.RequestCount)))

	return b.String()
}
