package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── Memory System (Long-term + Short-term) ──────────────────

// Memory holds all persistent memory data.
type Memory struct {
	Version   int                `json:"version"`
	UserID    string             `json:"userId"`
	Global    GlobalMemory       `json:"global"`
	Projects  map[string]*ProjectMemory `json:"projects"`
}

// GlobalMemory holds user-level preferences and facts.
type GlobalMemory struct {
	Preferences Preferences     `json:"preferences"`
	Patterns    []Pattern       `json:"patterns"`
	Facts       []Fact          `json:"facts"`
}

// Preferences holds user preferences.
type Preferences struct {
	Theme        string `json:"theme,omitempty"`
	DefaultModel string `json:"defaultModel,omitempty"`
	Language     string `json:"language,omitempty"`
}

// Pattern holds a detected user pattern.
type Pattern struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Confidence  float64   `json:"confidence"`
	LastSeen    time.Time `json:"lastSeen"`
}

// Fact holds a remembered fact.
type Fact struct {
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

// ProjectMemory holds project-specific memory.
type ProjectMemory struct {
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Context   ProjectContext    `json:"context"`
	Decisions []Decision        `json:"decisions"`
}

// ProjectContext holds project context.
type ProjectContext struct {
	Description string   `json:"description,omitempty"`
	TechStack   []string `json:"techStack,omitempty"`
	KeyFiles    []string `json:"keyFiles,omitempty"`
}

// Decision holds a recorded decision.
type Decision struct {
	Topic     string    `json:"topic"`
	Decision  string    `json:"decision"`
	Timestamp time.Time `json:"timestamp"`
}

// ─── Memory Manager ──────────────────────────────────────────

// MemoryManager manages memory operations.
type MemoryManager struct {
	path    string
	memory  *Memory
	dirty   bool
}

// NewMemoryManager creates a new memory manager.
// Uses cross-platform paths (see paths.go for details).
func NewMemoryManager() (*MemoryManager, error) {
	paths, err := GetPaths()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(paths.Memory, "global.json")
	mm := &MemoryManager{path: path}

	// Load existing memory or create new
	if err := mm.load(); err != nil {
		if os.IsNotExist(err) {
			mm.memory = &Memory{
				Version:  1,
				Projects: make(map[string]*ProjectMemory),
			}
		} else {
			return nil, err
		}
	}

	return mm, nil
}

// load reads memory from disk.
func (mm *MemoryManager) load() error {
	data, err := os.ReadFile(mm.path)
	if err != nil {
		return err
	}

	var mem Memory
	if err := json.Unmarshal(data, &mem); err != nil {
		return fmt.Errorf("parse memory: %w", err)
	}

	if mem.Projects == nil {
		mem.Projects = make(map[string]*ProjectMemory)
	}

	mm.memory = &mem
	return nil
}

// Save writes memory to disk.
func (mm *MemoryManager) Save() error {
	if !mm.dirty {
		return nil
	}

	data, err := json.MarshalIndent(mm.memory, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memory: %w", err)
	}

	if err := os.WriteFile(mm.path, data, 0600); err != nil {
		return fmt.Errorf("write memory: %w", err)
	}

	mm.dirty = false
	return nil
}

// ─── Global Memory Operations ────────────────────────────────

// SetPreference sets a user preference.
func (mm *MemoryManager) SetPreference(key, value string) {
	switch key {
	case "theme":
		mm.memory.Global.Preferences.Theme = value
	case "defaultModel":
		mm.memory.Global.Preferences.DefaultModel = value
	case "language":
		mm.memory.Global.Preferences.Language = value
	}
	mm.dirty = true
}

// GetPreference gets a user preference.
func (mm *MemoryManager) GetPreference(key string) string {
	switch key {
	case "theme":
		return mm.memory.Global.Preferences.Theme
	case "defaultModel":
		return mm.memory.Global.Preferences.DefaultModel
	case "language":
		return mm.memory.Global.Preferences.Language
	}
	return ""
}

// AddFact adds a fact to global memory.
func (mm *MemoryManager) AddFact(content, source string) {
	fact := Fact{
		Content:   content,
		Source:    source,
		Timestamp: time.Now(),
	}
	mm.memory.Global.Facts = append(mm.memory.Global.Facts, fact)
	mm.dirty = true
}

// GetFacts returns all facts matching a query.
func (mm *MemoryManager) GetFacts(query string) []Fact {
	var results []Fact
	query = strings.ToLower(query)

	for _, fact := range mm.memory.Global.Facts {
		if strings.Contains(strings.ToLower(fact.Content), query) {
			results = append(results, fact)
		}
	}

	return results
}

// ─── Project Memory Operations ───────────────────────────────

// GetProjectMemory gets or creates project memory.
func (mm *MemoryManager) GetProjectMemory(projectPath string) *ProjectMemory {
	hash := hashPath(projectPath)

	if proj, ok := mm.memory.Projects[hash]; ok {
		return proj
	}

	proj := &ProjectMemory{
		Name: filepath.Base(projectPath),
		Path: projectPath,
	}
	mm.memory.Projects[hash] = proj
	mm.dirty = true
	return proj
}

// AddDecision records a project decision.
func (mm *MemoryManager) AddDecision(projectPath, topic, decision string) {
	proj := mm.GetProjectMemory(projectPath)
	proj.Decisions = append(proj.Decisions, Decision{
		Topic:     topic,
		Decision:  decision,
		Timestamp: time.Now(),
	})
	mm.dirty = true
}

// GetDecisions returns decisions for a project.
func (mm *MemoryManager) GetDecisions(projectPath string) []Decision {
	proj := mm.GetProjectMemory(projectPath)
	return proj.Decisions
}

// ─── Memory Extraction from Messages ─────────────────────────

// ExtractMemory analyzes messages and extracts important information.
func (mm *MemoryManager) ExtractMemory(msgs []Message, projectPath string) {
	for _, msg := range msgs {
		if msg.Role != RoleUser {
			continue
		}

		content := strings.ToLower(msg.Content)

		// Extract preferences
		if strings.Contains(content, "我喜欢") || strings.Contains(content, "i prefer") {
			mm.AddFact(msg.Content, "preference")
		}
		if strings.Contains(content, "记住") || strings.Contains(content, "remember") {
			mm.AddFact(msg.Content, "instruction")
		}

		// Extract project context
		if strings.Contains(content, "这个项目") || strings.Contains(content, "this project") {
			proj := mm.GetProjectMemory(projectPath)
			if proj.Context.Description == "" {
				proj.Context.Description = msg.Content
				mm.dirty = true
			}
		}
	}
}

// ─── Helper Functions ────────────────────────────────────────

// hashPath creates a simple hash of a path for use as a key.
func hashPath(path string) string {
	// Normalize path
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.ToLower(path)

	// Simple hash (in production, use a proper hash)
	var hash int
	for _, c := range path {
		hash = hash*31 + int(c)
	}
	return fmt.Sprintf("%x", hash)
}

// ─── Memory Cleanup ──────────────────────────────────────────

// CleanupOldFacts removes facts older than maxAge.
func (mm *MemoryManager) CleanupOldFacts(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	removed := 0

	var newFacts []Fact
	for _, fact := range mm.memory.Global.Facts {
		if fact.Timestamp.After(cutoff) {
			newFacts = append(newFacts, fact)
		} else {
			removed++
		}
	}

	if removed > 0 {
		mm.memory.Global.Facts = newFacts
		mm.dirty = true
	}

	return removed
}
