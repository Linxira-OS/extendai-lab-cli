package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// ─── Three-Region Context Model (from DeepSeek-Reasonix) ──────
//
// Architecture:
//   ImmutablePrefix (冻结)
//     - system prompt + tool specs + few-shots
//     - SHA-256 指纹验证
//     - session 内不变 → 缓存命中
//
//   AppendOnlyLog (仅追加)
//     - user messages, assistant messages, tool results
//     - 只追加不重写，保留前缀缓存
//
//   VolatileScratch (临时)
//     - reasoning, plan state, notes
//     - 每轮重置

// ─── ImmutablePrefix ──────────────────────────────────────────

// ImmutablePrefix holds the frozen prefix that never changes within a session.
// This maximizes provider prefix cache hits (DeepSeek, Anthropic).
type ImmutablePrefix struct {
	mu sync.RWMutex

	// System prompt
	systemPrompt string

	// Tool specifications (frozen after registration)
	toolSpecs []ToolDefinition

	// Few-shot examples (optional)
	fewShots []Message

	// SHA-256 fingerprint for cache validation
	fingerprint string

	// Dirty flag (should be false after first computation)
	dirty bool
}

// NewImmutablePrefix creates a new immutable prefix.
func NewImmutablePrefix(systemPrompt string, toolSpecs []ToolDefinition) *ImmutablePrefix {
	p := &ImmutablePrefix{
		systemPrompt: systemPrompt,
		toolSpecs:    toolSpecs,
		dirty:        true,
	}
	p.computeFingerprint()
	return p
}

// GetSystemPrompt returns the system prompt.
func (p *ImmutablePrefix) GetSystemPrompt() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.systemPrompt
}

// GetToolSpecs returns the tool specifications.
func (p *ImmutablePrefix) GetToolSpecs() []ToolDefinition {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.toolSpecs
}

// GetFewShots returns the few-shot examples.
func (p *ImmutablePrefix) GetFewShots() []Message {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.fewShots
}

// SetFewShots sets the few-shot examples (must be done before first use).
func (p *ImmutablePrefix) SetFewShots(shots []Message) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fewShots = shots
	p.dirty = true
	p.computeFingerprint()
}

// AddTool adds a tool to the prefix (allowed, doesn't break immutability).
func (p *ImmutablePrefix) AddTool(tool ToolDefinition) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.toolSpecs = append(p.toolSpecs, tool)
	p.dirty = true
	p.computeFingerprint()
}

// RemoveTool removes a tool by name (allowed, doesn't break immutability).
func (p *ImmutablePrefix) RemoveTool(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var newSpecs []ToolDefinition
	for _, t := range p.toolSpecs {
		if t.Function.Name != name {
			newSpecs = append(newSpecs, t)
		}
	}
	p.toolSpecs = newSpecs
	p.dirty = true
	p.computeFingerprint()
}

// ReplaceSystemPrompt replaces the system prompt (breaks immutability - use with caution).
func (p *ImmutablePrefix) ReplaceSystemPrompt(newPrompt string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.systemPrompt = newPrompt
	p.dirty = true
	p.computeFingerprint()
}

// GetFingerprint returns the SHA-256 fingerprint of the prefix.
func (p *ImmutablePrefix) GetFingerprint() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.fingerprint
}

// VerifyFingerprint checks if the current fingerprint matches the expected one.
// Returns true if the prefix hasn't been mutated unexpectedly.
func (p *ImmutablePrefix) VerifyFingerprint(expected string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.fingerprint == expected
}

// computeFingerprint computes the SHA-256 fingerprint of the prefix.
// Must be called with mu held.
func (p *ImmutablePrefix) computeFingerprint() {
	h := sha256.New()

	// Hash system prompt
	h.Write([]byte(p.systemPrompt))

	// Hash tool specs
	for _, tool := range p.toolSpecs {
		h.Write([]byte(tool.Function.Name))
		h.Write([]byte(tool.Function.Description))
	}

	// Hash few-shots
	for _, shot := range p.fewShots {
		h.Write([]byte(shot.Role))
		h.Write([]byte(shot.Content))
	}

	p.fingerprint = hex.EncodeToString(h.Sum(nil))
	p.dirty = false
}

// ToMessages converts the prefix to a slice of messages for API requests.
func (p *ImmutablePrefix) ToMessages() []Message {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var msgs []Message

	// System message
	if p.systemPrompt != "" {
		msgs = append(msgs, Message{
			Role:    "system",
			Content: p.systemPrompt,
		})
	}

	// Few-shot examples
	msgs = append(msgs, p.fewShots...)

	return msgs
}

// ─── AppendOnlyLog ────────────────────────────────────────────

// AppendOnlyLog holds messages that only grow monotonically.
// Prior turns are NEVER rewritten during normal operation.
// This preserves the prefix of prior turns for cache hits.
type AppendOnlyLog struct {
	mu sync.RWMutex

	// Messages in chronological order
	messages []Message

	// Compaction boundary (index of first message after last compaction)
	compactionBoundary int
}

// NewAppendOnlyLog creates a new append-only log.
func NewAppendOnlyLog() *AppendOnlyLog {
	return &AppendOnlyLog{
		messages:           make([]Message, 0),
		compactionBoundary: 0,
	}
}

// Append adds a message to the log.
func (l *AppendOnlyLog) Append(msg Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, msg)
}

// Extend adds multiple messages to the log.
func (l *AppendOnlyLog) Extend(msgs []Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, msgs...)
}

// GetMessages returns all messages from the log.
func (l *AppendOnlyLog) GetMessages() []Message {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.messages
}

// GetMessagesSince returns messages since the given index.
func (l *AppendOnlyLog) GetMessagesSince(index int) []Message {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if index < 0 || index >= len(l.messages) {
		return nil
	}
	return l.messages[index:]
}

// GetCompactionBoundary returns the compaction boundary index.
func (l *AppendOnlyLog) GetCompactionBoundary() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.compactionBoundary
}

// CompactInPlace replaces messages up to the boundary with a summary.
// This is the ONE exception to append-only semantics.
// Reserved exclusively for /compact, fold, and recovery.
func (l *AppendOnlyLog) CompactInPlace(boundary int, summary Message) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if boundary < 0 || boundary > len(l.messages) {
		return
	}

	// Replace messages up to boundary with summary
	newMessages := make([]Message, 0, len(l.messages)-boundary+1)
	newMessages = append(newMessages, summary)
	newMessages = append(newMessages, l.messages[boundary:]...)

	l.messages = newMessages
	l.compactionBoundary = 0 // Reset boundary after compaction
}

// Len returns the number of messages in the log.
func (l *AppendOnlyLog) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.messages)
}

// Clear removes all messages from the log.
func (l *AppendOnlyLog) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = l.messages[:0]
	l.compactionBoundary = 0
}

// ─── VolatileScratch ──────────────────────────────────────────

// VolatileScratch holds temporary data that resets each turn.
// This includes reasoning, plan state, and notes.
type VolatileScratch struct {
	mu sync.RWMutex

	// Reasoning content (R1 thought)
	Reasoning string

	// Plan state (if using plan system)
	PlanState *PlanState

	// Temporary notes
	Notes []string
}

// PlanState holds the current plan state.
type PlanState struct {
	CurrentStep int
	TotalSteps  int
	Steps       []PlanStep
}

// PlanStep holds a single plan step.
type PlanStep struct {
	ID          string
	Title       string
	Action      string
	Status      string // "pending", "in_progress", "completed", "skipped"
	Result      string
}

// NewVolatileScratch creates a new volatile scratch.
func NewVolatileScratch() *VolatileScratch {
	return &VolatileScratch{}
}

// Reset clears all volatile data (called at the start of each turn).
func (s *VolatileScratch) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Reasoning = ""
	s.PlanState = nil
	s.Notes = nil
}

// SetReasoning sets the reasoning content.
func (s *VolatileScratch) SetReasoning(reasoning string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Reasoning = reasoning
}

// GetReasoning returns the reasoning content.
func (s *VolatileScratch) GetReasoning() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Reasoning
}

// SetPlanState sets the plan state.
func (s *VolatileScratch) SetPlanState(state *PlanState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PlanState = state
}

// GetPlanState returns the plan state.
func (s *VolatileScratch) GetPlanState() *PlanState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PlanState
}

// AddNote adds a note to the scratch.
func (s *VolatileScratch) AddNote(note string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Notes = append(s.Notes, note)
}

// GetNotes returns all notes.
func (s *VolatileScratch) GetNotes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Notes
}

// ─── CacheFirstContext ────────────────────────────────────────

// CacheFirstContext combines the three regions into a single context manager.
type CacheFirstContext struct {
	prefix *ImmutablePrefix
	log    *AppendOnlyLog
	scratch *VolatileScratch

	// Cached prefix messages (computed once)
	cachedPrefixMsgs []Message
}

// NewCacheFirstContext creates a new cache-first context.
func NewCacheFirstContext(systemPrompt string, toolSpecs []ToolDefinition) *CacheFirstContext {
	prefix := NewImmutablePrefix(systemPrompt, toolSpecs)
	return &CacheFirstContext{
		prefix:  prefix,
		log:     NewAppendOnlyLog(),
		scratch: NewVolatileScratch(),
	}
}

// GetPrefix returns the immutable prefix.
func (c *CacheFirstContext) GetPrefix() *ImmutablePrefix {
	return c.prefix
}

// GetLog returns the append-only log.
func (c *CacheFirstContext) GetLog() *AppendOnlyLog {
	return c.log
}

// GetScratch returns the volatile scratch.
func (c *CacheFirstContext) GetScratch() *VolatileScratch {
	return c.scratch
}

// GetFingerprint returns the prefix fingerprint.
func (c *CacheFirstContext) GetFingerprint() string {
	return c.prefix.GetFingerprint()
}

// VerifyFingerprint checks if the prefix hasn't been mutated.
func (c *CacheFirstContext) VerifyFingerprint(expected string) bool {
	return c.prefix.VerifyFingerprint(expected)
}

// ToMessages converts the context to a slice of messages for API requests.
// Order: prefix messages + log messages (scratch is NOT included).
func (c *CacheFirstContext) ToMessages() []Message {
	// Get prefix messages (cached after first call)
	if c.cachedPrefixMsgs == nil {
		c.cachedPrefixMsgs = c.prefix.ToMessages()
	}

	// Get log messages
	logMsgs := c.log.GetMessages()

	// Combine: prefix + log
	allMsgs := make([]Message, 0, len(c.cachedPrefixMsgs)+len(logMsgs))
	allMsgs = append(allMsgs, c.cachedPrefixMsgs...)
	allMsgs = append(allMsgs, logMsgs...)

	return allMsgs
}

// StartNewTurn resets the volatile scratch (called at the start of each turn).
func (c *CacheFirstContext) StartNewTurn() {
	c.scratch.Reset()
}

// AppendMessage adds a message to the log.
func (c *CacheFirstContext) AppendMessage(msg Message) {
	c.log.Append(msg)
}

// AppendMessages adds multiple messages to the log.
func (c *CacheFirstContext) AppendMessages(msgs []Message) {
	c.log.Extend(msgs)
}

// GetPrefixFingerprint returns the prefix fingerprint for cache validation.
func (c *CacheFirstContext) GetPrefixFingerprint() string {
	return c.prefix.GetFingerprint()
}

// InvalidatePrefixCache invalidates the cached prefix messages.
// Call this after modifying the prefix (e.g., adding tools).
func (c *CacheFirstContext) InvalidatePrefixCache() {
	c.cachedPrefixMsgs = nil
}

// ─── Helper Functions ─────────────────────────────────────────

// estimateTokensInContext estimates the total tokens in a string.
// Uses CJK-aware estimation.
func estimateTokensInContext(text string) int {
	if text == "" {
		return 0
	}

	cjkChars := 0
	otherChars := 0

	for _, r := range text {
		if isCJKChar(r) {
			cjkChars++
		} else {
			otherChars++
		}
	}

	// CJK: ~0.67 tokens per char, Other: ~0.25 tokens per char
	cjkTokens := float64(cjkChars) * 0.67
	otherTokens := float64(otherChars) * 0.25

	return int(cjkTokens + otherTokens + 0.5)
}

// isCJKChar returns true if the rune is a CJK character.
func isCJKChar(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Hangul Syllables
}

// EstimateTokensFromContext estimates the total tokens in the context.
func (c *CacheFirstContext) EstimateTokensFromContext() int {
	msgs := c.ToMessages()
	total := 0
	for _, msg := range msgs {
		total += estimateTokensInContext(msg.Content)
	}
	return total
}

// GetContextStats returns statistics about the context.
func (c *CacheFirstContext) GetContextStats() ContextStats {
	msgs := c.ToMessages()
	logMsgs := c.log.GetMessages()

	stats := ContextStats{
		PrefixMessages: len(msgs) - len(logMsgs),
		LogMessages:    len(logMsgs),
		TotalMessages:  len(msgs),
		PrefixFingerprint: c.GetPrefixFingerprint(),
	}

	// Count tokens by region
	for i, msg := range msgs {
		tokens := estimateTokensInContext(msg.Content)
		if i < stats.PrefixMessages {
			stats.PrefixTokens += tokens
		} else {
			stats.LogTokens += tokens
		}
		stats.TotalTokens += tokens
	}

	return stats
}

// ContextStats holds statistics about the context.
type ContextStats struct {
	PrefixMessages    int
	LogMessages       int
	TotalMessages     int
	PrefixTokens      int
	LogTokens         int
	TotalTokens       int
	PrefixFingerprint string
}

// String returns a string representation of the context stats.
func (s ContextStats) String() string {
	return fmt.Sprintf("prefix=%d msgs (%d tok), log=%d msgs (%d tok), total=%d msgs (%d tok), fingerprint=%s",
		s.PrefixMessages, s.PrefixTokens,
		s.LogMessages, s.LogTokens,
		s.TotalMessages, s.TotalTokens,
		s.PrefixFingerprint[:8]+"...")
}
