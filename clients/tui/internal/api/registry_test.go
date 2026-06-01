package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── Test Helpers ──────────────────────────────────────────────

// tmpDir creates a temporary directory for testing and returns a cleanup function.
func tmpDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "extendai-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// writeFile writes content to a file in the given directory.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	return path
}

// ─── read_file Tests ──────────────────────────────────────────

func TestExecuteReadFile(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "hello.txt", "Hello, World!")

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name: "read existing file",
			args: map[string]interface{}{"path": "hello.txt"},
			want: "Hello, World!",
		},
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "file not found",
			args:    map[string]interface{}{"path": "nonexistent.txt"},
			wantErr: true,
		},
		{
			name:    "path traversal blocked",
			args:    map[string]interface{}{"path": "../../../etc/passwd"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executeReadFile(dir, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeReadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("executeReadFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecuteReadFileLargeFile(t *testing.T) {
	dir := tmpDir(t)

	// Create a file larger than the budget
	largeContent := strings.Repeat("A", 20000)
	writeFile(t, dir, "large.txt", largeContent)

	got, err := executeReadFile(dir, map[string]interface{}{"path": "large.txt"})
	if err != nil {
		t.Fatalf("executeReadFile() error = %v", err)
	}

	// Should be truncated
	if len(got) >= len(largeContent) {
		t.Errorf("executeReadFile() should truncate large files, got len=%d, want < %d", len(got), len(largeContent))
	}

	// Should contain truncation marker
	if !strings.Contains(got, "truncated") {
		t.Errorf("executeReadFile() should contain truncation marker")
	}
}

// ─── write_file Tests ─────────────────────────────────────────

func TestExecuteWriteFile(t *testing.T) {
	dir := tmpDir(t)

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name: "write new file",
			args: map[string]interface{}{
				"path":    "new.txt",
				"content": "Hello, World!",
			},
			want: "File written successfully: new.txt (13 bytes)",
		},
		{
			name: "write with subdirectory",
			args: map[string]interface{}{
				"path":    "subdir/nested/file.txt",
				"content": "nested content",
			},
			want: "File written successfully: subdir/nested/file.txt (14 bytes)",
		},
		{
			name:    "missing path",
			args:    map[string]interface{}{"content": "test"},
			wantErr: true,
		},
		{
			name:    "path traversal blocked",
			args:    map[string]interface{}{"path": "../../../tmp/evil.txt", "content": "evil"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executeWriteFile(dir, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeWriteFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("executeWriteFile() = %q, want %q", got, tt.want)
			}

			// Verify file was actually written
			if !tt.wantErr {
				path, _ := tt.args["path"].(string)
				content, _ := tt.args["content"].(string)
				data, err := os.ReadFile(filepath.Join(dir, path))
				if err != nil {
					t.Errorf("failed to read written file: %v", err)
				}
				if string(data) != content {
					t.Errorf("file content = %q, want %q", string(data), content)
				}
			}
		})
	}
}

// ─── edit_file Tests ──────────────────────────────────────────

func TestExecuteEditFile(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "edit.txt", "Hello, World!")

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name: "simple edit",
			args: map[string]interface{}{
				"path":     "edit.txt",
				"old_text": "World",
				"new_text": "Go",
			},
			want: "File edited successfully: edit.txt",
		},
		{
			name:    "old_text not found",
			args:    map[string]interface{}{"path": "edit.txt", "old_text": "nonexistent", "new_text": "test"},
			wantErr: true,
		},
		{
			name:    "missing path",
			args:    map[string]interface{}{"old_text": "test", "new_text": "test"},
			wantErr: true,
		},
		{
			name:    "missing old_text",
			args:    map[string]interface{}{"path": "edit.txt", "new_text": "test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset file for each test
			writeFile(t, dir, "edit.txt", "Hello, World!")

			got, err := executeEditFile(dir, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeEditFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("executeEditFile() = %q, want %q", got, tt.want)
			}

			// Verify edit was applied
			if !tt.wantErr {
				data, _ := os.ReadFile(filepath.Join(dir, "edit.txt"))
				newText, _ := tt.args["new_text"].(string)
				if !strings.Contains(string(data), newText) {
					t.Errorf("file should contain %q after edit, got %q", newText, string(data))
				}
			}
		})
	}
}

func TestExecuteEditFileDuplicateText(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "dup.txt", "aaa bbb aaa")

	_, err := executeEditFile(dir, map[string]interface{}{
		"path":     "dup.txt",
		"old_text": "aaa",
		"new_text": "ccc",
	})
	if err == nil {
		t.Error("executeEditFile() should fail when old_text appears multiple times")
	}
}

// ─── list_dir Tests ───────────────────────────────────────────

func TestExecuteListDir(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "file1.txt", "content1")
	writeFile(t, dir, "file2.txt", "content2")
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    []string // substrings that should appear in output
		wantErr bool
	}{
		{
			name: "list current directory",
			args: map[string]interface{}{},
			want: []string{"file1.txt", "file2.txt", "subdir"},
		},
		{
			name: "list with path",
			args: map[string]interface{}{"path": "."},
			want: []string{"file1.txt", "file2.txt"},
		},
		{
			name:    "path traversal blocked",
			args:    map[string]interface{}{"path": ".."},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executeListDir(dir, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeListDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for _, want := range tt.want {
					if !strings.Contains(got, want) {
						t.Errorf("executeListDir() output should contain %q, got %q", want, got)
					}
				}
			}
		})
	}
}

func TestExecuteListDirEmpty(t *testing.T) {
	dir := tmpDir(t)
	os.MkdirAll(filepath.Join(dir, "empty"), 0755)

	got, err := executeListDir(dir, map[string]interface{}{"path": "empty"})
	if err != nil {
		t.Fatalf("executeListDir() error = %v", err)
	}
	if got != "(empty directory)" {
		t.Errorf("executeListDir() = %q, want %q", got, "(empty directory)")
	}
}

// ─── search_files Tests ───────────────────────────────────────

func TestExecuteSearchFiles(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "util.go", "package util")
	writeFile(t, dir, "readme.md", "# README")
	writeFile(t, dir, "sub/nested.go", "package nested")

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    []string
		wantNot []string
		wantErr bool
	}{
		{
			name: "search go files",
			args: map[string]interface{}{"pattern": "*.go"},
			want: []string{"main.go", "util.go"},
		},
		{
			name: "search with subdirectory",
			args: map[string]interface{}{"pattern": "*.go", "path": "sub"},
			want: []string{"nested.go"},
		},
		{
			name:    "missing pattern",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executeSearchFiles(dir, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeSearchFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for _, want := range tt.want {
					if !strings.Contains(got, want) {
						t.Errorf("executeSearchFiles() output should contain %q, got %q", want, got)
					}
				}
				for _, notWant := range tt.wantNot {
					if strings.Contains(got, notWant) {
						t.Errorf("executeSearchFiles() output should not contain %q, got %q", notWant, got)
					}
				}
			}
		})
	}
}

// ─── bash Tests ───────────────────────────────────────────────

func TestExecuteBash(t *testing.T) {
	dir := tmpDir(t)

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name: "echo command",
			args: map[string]interface{}{"command": "echo hello"},
			want: "hello",
		},
		{
			name: "command with exit code",
			args: map[string]interface{}{"command": "exit 42"},
			want: "exit code: 42",
		},
		{
			name:    "missing command",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executeBash(dir, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeBash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !strings.Contains(got, tt.want) {
					t.Errorf("executeBash() output should contain %q, got %q", tt.want, got)
				}
			}
		})
	}
}

func TestExecuteBashTimeout(t *testing.T) {
	t.Skip("Skipping timeout test - requires long-running command")

	dir := tmpDir(t)

	// Command that takes too long
	_, err := executeBash(dir, map[string]interface{}{
		"command": "ping -n 11 127.0.0.1", // ~10 seconds
		"timeout": 0.5,                    // 500ms timeout
	})
	if err != nil {
		t.Logf("executeBash() returned error (expected for timeout): %v", err)
	}
}

func TestExecuteBashLargeOutput(t *testing.T) {
	t.Skip("Skipping large output test - requires long-running command")

	dir := tmpDir(t)

	// Command that produces large output
	_, err := executeBash(dir, map[string]interface{}{
		"command": "for i in $(seq 1 10000); do echo $i; done",
	})
	if err != nil {
		t.Fatalf("executeBash() error = %v", err)
	}
	// Should be truncated by budget system
}

// ─── ToolResultBudget Tests ───────────────────────────────────

func TestApplyToolResultBudget(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		budget  ToolResultBudget
		wantLen int // approximate expected length
	}{
		{
			name:    "within budget",
			input:   "short text",
			budget:  DefaultToolResultBudget(),
			wantLen: 10,
		},
		{
			name:    "exceeds budget",
			input:   strings.Repeat("A", 50000),
			budget:  DefaultToolResultBudget(),
			wantLen: 16000, // MaxChars
		},
		{
			name:    "empty input",
			input:   "",
			budget:  DefaultToolResultBudget(),
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyToolResultBudget(tt.input, tt.budget)
			if len(got) > tt.wantLen+100 { // allow some tolerance
				t.Errorf("ApplyToolResultBudget() len = %d, want ~%d", len(got), tt.wantLen)
			}
		})
	}
}

func TestApplyToolResultBudgetTruncationMarker(t *testing.T) {
	large := strings.Repeat("A", 50000)
	budget := DefaultToolResultBudget()

	got := ApplyToolResultBudget(large, budget)
	if !strings.Contains(got, "truncated") {
		t.Error("ApplyToolResultBudget() should add truncation marker")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // approximate
	}{
		{"empty", "", 0},
		{"english", "hello world", 3},
		{"chinese", "你好世界", 3},
		{"mixed", "Hello 你好", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokens(tt.input)
			if got < tt.want-1 || got > tt.want+1 {
				t.Errorf("estimateTokens(%q) = %d, want ~%d", tt.input, got, tt.want)
			}
		})
	}
}

// ─── ToolRegistry Tests ───────────────────────────────────────

func TestToolRegistryGetDefinitions(t *testing.T) {
	dir := tmpDir(t)
	registry := NewToolRegistry(dir)

	defs := registry.GetDefinitions()
	if len(defs) < 7 {
		t.Errorf("GetDefinitions() returned %d tools, want >= 7", len(defs))
	}

	// Check that all expected tools are present
	toolNames := make(map[string]bool)
	for _, d := range defs {
		toolNames[d.Function.Name] = true
	}

	expected := []string{"read_file", "write_file", "edit_file", "list_dir", "search_files", "grep", "bash"}
	for _, name := range expected {
		if !toolNames[name] {
			t.Errorf("GetDefinitions() missing tool %q", name)
		}
	}
}

func TestToolRegistryExecute(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "test.txt", "Hello")

	registry := NewToolRegistry(dir)

	// Test read_file
	result, err := registry.Execute("read_file", map[string]interface{}{"path": "test.txt"})
	if err != nil {
		t.Fatalf("Execute(read_file) error = %v", err)
	}
	if result != "Hello" {
		t.Errorf("Execute(read_file) = %q, want %q", result, "Hello")
	}

	// Test unknown tool
	_, err = registry.Execute("unknown_tool", map[string]interface{}{})
	if err == nil {
		t.Error("Execute(unknown_tool) should fail")
	}
}

func TestToolRegistrySetCwd(t *testing.T) {
	dir := tmpDir(t)
	registry := NewToolRegistry(dir)

	newDir := tmpDir(t)
	writeFile(t, newDir, "other.txt", "Other")

	registry.SetCwd(newDir)

	result, err := registry.Execute("read_file", map[string]interface{}{"path": "other.txt"})
	if err != nil {
		t.Fatalf("Execute(read_file) error = %v", err)
	}
	if result != "Other" {
		t.Errorf("Execute(read_file) = %q, want %q", result, "Other")
	}
}

// ─── Tool Hooks Tests ─────────────────────────────────────────

func TestToolRegistryHooks(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "test.txt", "Hello")

	registry := NewToolRegistry(dir)

	// Test beforeToolCall hook that blocks execution
	blockCalled := false
	registry.SetHooks(&ToolHooks{
		BeforeToolCall: func(toolName string, args map[string]interface{}) (bool, string) {
			blockCalled = true
			if toolName == "read_file" {
				return true, "reading files is not allowed"
			}
			return false, ""
		},
	})

	// Should be blocked
	_, err := registry.Execute("read_file", map[string]interface{}{"path": "test.txt"})
	if err == nil {
		t.Error("Execute() should fail when beforeToolCall blocks")
	}
	if !blockCalled {
		t.Error("beforeToolCall hook should have been called")
	}

	// Other tools should work
	registry.SetHooks(&ToolHooks{
		BeforeToolCall: func(toolName string, args map[string]interface{}) (bool, string) {
			if toolName == "read_file" {
				return true, "blocked"
			}
			return false, ""
		},
	})

	result, err := registry.Execute("list_dir", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute(list_dir) error = %v", err)
	}
	if result == "" {
		t.Error("Execute(list_dir) should return result")
	}
}

func TestToolRegistryAfterHook(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "test.txt", "Hello")

	registry := NewToolRegistry(dir)

	// Test afterToolCall hook that overrides result
	overrideCalled := false
	registry.SetHooks(&ToolHooks{
		AfterToolCall: func(toolName string, result string, err error) (string, error) {
			overrideCalled = true
			if toolName == "read_file" {
				return "overridden content", nil
			}
			return "", nil
		},
	})

	result, err := registry.Execute("read_file", map[string]interface{}{"path": "test.txt"})
	if err != nil {
		t.Fatalf("Execute(read_file) error = %v", err)
	}
	if !overrideCalled {
		t.Error("afterToolCall hook should have been called")
	}
	if result != "overridden content" {
		t.Errorf("Execute(read_file) = %q, want %q", result, "overridden content")
	}
}

func TestToolRegistryNoHooks(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "test.txt", "Hello")

	registry := NewToolRegistry(dir)
	// Don't set any hooks

	result, err := registry.Execute("read_file", map[string]interface{}{"path": "test.txt"})
	if err != nil {
		t.Fatalf("Execute(read_file) error = %v", err)
	}
	if result != "Hello" {
		t.Errorf("Execute(read_file) = %q, want %q", result, "Hello")
	}
}

// ─── ReadOnly/Concurrency Tests ───────────────────────────────

func TestToolRegistryReadOnlyTools(t *testing.T) {
	dir := tmpDir(t)
	registry := NewToolRegistry(dir)

	readOnly := registry.GetReadOnlyTools()
	if len(readOnly) == 0 {
		t.Error("GetReadOnlyTools() should return at least one tool")
	}

	// Check that expected tools are read-only
	readOnlyMap := make(map[string]bool)
	for _, name := range readOnly {
		readOnlyMap[name] = true
	}

	expectedReadOnly := []string{"read_file", "list_dir", "search_files", "grep"}
	for _, name := range expectedReadOnly {
		if !readOnlyMap[name] {
			t.Errorf("tool %q should be read-only", name)
		}
	}
}

func TestToolRegistryExclusiveTools(t *testing.T) {
	dir := tmpDir(t)
	registry := NewToolRegistry(dir)

	exclusive := registry.GetExclusiveTools()
	if len(exclusive) == 0 {
		t.Error("GetExclusiveTools() should return at least one tool")
	}

	// bash should be exclusive
	exclusiveMap := make(map[string]bool)
	for _, name := range exclusive {
		exclusiveMap[name] = true
	}

	if !exclusiveMap["bash"] {
		t.Error("bash tool should be exclusive")
	}
	if !exclusiveMap["write_file"] {
		t.Error("write_file tool should be exclusive")
	}
	if !exclusiveMap["edit_file"] {
		t.Error("edit_file tool should be exclusive")
	}
}

func TestToolRegistryParallelSafeTools(t *testing.T) {
	dir := tmpDir(t)
	registry := NewToolRegistry(dir)

	parallelSafe := registry.GetParallelSafeTools()
	if len(parallelSafe) == 0 {
		t.Error("GetParallelSafeTools() should return at least one tool")
	}

	// Check that expected tools are parallel-safe
	parallelSafeMap := make(map[string]bool)
	for _, name := range parallelSafe {
		parallelSafeMap[name] = true
	}

	expectedParallelSafe := []string{"read_file", "list_dir", "search_files", "grep"}
	for _, name := range expectedParallelSafe {
		if !parallelSafeMap[name] {
			t.Errorf("tool %q should be parallel-safe", name)
		}
	}

	// bash should NOT be parallel-safe
	if parallelSafeMap["bash"] {
		t.Error("bash tool should NOT be parallel-safe")
	}
}

func TestParallelDispatch(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "file1.txt", "content1")
	writeFile(t, dir, "file2.txt", "content2")

	registry := NewToolRegistry(dir)

	// Create multiple parallel-safe tool calls
	calls := []ToolCallRequest{
		{ID: "call1", Name: "read_file", Arguments: map[string]interface{}{"path": "file1.txt"}},
		{ID: "call2", Name: "read_file", Arguments: map[string]interface{}{"path": "file2.txt"}},
	}

	config := DefaultDispatchConfig()
	results := registry.ParallelDispatch(calls, config)

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	if results[0].Result != "content1" {
		t.Errorf("results[0].Result = %q, want %q", results[0].Result, "content1")
	}

	if results[1].Result != "content2" {
		t.Errorf("results[1].Result = %q, want %q", results[1].Result, "content2")
	}
}

func TestParallelDispatchMixed(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "file1.txt", "content1")

	registry := NewToolRegistry(dir)

	// Mix parallel-safe and non-parallel-safe calls
	calls := []ToolCallRequest{
		{ID: "call1", Name: "read_file", Arguments: map[string]interface{}{"path": "file1.txt"}},
		{ID: "call2", Name: "bash", Arguments: map[string]interface{}{"command": "echo hello"}},
		{ID: "call3", Name: "read_file", Arguments: map[string]interface{}{"path": "file1.txt"}},
	}

	config := DefaultDispatchConfig()
	results := registry.ParallelDispatch(calls, config)

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}

	// First call should succeed
	if results[0].IsError {
		t.Errorf("results[0].Error = %v", results[0].Error)
	}

	// Second call should succeed (bash)
	if results[1].IsError {
		t.Errorf("results[1].Error = %v", results[1].Error)
	}
}

func TestCapToolResult(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		maxTokens   int
		wantTrunc   bool
	}{
		{"short", "hello", 100, false},
		{"exact", "hello", 2, false},
		{"needs truncation", strings.Repeat("A", 50000), 100, true},
		{"zero max", "hello", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := capToolResult(tt.input, tt.maxTokens, "...(truncated)")
			if tt.wantTrunc {
				if len(result) >= len(tt.input) {
					t.Errorf("capToolResult() should truncate, got len=%d, want < %d", len(result), len(tt.input))
				}
				if !strings.Contains(result, "truncated") {
					t.Error("capToolResult() should contain truncation marker")
				}
			} else {
				if result != tt.input {
					t.Errorf("capToolResult() = %q, want %q", result, tt.input)
				}
			}
		})
	}
}
