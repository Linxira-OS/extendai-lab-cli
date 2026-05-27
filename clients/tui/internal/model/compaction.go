package model

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// ─── Context Compaction (CodeWhale-style) ─────────────────────

// CompactionConfig holds compaction settings.
type CompactionConfig struct {
	Enabled       bool
	TokenThreshold int   // trigger when estimated tokens exceed this
	MinTokens     int   // don't compact below this (prefix cache economics)
	KeepRecent    int   // keep last N messages
	MaxInputChars int   // max chars to send to LLM for summarization
}

// DefaultCompactionConfig returns sensible defaults.
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		Enabled:       true,
		TokenThreshold: 800_000, // 80% of 1M context
		MinTokens:     500_000,  // hard floor
		KeepRecent:    4,
		MaxInputChars: 24_000,
	}
}

// CompactionPlan describes what to compact.
type CompactionPlan struct {
	SummarizeIndices []int // indices of messages to summarize
	KeepIndices      []int // indices of messages to keep
	TotalTokens      int
	EstimatedSavings int
}

// PlanCompaction decides which messages to summarize vs keep.
func (m *Model) PlanCompaction(config CompactionConfig) *CompactionPlan {
	if m.session == nil {
		return nil
	}

	usage := m.buildContextUsage()
	if usage.UsedTokens < config.MinTokens {
		return nil // below minimum threshold
	}
	if usage.UsedTokens < config.TokenThreshold && config.TokenThreshold > 0 {
		return nil // below trigger threshold
	}

	msgs := m.session.GetMessages()
	if len(msgs) <= config.KeepRecent {
		return nil // nothing to compact
	}

	// Keep the last N messages, summarize the rest
	splitIdx := len(msgs) - config.KeepRecent

	var summarizeIndices, keepIndices []int
	for i := range msgs {
		if i < splitIdx {
			summarizeIndices = append(summarizeIndices, i)
		} else {
			keepIndices = append(keepIndices, i)
		}
	}

	// Estimate savings
	summarizedTokens := 0
	for _, idx := range summarizeIndices {
		c := utf8.RuneCountInString(msgs[idx].Content)
		summarizedTokens += c / 4
	}

	return &CompactionPlan{
		SummarizeIndices: summarizeIndices,
		KeepIndices:      keepIndices,
		TotalTokens:      usage.UsedTokens,
		EstimatedSavings: summarizedTokens,
	}
}

// ─── Compaction relay template (CodeWhale compact.md style) ──

// CompactionRelay is the structured summary template.
type CompactionRelay struct {
	Goal        string
	Constraints string
	Progress    ProgressSection
	Decisions   string
	NextStep    string
}

type ProgressSection struct {
	Done      string
	InProgress string
	Blocked   string
}

// BuildRelayPrompt builds the LLM prompt for generating a compaction summary.
func (m *Model) BuildRelayPrompt(plan *CompactionPlan) string {
	if plan == nil || m.session == nil {
		return ""
	}

	msgs := m.session.GetMessages()

	var b strings.Builder
	b.WriteString("The conversation below needs to be compacted into a structured summary.\n")
	b.WriteString("Generate a relay following this exact format:\n\n")
	b.WriteString("## Compaction Relay\n\n")
	b.WriteString("The conversation above this point has been compacted. Below is a structured summary.\n\n")
	b.WriteString("### Goal\n")
	b.WriteString("[The user's high-level objective for this session]\n\n")
	b.WriteString("### Constraints\n")
	b.WriteString("[What's off-limits, what bounds the work]\n\n")
	b.WriteString("### Progress\n\n")
	b.WriteString("#### Done\n")
	b.WriteString("[What's complete and verified]\n\n")
	b.WriteString("#### In Progress\n")
	b.WriteString("[What's mid-flight]\n\n")
	b.WriteString("#### Blocked\n")
	b.WriteString("[What's stuck and why]\n\n")
	b.WriteString("### Key Decisions\n")
	b.WriteString("[Architectural choices and trade-offs]\n\n")
	b.WriteString("### Next Step\n")
	b.WriteString("[The single next action to take]\n\n")
	b.WriteString("---\n\n")
	b.WriteString("Conversation to summarize:\n\n")

	// Add messages to summarize (with size limit)
	charCount := 0
	maxChars := 24000
	for _, idx := range plan.SummarizeIndices {
		if idx >= len(msgs) {
			continue
		}
		msg := msgs[idx]
		entry := fmt.Sprintf("[%s] %s\n\n", msg.Role, msg.Content)
		if charCount+len(entry) > maxChars {
			b.WriteString("[... truncated ...]\n")
			break
		}
		b.WriteString(entry)
		charCount += len(entry)
	}

	return b.String()
}

// RenderCompactionRelay renders the relay for display in the TUI.
func RenderCompactionRelay(relay CompactionRelay, width int) string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Primary).
		Bold(true)
	sectionStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.Secondary).
		Bold(true)
	dimStyle := lipgloss.NewStyle().
		Foreground(theme.Colors.TextDim)

	b.WriteString(titleStyle.Render("## Compaction Relay"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Conversation compacted. Summary:"))
	b.WriteString("\n\n")

	// Goal
	b.WriteString(sectionStyle.Render("### Goal"))
	b.WriteString("\n")
	b.WriteString(relay.Goal)
	b.WriteString("\n\n")

	// Constraints
	b.WriteString(sectionStyle.Render("### Constraints"))
	b.WriteString("\n")
	b.WriteString(relay.Constraints)
	b.WriteString("\n\n")

	// Progress
	b.WriteString(sectionStyle.Render("### Progress"))
	b.WriteString("\n")
	if relay.Progress.Done != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.Success).Render("#### Done"))
		b.WriteString("\n")
		b.WriteString(relay.Progress.Done)
		b.WriteString("\n")
	}
	if relay.Progress.InProgress != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.Warning).Render("#### In Progress"))
		b.WriteString("\n")
		b.WriteString(relay.Progress.InProgress)
		b.WriteString("\n")
	}
	if relay.Progress.Blocked != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Colors.Error).Render("#### Blocked"))
		b.WriteString("\n")
		b.WriteString(relay.Progress.Blocked)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Decisions
	b.WriteString(sectionStyle.Render("### Key Decisions"))
	b.WriteString("\n")
	b.WriteString(relay.Decisions)
	b.WriteString("\n\n")

	// Next Step
	b.WriteString(sectionStyle.Render("### Next Step"))
	b.WriteString("\n")
	b.WriteString(relay.NextStep)

	return b.String()
}

// ─── Compaction status in footer ─────────────────────────────

// CompactionStatus represents the current compaction state.
type CompactionStatus int

const (
	CompactionIdle CompactionStatus = iota
	CompactionPlanning
	CompactionSummarizing
	CompactionReplacing
	CompactionDone
)

func (s CompactionStatus) String() string {
	switch s {
	case CompactionPlanning:
		return "planning"
	case CompactionSummarizing:
		return "summarizing"
	case CompactionReplacing:
		return "replacing"
	case CompactionDone:
		return "done"
	default:
		return ""
	}
}

// ShouldCompact checks if compaction should be triggered.
func (m *Model) ShouldCompact(config CompactionConfig) bool {
	if !config.Enabled {
		return false
	}
	plan := m.PlanCompaction(config)
	return plan != nil
}
