package model

// ─── Session Tree ──────────────────────────────────────────────
//
// Architecture mirrors Pi / Oh My Pi:
//
//	Session = flat entry list with id/parentId forming a tree
//	LeafId tracks "current position" in the tree
//	branch() = move leaf pointer to an existing entry
//	fork() = copy entries to a new session file
//	getBranch() = walk parentId chain from leaf to root
//
// Reference: studying/pi/packages/agent/src/harness/session/session.ts
//            studying/oh-my-pi/packages/coding-agent/src/session/session-manager.ts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/xid"
)

// ─── Entry types ──────────────────────────────────────────────

const (
	EntryTypeMessage    = "message"
	EntryTypeLeaf       = "leaf"
	EntryTypeSession    = "session" // header entry
	EntryTypeBranchSum  = "branch_summary"
	EntryTypeModelChg   = "model_change"
	EntryTypeThinkChg   = "thinking_level_change"
)

// SessionEntryBase is the base for all session entries.
type SessionEntryBase struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	ParentID  string `json:"parentId"` // empty string = root
	Timestamp string `json:"timestamp"` // ISO 8601
}

// SessionEntry is a union-like entry in the session tree.
type SessionEntry struct {
	SessionEntryBase

	// MessageEntry fields
	Message *Message `json:"message,omitempty"`

	// LeafEntry fields: targetId = current leaf position
	TargetID string `json:"targetId,omitempty"`

	// BranchSummary fields
	Summary string `json:"summary,omitempty"`
	FromID  string `json:"fromId,omitempty"`

	// ModelChange / ThinkingChange fields
	Provider string `json:"provider,omitempty"`
	ModelID  string `json:"modelId,omitempty"`
	Level    string `json:"level,omitempty"`
}

// ─── Session ──────────────────────────────────────────────────

// Session represents a complete conversation session.
// It is backed by a JSONL file for persistence.
type Session struct {
	mu sync.RWMutex

	// File path
	path string

	// Header
	ID              string `json:"id"`
	Version         int    `json:"version"`
	CWD             string `json:"cwd"`
	ParentSession   string `json:"parentSession,omitempty"` // forked from

	// In-memory tree
	byID    map[string]*SessionEntry
	leafID  string // current position (empty = root)
	entries []*SessionEntry // insertion order

	// Dirty flag
	dirty bool
}

// NewSession creates a new session with the given working directory.
func NewSession(cwd string) *Session {
	id := xid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	s := &Session{
		path:    "",
		ID:      id,
		Version: 1,
		CWD:     cwd,
		byID:    make(map[string]*SessionEntry),
		dirty:   true,
	}

	// Create root leaf entry
	leafID := xid.New().String()
	leaf := &SessionEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      EntryTypeLeaf,
			ID:        leafID,
			ParentID:  "",
			Timestamp: now,
		},
		TargetID: "",
	}
	s.byID[leaf.ID] = leaf
	s.entries = append(s.entries, leaf)
	s.leafID = leafID

	return s
}

// ─── Message operations ──────────────────────────────────────

// AppendMessage appends a new message entry as a child of the current leaf.
// Returns the entry ID.
func (s *Session) AppendMessage(msg Message) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	id := xid.New().String()

	entry := &SessionEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      EntryTypeMessage,
			ID:        id,
			ParentID:  s.leafID,
			Timestamp: now,
		},
		Message: &msg,
	}

	s.byID[id] = entry
	s.entries = append(s.entries, entry)
	s.leafID = id
	s.updateLeafTarget(id)
	s.dirty = true

	return id
}

// GetMessages returns all messages from the active branch (leaf → root).
func (s *Session) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.getBranch(s.leafID)
	var msgs []Message
	for _, e := range entries {
		if e.Type == EntryTypeMessage && e.Message != nil {
			msgs = append(msgs, *e.Message)
		}
	}
	return msgs
}

// GetLastMessage returns the most recent message in the active branch.
func (s *Session) GetLastMessage() *Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.getBranch(s.leafID)
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Type == EntryTypeMessage && entries[i].Message != nil {
			return entries[i].Message
		}
	}
	return nil
}

// GetMessageCount returns the number of user+assistant messages in the active branch.
func (s *Session) GetMessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, e := range s.getBranch(s.leafID) {
		if e.Type == EntryTypeMessage && e.Message != nil &&
			(e.Message.Role == RoleUser || e.Message.Role == RoleAssistant) {
			count++
		}
	}
	return count
}

// ─── Branch operations ───────────────────────────────────────

// Branch moves the leaf pointer to a specific entry.
// Subsequent appends become children of that entry, forming a branch.
func (s *Session) Branch(entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.byID[entryID]; !ok {
		return fmt.Errorf("entry not found: %s", entryID)
	}

	s.leafID = entryID
	s.updateLeafTarget(entryID)
	s.dirty = true
	return nil
}

// BranchWithSummary moves the leaf pointer and appends a branch summary entry.
func (s *Session) BranchWithSummary(entryID, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.byID[entryID]; !ok {
		return fmt.Errorf("entry not found: %s", entryID)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	newID := xid.New().String()

	// Append branch summary
	summaryEntry := &SessionEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      EntryTypeBranchSum,
			ID:        newID,
			ParentID:  entryID,
			Timestamp: now,
		},
		Summary: summary,
		FromID:  s.leafID,
	}
	s.byID[newID] = summaryEntry
	s.entries = append(s.entries, summaryEntry)

	s.leafID = newID
	s.updateLeafTarget(newID)
	s.dirty = true
	return nil
}

// GetBranchIDs returns the entry IDs from leaf to root (for fork operations).
func (s *Session) GetBranchIDs(leafID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if leafID == "" {
		leafID = s.leafID
	}

	var ids []string
	current := leafID
	for current != "" {
		entry, ok := s.byID[current]
		if !ok {
			break
		}
		ids = append([]string{current}, ids...) // prepend
		current = entry.ParentID
	}
	return ids
}

// GetLeafID returns the current leaf position.
func (s *Session) GetLeafID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.leafID
}

// ─── Fork ─────────────────────────────────────────────────────

// Fork creates a new session by copying entries from the current branch.
// The new session references the original via ParentSession.
func (s *Session) Fork(cwd string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.GetBranchIDs(s.leafID)
	newS := NewSession(cwd)
	newS.ParentSession = s.ID

	for _, id := range ids {
		entry := s.byID[id]
		// Deep copy the entry
		clone := *entry
		// Generate new IDs for the clone
		newID := xid.New().String()
		clone.ID = newID
		if clone.Message != nil {
			m := *clone.Message
			clone.Message = &m
		}
		newS.byID[newID] = &clone
		newS.entries = append(newS.entries, &clone)

		// Link to previous in new chain
		if len(newS.entries) > 0 {
			prev := newS.entries[len(newS.entries)-1]
			clone.ParentID = prev.ID
		} else {
			clone.ParentID = ""
		}
	}

	// Update leaf to point to last entry
	if len(newS.entries) > 0 {
		last := newS.entries[len(newS.entries)-1]
		newS.leafID = last.ID
		newS.updateLeafTarget(last.ID)
	}

	newS.dirty = true
	return newS
}

// ─── Persistence (JSONL) ─────────────────────────────────────

// SessionDir returns the default session storage directory.
// Uses cross-platform paths (see paths.go for details).
func SessionDir() (string, error) {
	paths, err := GetPaths()
	if err != nil {
		return "", err
	}
	return paths.Sessions, nil
}

// Save persists the session to a JSONL file.
// Format: one JSON object per line, first line is the header.
func (s *Session) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.dirty && s.path != "" {
		return nil // nothing to save
	}

	dir, err := SessionDir()
	if err != nil {
		return err
	}

	if s.path == "" {
		cwdHash := sanitizePath(s.CWD)
		ts := time.Now().UTC().Format("20060102T150405Z")
		s.path = filepath.Join(dir, fmt.Sprintf("%s_%s_%s.jsonl", ts, cwdHash, s.ID))
	}

	f, err := os.Create(s.path)
	if err != nil {
		return fmt.Errorf("create session file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	// Write header
	header := map[string]interface{}{
		"type":    "session",
		"version": s.Version,
		"id":      s.ID,
		"cwd":     s.CWD,
	}
	if s.ParentSession != "" {
		header["parentSession"] = s.ParentSession
	}
	if err := enc.Encode(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Write entries
	for _, entry := range s.entries {
		if err := enc.Encode(entry); err != nil {
			return fmt.Errorf("write entry %s: %w", entry.ID, err)
		}
	}

	s.dirty = false
	return nil
}

// LoadSession loads a session from a JSONL file.
func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("empty session file: %s", path)
	}

	s := &Session{
		path:    path,
		byID:    make(map[string]*SessionEntry),
		dirty:   false,
	}

	// Parse header (first line)
	var header map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	s.ID, _ = header["id"].(string)
	if v, ok := header["version"].(float64); ok {
		s.Version = int(v)
	}
	s.CWD, _ = header["cwd"].(string)
	s.ParentSession, _ = header["parentSession"].(string)

	// Parse entries
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var entry SessionEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse entry %d: %w", i, err)
		}

		s.byID[entry.ID] = &entry
		s.entries = append(s.entries, &entry)

		// Track leaf entry - use TargetID if available, otherwise use entry ID
		if entry.Type == EntryTypeLeaf {
			if entry.TargetID != "" {
				s.leafID = entry.TargetID
			} else {
				s.leafID = entry.ID
			}
		}
	}

	return s, nil
}

// ListSessions lists all session files in the session directory,
// sorted by modification time (newest first).
func ListSessions() ([]string, error) {
	dir, err := SessionDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		fi, _ := os.Stat(files[i])
		fj, _ := os.Stat(files[j])
		if fi != nil && fj != nil {
			return fi.ModTime().After(fj.ModTime())
		}
		return false
	})

	return files, nil
}

// ─── Private helpers ─────────────────────────────────────────

// getBranch walks parentId chain from leafID to root.
// Returns entries in root→leaf order.
func (s *Session) getBranch(leafID string) []*SessionEntry {
	var result []*SessionEntry
	seen := make(map[string]bool)

	current := leafID
	// First pass: collect IDs from leaf to root
	var ids []string
	for current != "" {
		if seen[current] {
			break // cycle detection
		}
		seen[current] = true

		entry, ok := s.byID[current]
		if !ok {
			break
		}
		ids = append([]string{current}, ids...) // prepend = root first
		current = entry.ParentID
	}

	// Second pass: map to entries
	for _, id := range ids {
		if entry, ok := s.byID[id]; ok {
			result = append(result, entry)
		}
	}

	return result
}

// updateLeafTarget updates the leaf entry's TargetID to point to the given entry.
func (s *Session) updateLeafTarget(targetID string) {
	if leaf, ok := s.byID[s.leafID]; ok && leaf.Type == EntryTypeLeaf {
		leaf.TargetID = targetID
		s.dirty = true
	}
}

// sanitizePath replaces path separators to create a safe directory/file name.
func sanitizePath(p string) string {
	p = strings.ReplaceAll(p, ":", "_")
	p = strings.ReplaceAll(p, string(os.PathSeparator), "_")
	return p
}


