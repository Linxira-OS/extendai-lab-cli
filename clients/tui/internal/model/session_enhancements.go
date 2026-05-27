package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── Session Enhancements ─────────────────────────────────────

// SessionMetadata holds additional session metadata.
type SessionMetadata struct {
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	MessageCount int      `json:"messageCount"`
	CostUSD     float64   `json:"costUsd,omitempty"`
}

// GetSessionMetadata extracts metadata from a session.
func GetSessionMetadata(s *Session) SessionMetadata {
	if s == nil {
		return SessionMetadata{}
	}

	msgs := s.GetMessages()
	title := "Untitled Session"
	if len(msgs) > 0 {
		// Use first user message as title
		for _, msg := range msgs {
			if msg.Role == RoleUser {
				title = msg.Content
				if len(title) > 50 {
					title = title[:50] + "..."
				}
				break
			}
		}
	}

	return SessionMetadata{
		Title:        title,
		CreatedAt:    time.Now(), // Would need to parse from session file
		UpdatedAt:    time.Now(),
		MessageCount: len(msgs),
	}
}

// ─── Session Search ──────────────────────────────────────────

// SearchSessions searches sessions by query string.
func SearchSessions(query string) ([]string, error) {
	files, err := ListSessions()
	if err != nil {
		return nil, err
	}

	if query == "" {
		return files, nil
	}

	query = strings.ToLower(query)
	var matches []string

	for _, path := range files {
		// Load session and check content
		s, err := LoadSession(path)
		if err != nil {
			continue
		}

		// Search in messages
		msgs := s.GetMessages()
		for _, msg := range msgs {
			if strings.Contains(strings.ToLower(msg.Content), query) {
				matches = append(matches, path)
				break
			}
		}
	}

	return matches, nil
}

// ─── Session Deletion ────────────────────────────────────────

// DeleteSession deletes a session file.
func DeleteSession(path string) error {
	return os.Remove(path)
}

// ─── Session Statistics ──────────────────────────────────────

// SessionStats holds statistics about a session.
type SessionStats struct {
	TotalMessages    int
	UserMessages     int
	AssistantMessages int
	SystemMessages   int
	ToolMessages     int
	ErrorMessages    int
	TotalChars       int
	AvgMessageLength int
	SessionDuration  time.Duration
}

// GetSessionStats computes statistics for a session.
func GetSessionStats(s *Session) SessionStats {
	if s == nil {
		return SessionStats{}
	}

	msgs := s.GetMessages()
	stats := SessionStats{
		TotalMessages: len(msgs),
	}

	for _, msg := range msgs {
		stats.TotalChars += len(msg.Content)

		switch msg.Role {
		case RoleUser:
			stats.UserMessages++
		case RoleAssistant:
			stats.AssistantMessages++
		case RoleSystem:
			stats.SystemMessages++
		case RoleTool:
			stats.ToolMessages++
		case RoleError:
			stats.ErrorMessages++
		}
	}

	if stats.TotalMessages > 0 {
		stats.AvgMessageLength = stats.TotalChars / stats.TotalMessages
	}

	return stats
}

// ─── Session Export ──────────────────────────────────────────

// ExportSession exports a session to a human-readable format.
func ExportSession(s *Session, format string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("session is nil")
	}

	msgs := s.GetMessages()

	switch format {
	case "markdown":
		return exportMarkdown(s, msgs), nil
	case "text":
		return exportText(s, msgs), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func exportMarkdown(s *Session, msgs []Message) string {
	var b strings.Builder

	b.WriteString("# Session Export\n\n")
	b.WriteString(fmt.Sprintf("**Session ID:** %s\n", s.ID))
	b.WriteString(fmt.Sprintf("**CWD:** %s\n", s.CWD))
	b.WriteString(fmt.Sprintf("**Messages:** %d\n\n", len(msgs)))
	b.WriteString("---\n\n")

	for _, msg := range msgs {
		switch msg.Role {
		case RoleUser:
			b.WriteString("## User\n\n")
		case RoleAssistant:
			b.WriteString("## Assistant\n\n")
		case RoleSystem:
			b.WriteString("## System\n\n")
		case RoleTool:
			b.WriteString(fmt.Sprintf("## Tool: %s\n\n", msg.ToolName))
		case RoleError:
			b.WriteString("## Error\n\n")
		}
		b.WriteString(msg.Content)
		b.WriteString("\n\n")
	}

	return b.String()
}

func exportText(s *Session, msgs []Message) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Session: %s\n", s.ID))
	b.WriteString(fmt.Sprintf("CWD: %s\n", s.CWD))
	b.WriteString(fmt.Sprintf("Messages: %d\n\n", len(msgs)))

	for _, msg := range msgs {
		b.WriteString(fmt.Sprintf("[%s] %s\n", msg.Role, msg.Content))
		b.WriteString("\n")
	}

	return b.String()
}

// ─── Session Cleanup ─────────────────────────────────────────

// CleanupSessions removes old session files.
func CleanupSessions(maxAge time.Duration) (int, error) {
	dir, err := SessionDir()
	if err != nil {
		return 0, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, e.Name())
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}
