package api

import (
	"testing"
)

// ─── ImmutablePrefix Tests ────────────────────────────────────

func TestNewImmutablePrefix(t *testing.T) {
	toolSpecs := []ToolDefinition{
		{Type: "function", Function: ToolFunction{Name: "read_file", Description: "Read a file"}},
	}

	prefix := NewImmutablePrefix("You are helpful.", toolSpecs)
	if prefix == nil {
		t.Fatal("NewImmutablePrefix() returned nil")
	}

	if prefix.GetSystemPrompt() != "You are helpful." {
		t.Errorf("GetSystemPrompt() = %q, want %q", prefix.GetSystemPrompt(), "You are helpful.")
	}

	if len(prefix.GetToolSpecs()) != 1 {
		t.Errorf("len(GetToolSpecs()) = %d, want 1", len(prefix.GetToolSpecs()))
	}

	if prefix.GetFingerprint() == "" {
		t.Error("GetFingerprint() should not be empty")
	}
}

func TestImmutablePrefixFingerprint(t *testing.T) {
	prefix1 := NewImmutablePrefix("You are helpful.", nil)
	prefix2 := NewImmutablePrefix("You are helpful.", nil)

	// Same content should produce same fingerprint
	if prefix1.GetFingerprint() != prefix2.GetFingerprint() {
		t.Error("Same content should produce same fingerprint")
	}

	// Different content should produce different fingerprint
	prefix3 := NewImmutablePrefix("You are a coder.", nil)
	if prefix1.GetFingerprint() == prefix3.GetFingerprint() {
		t.Error("Different content should produce different fingerprint")
	}
}

func TestImmutablePrefixVerifyFingerprint(t *testing.T) {
	prefix := NewImmutablePrefix("You are helpful.", nil)
	fingerprint := prefix.GetFingerprint()

	if !prefix.VerifyFingerprint(fingerprint) {
		t.Error("VerifyFingerprint() should return true for matching fingerprint")
	}

	if prefix.VerifyFingerprint("wrong-fingerprint") {
		t.Error("VerifyFingerprint() should return false for wrong fingerprint")
	}
}

func TestImmutablePrefixAddTool(t *testing.T) {
	prefix := NewImmutablePrefix("You are helpful.", nil)
	fingerprint1 := prefix.GetFingerprint()

	tool := ToolDefinition{
		Type:     "function",
		Function: ToolFunction{Name: "read_file", Description: "Read a file"},
	}
	prefix.AddTool(tool)

	// Fingerprint should change after adding tool
	if prefix.GetFingerprint() == fingerprint1 {
		t.Error("Fingerprint should change after adding tool")
	}

	if len(prefix.GetToolSpecs()) != 1 {
		t.Errorf("len(GetToolSpecs()) = %d, want 1", len(prefix.GetToolSpecs()))
	}
}

func TestImmutablePrefixRemoveTool(t *testing.T) {
	toolSpecs := []ToolDefinition{
		{Type: "function", Function: ToolFunction{Name: "read_file", Description: "Read a file"}},
		{Type: "function", Function: ToolFunction{Name: "write_file", Description: "Write a file"}},
	}
	prefix := NewImmutablePrefix("You are helpful.", toolSpecs)

	prefix.RemoveTool("read_file")

	if len(prefix.GetToolSpecs()) != 1 {
		t.Errorf("len(GetToolSpecs()) = %d, want 1", len(prefix.GetToolSpecs()))
	}

	if prefix.GetToolSpecs()[0].Function.Name != "write_file" {
		t.Errorf("Remaining tool should be write_file, got %s", prefix.GetToolSpecs()[0].Function.Name)
	}
}

func TestImmutablePrefixFewShots(t *testing.T) {
	prefix := NewImmutablePrefix("You are helpful.", nil)

	shots := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}
	prefix.SetFewShots(shots)

	if len(prefix.GetFewShots()) != 2 {
		t.Errorf("len(GetFewShots()) = %d, want 2", len(prefix.GetFewShots()))
	}
}

func TestImmutablePrefixToMessages(t *testing.T) {
	toolSpecs := []ToolDefinition{
		{Type: "function", Function: ToolFunction{Name: "read_file", Description: "Read a file"}},
	}
	prefix := NewImmutablePrefix("You are helpful.", toolSpecs)

	shots := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}
	prefix.SetFewShots(shots)

	msgs := prefix.ToMessages()

	// Should have: system + 2 few-shots = 3 messages
	if len(msgs) != 3 {
		t.Errorf("len(msgs) = %d, want 3", len(msgs))
	}

	if msgs[0].Role != "system" {
		t.Errorf("msgs[0].Role = %q, want %q", msgs[0].Role, "system")
	}

	if msgs[1].Role != "user" {
		t.Errorf("msgs[1].Role = %q, want %q", msgs[1].Role, "user")
	}
}

// ─── AppendOnlyLog Tests ──────────────────────────────────────

func TestNewAppendOnlyLog(t *testing.T) {
	log := NewAppendOnlyLog()
	if log == nil {
		t.Fatal("NewAppendOnlyLog() returned nil")
	}

	if log.Len() != 0 {
		t.Errorf("Len() = %d, want 0", log.Len())
	}
}

func TestAppendOnlyLogAppend(t *testing.T) {
	log := NewAppendOnlyLog()

	log.Append(Message{Role: "user", Content: "Hello"})
	log.Append(Message{Role: "assistant", Content: "Hi!"})

	if log.Len() != 2 {
		t.Errorf("Len() = %d, want 2", log.Len())
	}

	msgs := log.GetMessages()
	if msgs[0].Content != "Hello" {
		t.Errorf("msgs[0].Content = %q, want %q", msgs[0].Content, "Hello")
	}
	if msgs[1].Content != "Hi!" {
		t.Errorf("msgs[1].Content = %q, want %q", msgs[1].Content, "Hi!")
	}
}

func TestAppendOnlyLogExtend(t *testing.T) {
	log := NewAppendOnlyLog()

	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
	}
	log.Extend(msgs)

	if log.Len() != 2 {
		t.Errorf("Len() = %d, want 2", log.Len())
	}
}

func TestAppendOnlyLogGetMessagesSince(t *testing.T) {
	log := NewAppendOnlyLog()

	log.Append(Message{Role: "user", Content: "Hello"})
	log.Append(Message{Role: "assistant", Content: "Hi!"})
	log.Append(Message{Role: "user", Content: "How are you?"})

	msgs := log.GetMessagesSince(1)
	if len(msgs) != 2 {
		t.Errorf("len(msgs) = %d, want 2", len(msgs))
	}

	if msgs[0].Content != "Hi!" {
		t.Errorf("msgs[0].Content = %q, want %q", msgs[0].Content, "Hi!")
	}
}

func TestAppendOnlyLogCompactInPlace(t *testing.T) {
	log := NewAppendOnlyLog()

	log.Append(Message{Role: "user", Content: "Hello"})
	log.Append(Message{Role: "assistant", Content: "Hi!"})
	log.Append(Message{Role: "user", Content: "How are you?"})
	log.Append(Message{Role: "assistant", Content: "I'm good!"})

	// Compact first 2 messages into a summary
	summary := Message{Role: "system", Content: "[Previous conversation summarized]"}
	log.CompactInPlace(2, summary)

	if log.Len() != 3 {
		t.Errorf("Len() = %d, want 3", log.Len())
	}

	msgs := log.GetMessages()
	if msgs[0].Content != "[Previous conversation summarized]" {
		t.Errorf("msgs[0].Content = %q, want summary", msgs[0].Content)
	}
	if msgs[1].Content != "How are you?" {
		t.Errorf("msgs[1].Content = %q, want %q", msgs[1].Content, "How are you?")
	}
}

func TestAppendOnlyLogClear(t *testing.T) {
	log := NewAppendOnlyLog()

	log.Append(Message{Role: "user", Content: "Hello"})
	log.Append(Message{Role: "assistant", Content: "Hi!"})

	log.Clear()

	if log.Len() != 0 {
		t.Errorf("Len() = %d, want 0", log.Len())
	}
}

// ─── VolatileScratch Tests ────────────────────────────────────

func TestNewVolatileScratch(t *testing.T) {
	scratch := NewVolatileScratch()
	if scratch == nil {
		t.Fatal("NewVolatileScratch() returned nil")
	}

	if scratch.GetReasoning() != "" {
		t.Error("GetReasoning() should be empty initially")
	}

	if scratch.GetPlanState() != nil {
		t.Error("GetPlanState() should be nil initially")
	}
}

func TestVolatileScratchReasoning(t *testing.T) {
	scratch := NewVolatileScratch()

	scratch.SetReasoning("Let me think...")
	if scratch.GetReasoning() != "Let me think..." {
		t.Errorf("GetReasoning() = %q, want %q", scratch.GetReasoning(), "Let me think...")
	}

	scratch.Reset()
	if scratch.GetReasoning() != "" {
		t.Error("GetReasoning() should be empty after reset")
	}
}

func TestVolatileScratchPlanState(t *testing.T) {
	scratch := NewVolatileScratch()

	state := &PlanState{
		CurrentStep: 1,
		TotalSteps:  3,
		Steps: []PlanStep{
			{ID: "step1", Title: "Step 1", Status: "completed"},
			{ID: "step2", Title: "Step 2", Status: "in_progress"},
			{ID: "step3", Title: "Step 3", Status: "pending"},
		},
	}

	scratch.SetPlanState(state)
	if scratch.GetPlanState() == nil {
		t.Fatal("GetPlanState() should not be nil")
	}

	if scratch.GetPlanState().CurrentStep != 1 {
		t.Errorf("CurrentStep = %d, want 1", scratch.GetPlanState().CurrentStep)
	}

	scratch.Reset()
	if scratch.GetPlanState() != nil {
		t.Error("GetPlanState() should be nil after reset")
	}
}

func TestVolatileScratchNotes(t *testing.T) {
	scratch := NewVolatileScratch()

	scratch.AddNote("Note 1")
	scratch.AddNote("Note 2")

	notes := scratch.GetNotes()
	if len(notes) != 2 {
		t.Errorf("len(notes) = %d, want 2", len(notes))
	}

	if notes[0] != "Note 1" {
		t.Errorf("notes[0] = %q, want %q", notes[0], "Note 1")
	}

	scratch.Reset()
	if len(scratch.GetNotes()) != 0 {
		t.Error("GetNotes() should be empty after reset")
	}
}

// ─── CacheFirstContext Tests ──────────────────────────────────

func TestNewCacheFirstContext(t *testing.T) {
	toolSpecs := []ToolDefinition{
		{Type: "function", Function: ToolFunction{Name: "read_file", Description: "Read a file"}},
	}

	ctx := NewCacheFirstContext("You are helpful.", toolSpecs)
	if ctx == nil {
		t.Fatal("NewCacheFirstContext() returned nil")
	}

	if ctx.GetPrefix() == nil {
		t.Error("GetPrefix() should not be nil")
	}

	if ctx.GetLog() == nil {
		t.Error("GetLog() should not be nil")
	}

	if ctx.GetScratch() == nil {
		t.Error("GetScratch() should not be nil")
	}
}

func TestCacheFirstContextFingerprint(t *testing.T) {
	ctx := NewCacheFirstContext("You are helpful.", nil)
	fingerprint := ctx.GetPrefixFingerprint()

	if fingerprint == "" {
		t.Error("GetPrefixFingerprint() should not be empty")
	}

	if !ctx.VerifyFingerprint(fingerprint) {
		t.Error("VerifyFingerprint() should return true for matching fingerprint")
	}
}

func TestCacheFirstContextToMessages(t *testing.T) {
	ctx := NewCacheFirstContext("You are helpful.", nil)

	// Add messages to log
	ctx.AppendMessage(Message{Role: "user", Content: "Hello"})
	ctx.AppendMessage(Message{Role: "assistant", Content: "Hi!"})

	msgs := ctx.ToMessages()

	// Should have: system + 2 log messages = 3 messages
	if len(msgs) != 3 {
		t.Errorf("len(msgs) = %d, want 3", len(msgs))
	}

	if msgs[0].Role != "system" {
		t.Errorf("msgs[0].Role = %q, want %q", msgs[0].Role, "system")
	}

	if msgs[1].Content != "Hello" {
		t.Errorf("msgs[1].Content = %q, want %q", msgs[1].Content, "Hello")
	}
}

func TestCacheFirstContextStartNewTurn(t *testing.T) {
	ctx := NewCacheFirstContext("You are helpful.", nil)

	// Set some scratch data
	ctx.GetScratch().SetReasoning("Let me think...")
	ctx.GetScratch().AddNote("Note 1")

	// Start new turn should reset scratch
	ctx.StartNewTurn()

	if ctx.GetScratch().GetReasoning() != "" {
		t.Error("Reasoning should be empty after StartNewTurn()")
	}

	if len(ctx.GetScratch().GetNotes()) != 0 {
		t.Error("Notes should be empty after StartNewTurn()")
	}
}

func TestCacheFirstContextStats(t *testing.T) {
	ctx := NewCacheFirstContext("You are helpful.", nil)

	ctx.AppendMessage(Message{Role: "user", Content: "Hello"})
	ctx.AppendMessage(Message{Role: "assistant", Content: "Hi!"})

	stats := ctx.GetContextStats()

	if stats.PrefixMessages != 1 {
		t.Errorf("PrefixMessages = %d, want 1", stats.PrefixMessages)
	}

	if stats.LogMessages != 2 {
		t.Errorf("LogMessages = %d, want 2", stats.LogMessages)
	}

	if stats.TotalMessages != 3 {
		t.Errorf("TotalMessages = %d, want 3", stats.TotalMessages)
	}

	if stats.PrefixFingerprint == "" {
		t.Error("PrefixFingerprint should not be empty")
	}
}

func TestCacheFirstContextInvalidatePrefixCache(t *testing.T) {
	ctx := NewCacheFirstContext("You are helpful.", nil)

	// Get messages to populate cache
	ctx.ToMessages()

	// Invalidate cache
	ctx.InvalidatePrefixCache()

	// This should work without issues
	ctx.ToMessages()
}
