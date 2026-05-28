package model

// ─── Message types ─────────────────────────────────────────────
//
// These types mirror Pi / Oh My Pi's session tree message model:
//   - MessageEntry carries the actual AgentMessage
//   - Branching = moving leaf pointer
//   - Forking = copying entries to a new file
//   - getBranch() walks parentId chain from leaf → root
//
// Reference: studying/pi/packages/agent/src/harness/types.ts
//            studying/oh-my-pi/packages/coding-agent/src/session/session-manager.ts

import (
	"fmt"
	"strings"
	"time"
)

// ─── Role constants ────────────────────────────────────────────

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
	RoleSystem    = "system"
	RoleError     = "error"
)

// ─── Display brightness ────────────────────────────────────────

type Brightness int

const (
	BrightnessNormal Brightness = 100 // normal text
	BrightnessDim    Brightness = 80  // tool calls, thinking
	BrightnessDimmed Brightness = 60  // background overlay when dialog active
)

// ─── Message ──────────────────────────────────────────────────

// Message represents a single message in the conversation.
//
// Field mapping from Pi's AgentMessage:
//
//	AssistantMessage.usage        → TokensIn/TokensOut/CacheRead
//	AssistantMessage.duration     → Duration  (ms)
//	AssistantMessage.ttft         → TTFT      (ms)
//	AssistantMessage.timestamp    → Timestamp (Unix ms in Pi; we use time.Time)
//	UserMessage.timestamp         → Timestamp
//	ToolResultMessage.toolName    → ToolName
//	ToolResultMessage.toolCallId  → ToolCallID
type Message struct {
	Role      string    // "user" | "assistant" | "tool" | "system" | "error"
	Content   string    // message body (plain text or rendered markdown)
	Timestamp time.Time // when the message completed

	// Assistant-specific timing metadata (set after API response completes)
	Duration    time.Duration // total generation wall-clock time
	TTFt        time.Duration // time to first token
	TokensIn    int           // input tokens (from usage.input)
	TokensOut   int           // output tokens (from usage.output)
	CacheRead   int           // cache hit tokens (from usage.cacheRead)
	TokensPerSec float64     // computed: tokensOut / active_generation_seconds

	// Cost tracking
	CostInput  float64 // input cost in USD
	CostOutput float64 // output cost in USD

	// Tool message fields
	ToolCallID string // correlates tool result to tool call
	ToolName   string // tool name (e.g. "search_web", "calculator")

	// Error
	IsError bool // true if this is an error message

	// Display brightness (for dialog overlay dimming)
	Brightness Brightness
}

// NewUserMessage creates a user message with the current timestamp.
func NewUserMessage(content string) Message {
	return Message{
		Role:       RoleUser,
		Content:    content,
		Timestamp:  time.Now(),
		Brightness: BrightnessNormal,
	}
}

// NewAssistantMessage creates an assistant message with timing metadata.
func NewAssistantMessage(content string, duration, ttft time.Duration, tokensIn, tokensOut, cacheRead int) Message {
	tps := 0.0
	// Speed = total output tokens / total duration (not just active time)
	if duration > 0 && tokensOut > 0 {
		tps = float64(tokensOut) / duration.Seconds()
	}
	return Message{
		Role:         RoleAssistant,
		Content:      content,
		Timestamp:    time.Now(),
		Duration:     duration,
		TTFt:         ttft,
		TokensIn:     tokensIn,
		TokensOut:    tokensOut,
		CacheRead:    cacheRead,
		TokensPerSec: tps,
		Brightness:   BrightnessNormal,
	}
}

// NewToolMessage creates a tool call/result message.
func NewToolMessage(toolName, content string) Message {
	return Message{
		Role:       RoleTool,
		Content:    content,
		Timestamp:  time.Now(),
		ToolName:   toolName,
		Brightness: BrightnessDim, // dimmer than normal text
	}
}

// NewSystemMessage creates a system/info message.
func NewSystemMessage(content string) Message {
	return Message{
		Role:       RoleSystem,
		Content:    content,
		Timestamp:  time.Now(),
		Brightness: BrightnessNormal,
	}
}

// NewErrorMessage creates an error message.
func NewErrorMessage(err string) Message {
	return Message{
		Role:       RoleError,
		Content:    err,
		Timestamp:  time.Now(),
		IsError:    true,
		Brightness: BrightnessNormal,
	}
}

// TimingShort returns a compact one-line summary of generation timing.
// Example: "(3.2s · 156 tok · 48.7 t/s · TTFT 0.8s)"
func (m Message) TimingShort() string {
	if m.Role != RoleAssistant || m.Duration == 0 {
		return ""
	}

	ttft := m.TTFt.Round(time.Millisecond).String()
	dur := m.Duration.Round(time.Millisecond).String()

	if m.TokensPerSec > 0 {
		return fmt.Sprintf("(%s · %d tok · %.1f t/s · TTFT %s)",
			dur, m.TokensOut, m.TokensPerSec, ttft)
	}
	return fmt.Sprintf("(%s · %d tok · TTFT %s)", dur, m.TokensOut, ttft)
}

// TimingFull returns a detailed timing summary.
// Example: "用时 3.2s · 输入 156 tok · 输出 156 tok · 速度 48.7 t/s · TTFT 0.8s · 完成 14:30:28"
func (m Message) TimingFull() string {
	if m.Role != RoleAssistant || m.Duration == 0 {
		return ""
	}

	ttft := m.TTFt.Round(time.Millisecond).String()
	dur := m.Duration.Round(time.Millisecond).String()
	completion := m.Timestamp.Format("15:04:05")

	var parts []string
	parts = append(parts, "用时 "+dur)
	parts = append(parts, fmt.Sprintf("输入 %d tok", m.TokensIn))
	parts = append(parts, fmt.Sprintf("输出 %d tok", m.TokensOut))
	if m.TokensPerSec > 0 {
		parts = append(parts, fmt.Sprintf("速度 %.1f t/s", m.TokensPerSec))
	}
	parts = append(parts, "TTFT "+ttft)
	parts = append(parts, "完成 "+completion)

	return strings.Join(parts, " · ")
}

// TotalTokens returns the total token count for this message.
func (m Message) TotalTokens() int {
	return m.TokensIn + m.TokensOut + m.CacheRead
}
