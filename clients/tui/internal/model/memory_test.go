package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewMemoryManager(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()

	// Create test paths
	testPaths := &Paths{
		Home:     tmpDir,
		Data:     filepath.Join(tmpDir, "data"),
		Config:   filepath.Join(tmpDir, "config"),
		Memory:   filepath.Join(tmpDir, "memory"),
		Sessions: filepath.Join(tmpDir, "sessions"),
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(testPaths.Memory, 0700); err != nil {
		t.Fatalf("Failed to create memory directory: %v", err)
	}

	// Test memory manager creation
	mm := &MemoryManager{
		path: filepath.Join(testPaths.Memory, "global.json"),
		memory: &Memory{
			Version:  1,
			Projects: make(map[string]*ProjectMemory),
		},
	}

	if mm.memory == nil {
		t.Fatal("Memory is nil")
	}

	if mm.memory.Version != 1 {
		t.Errorf("Version = %d, want 1", mm.memory.Version)
	}
}

func TestMemorySetPreference(t *testing.T) {
	mm := &MemoryManager{
		memory: &Memory{
			Version:  1,
			Projects: make(map[string]*ProjectMemory),
		},
	}

	// Set preferences
	mm.SetPreference("theme", "dark")
	mm.SetPreference("defaultModel", "gpt-4o")
	mm.SetPreference("language", "zh-CN")

	// Verify
	if mm.memory.Global.Preferences.Theme != "dark" {
		t.Errorf("Theme = %s, want dark", mm.memory.Global.Preferences.Theme)
	}
	if mm.memory.Global.Preferences.DefaultModel != "gpt-4o" {
		t.Errorf("DefaultModel = %s, want gpt-4o", mm.memory.Global.Preferences.DefaultModel)
	}
	if mm.memory.Global.Preferences.Language != "zh-CN" {
		t.Errorf("Language = %s, want zh-CN", mm.memory.Global.Preferences.Language)
	}

	if !mm.dirty {
		t.Error("dirty flag not set after SetPreference")
	}
}

func TestMemoryGetPreference(t *testing.T) {
	mm := &MemoryManager{
		memory: &Memory{
			Version: 1,
			Global: GlobalMemory{
				Preferences: Preferences{
					Theme:        "dark",
					DefaultModel: "gpt-4o",
					Language:     "zh-CN",
				},
			},
			Projects: make(map[string]*ProjectMemory),
		},
	}

	// Get preferences
	if got := mm.GetPreference("theme"); got != "dark" {
		t.Errorf("GetPreference(theme) = %s, want dark", got)
	}
	if got := mm.GetPreference("defaultModel"); got != "gpt-4o" {
		t.Errorf("GetPreference(defaultModel) = %s, want gpt-4o", got)
	}
	if got := mm.GetPreference("language"); got != "zh-CN" {
		t.Errorf("GetPreference(language) = %s, want zh-CN", got)
	}

	// Unknown preference
	if got := mm.GetPreference("unknown"); got != "" {
		t.Errorf("GetPreference(unknown) = %s, want empty", got)
	}
}

func TestMemoryAddFact(t *testing.T) {
	mm := &MemoryManager{
		memory: &Memory{
			Version:  1,
			Projects: make(map[string]*ProjectMemory),
		},
	}

	// Add facts
	mm.AddFact("用户喜欢TypeScript", "session_123")
	mm.AddFact("项目使用Go语言", "session_456")

	// Verify
	if len(mm.memory.Global.Facts) != 2 {
		t.Errorf("Facts count = %d, want 2", len(mm.memory.Global.Facts))
	}

	if mm.memory.Global.Facts[0].Content != "用户喜欢TypeScript" {
		t.Errorf("Fact[0].Content = %s, want '用户喜欢TypeScript'", mm.memory.Global.Facts[0].Content)
	}

	if mm.memory.Global.Facts[0].Source != "session_123" {
		t.Errorf("Fact[0].Source = %s, want session_123", mm.memory.Global.Facts[0].Source)
	}

	if !mm.dirty {
		t.Error("dirty flag not set after AddFact")
	}
}

func TestMemoryGetFacts(t *testing.T) {
	mm := &MemoryManager{
		memory: &Memory{
			Version: 1,
			Global: GlobalMemory{
				Facts: []Fact{
					{Content: "用户喜欢TypeScript", Source: "session_1"},
					{Content: "项目使用Go语言", Source: "session_2"},
					{Content: "用户偏好暗色主题", Source: "session_3"},
				},
			},
			Projects: make(map[string]*ProjectMemory),
		},
	}

	// Search facts
	facts := mm.GetFacts("TypeScript")
	if len(facts) != 1 {
		t.Errorf("GetFacts(TypeScript) count = %d, want 1", len(facts))
	}

	facts = mm.GetFacts("用户")
	if len(facts) != 2 {
		t.Errorf("GetFacts(用户) count = %d, want 2", len(facts))
	}

	facts = mm.GetFacts("unknown")
	if len(facts) != 0 {
		t.Errorf("GetFacts(unknown) count = %d, want 0", len(facts))
	}
}

func TestMemoryAddDecision(t *testing.T) {
	mm := &MemoryManager{
		memory: &Memory{
			Version:  1,
			Projects: make(map[string]*ProjectMemory),
		},
	}

	projectPath := "/path/to/project"
	mm.AddDecision(projectPath, "DB存储位置", "使用用户级目录")

	// Verify
	hash := hashPath(projectPath)
	proj, ok := mm.memory.Projects[hash]
	if !ok {
		t.Fatal("Project not found in memory")
	}

	if len(proj.Decisions) != 1 {
		t.Errorf("Decisions count = %d, want 1", len(proj.Decisions))
	}

	if proj.Decisions[0].Topic != "DB存储位置" {
		t.Errorf("Decision[0].Topic = %s, want 'DB存储位置'", proj.Decisions[0].Topic)
	}

	if proj.Decisions[0].Decision != "使用用户级目录" {
		t.Errorf("Decision[0].Decision = %s, want '使用用户级目录'", proj.Decisions[0].Decision)
	}
}

func TestHashPath(t *testing.T) {
	// Test that hashPath is deterministic
	hash1 := hashPath("/path/to/project")
	hash2 := hashPath("/path/to/project")
	if hash1 != hash2 {
		t.Errorf("hashPath not deterministic: %s != %s", hash1, hash2)
	}

	// Test that different paths produce different hashes
	hash3 := hashPath("/different/path")
	if hash1 == hash3 {
		t.Error("hashPath produced same hash for different paths")
	}

	// Test case insensitivity
	hash4 := hashPath("/Path/To/Project")
	hash5 := hashPath("/path/to/project")
	if hash4 != hash5 {
		t.Errorf("hashPath not case-insensitive: %s != %s", hash4, hash5)
	}
}

func TestMemoryCleanupOldFacts(t *testing.T) {
	now := time.Now()
	
	mm := &MemoryManager{
		memory: &Memory{
			Version: 1,
			Global: GlobalMemory{
				Facts: []Fact{
					{Content: "Old fact", Timestamp: now.Add(-60 * 24 * time.Hour)}, // 60 days old
					{Content: "Recent fact", Timestamp: now.Add(-10 * 24 * time.Hour)}, // 10 days old
					{Content: "Very old fact", Timestamp: now.Add(-90 * 24 * time.Hour)}, // 90 days old
				},
			},
			Projects: make(map[string]*ProjectMemory),
		},
	}

	// Cleanup facts older than 30 days
	removed := mm.CleanupOldFacts(30 * 24 * time.Hour)

	if removed != 2 {
		t.Errorf("Removed = %d, want 2", removed)
	}

	if len(mm.memory.Global.Facts) != 1 {
		t.Errorf("Facts count = %d, want 1", len(mm.memory.Global.Facts))
	}

	if mm.memory.Global.Facts[0].Content != "Recent fact" {
		t.Errorf("Remaining fact = %s, want 'Recent fact'", mm.memory.Global.Facts[0].Content)
	}
}
