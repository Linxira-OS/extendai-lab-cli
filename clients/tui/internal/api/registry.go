package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ─── Tool Types ────────────────────────────────────────────

// ToolDefinition represents an OpenAI-compatible function tool.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents the function definition within a tool.
type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ─── Tool Hooks (from pi) ─────────────────────────────────────

// ToolHookFunc is the function signature for tool hooks.
// beforeToolCall: return (block=true, reason) to prevent execution.
// afterToolCall: return (overrideResult, overrideErr) to override result.
type BeforeToolCallFunc func(toolName string, args map[string]interface{}) (block bool, reason string)
type AfterToolCallFunc func(toolName string, result string, err error) (overrideResult string, overrideErr error)

// ToolHooks holds before/after hooks for tool execution.
type ToolHooks struct {
	BeforeToolCall BeforeToolCallFunc
	AfterToolCall  AfterToolCallFunc
}

// ─── Tool Registry ──────────────────────────────────────────

// ToolFunc is the function signature for tool execution.
type ToolFunc func(cwd string, args map[string]interface{}) (string, error)

// RegisteredTool holds a tool definition and its execution function.
type RegisteredTool struct {
	Definition ToolDefinition
	Execute    ToolFunc
	ReadOnly   bool   // true for read-only tools (can run in parallel)
	Concurrency string // "shared" (parallel) or "exclusive" (serial)
}

// ToolRegistry manages all available tools.
type ToolRegistry struct {
	tools map[string]*RegisteredTool
	cwd   string
	hooks *ToolHooks
}

// NewToolRegistry creates a new tool registry with all built-in tools.
func NewToolRegistry(cwd string) *ToolRegistry {
	r := &ToolRegistry{
		tools: make(map[string]*RegisteredTool),
		cwd:   cwd,
	}
	r.registerBuiltinTools()
	return r
}

// SetHooks sets the before/after hooks for tool execution.
func (r *ToolRegistry) SetHooks(hooks *ToolHooks) {
	r.hooks = hooks
}

// GetTool returns a tool by name.
func (r *ToolRegistry) GetTool(name string) *RegisteredTool {
	return r.tools[name]
}

// GetDefinitions returns all tool definitions for the API request.
func (r *ToolRegistry) GetDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition)
	}
	return defs
}

// Execute executes a tool by name with the given arguments.
// Runs beforeToolCall hook before execution and afterToolCall hook after.
func (r *ToolRegistry) Execute(name string, args map[string]interface{}) (string, error) {
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	// Run beforeToolCall hook
	if r.hooks != nil && r.hooks.BeforeToolCall != nil {
		block, reason := r.hooks.BeforeToolCall(name, args)
		if block {
			return "", fmt.Errorf("tool execution blocked: %s", reason)
		}
	}

	// Execute the tool
	result, err := tool.Execute(r.cwd, args)

	// Run afterToolCall hook
	if r.hooks != nil && r.hooks.AfterToolCall != nil {
		overrideResult, overrideErr := r.hooks.AfterToolCall(name, result, err)
		if overrideResult != "" || overrideErr != nil {
			return overrideResult, overrideErr
		}
	}

	return result, err
}

// SetCwd updates the working directory for all tools.
func (r *ToolRegistry) SetCwd(cwd string) {
	r.cwd = cwd
}

// GetReadOnlyTools returns names of tools that can run in parallel.
func (r *ToolRegistry) GetReadOnlyTools() []string {
	var names []string
	for name, tool := range r.tools {
		if tool.ReadOnly {
			names = append(names, name)
		}
	}
	return names
}

// GetExclusiveTools returns names of tools that must run serially.
func (r *ToolRegistry) GetExclusiveTools() []string {
	var names []string
	for name, tool := range r.tools {
		if !tool.ReadOnly {
			names = append(names, name)
		}
	}
	return names
}

// ─── Built-in Tools ────────────────────────────────────────

func (r *ToolRegistry) registerBuiltinTools() {
	// read_file
	r.tools["read_file"] = &RegisteredTool{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "read_file",
				Description: "Read the contents of a file. Returns the file content as text.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The file path to read (relative to working directory)",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		Execute:  executeReadFile,
		ReadOnly: true,
	}

	// write_file
	r.tools["write_file"] = &RegisteredTool{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "write_file",
				Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The file path to write (relative to working directory)",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "The content to write to the file",
						},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		Execute: executeWriteFile,
	}

	// edit_file
	r.tools["edit_file"] = &RegisteredTool{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "edit_file",
				Description: "Edit a file by replacing exact text. Use read_file first to see the current content.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The file path to edit",
						},
						"old_text": map[string]interface{}{
							"type":        "string",
							"description": "The exact text to find and replace",
						},
						"new_text": map[string]interface{}{
							"type":        "string",
							"description": "The replacement text",
						},
					},
					"required": []string{"path", "old_text", "new_text"},
				},
			},
		},
		Execute: executeEditFile,
	}

	// list_dir
	r.tools["list_dir"] = &RegisteredTool{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_dir",
				Description: "List files and directories in a path. Returns a listing with file sizes.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path to list (default: current directory)",
						},
					},
				},
			},
		},
		Execute:  executeListDir,
		ReadOnly: true,
	}

	// search_files (glob)
	r.tools["search_files"] = &RegisteredTool{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "search_files",
				Description: "Search for files matching a glob pattern. Returns matching file paths.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "Glob pattern to match (e.g. '*.go', '**/*.ts')",
						},
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory to search in (default: current directory)",
						},
					},
					"required": []string{"pattern"},
				},
			},
		},
		Execute:  executeSearchFiles,
		ReadOnly: true,
	}

	// grep (content search)
	r.tools["grep"] = &RegisteredTool{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "grep",
				Description: "Search for text patterns in file contents. Returns matching lines with file paths and line numbers.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "Regex pattern to search for",
						},
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory or file to search in (default: current directory)",
						},
						"include": map[string]interface{}{
							"type":        "string",
							"description": "File pattern to include (e.g. '*.go')",
						},
					},
					"required": []string{"pattern"},
				},
			},
		},
		Execute:  executeGrep,
		ReadOnly: true,
	}

	// bash (shell execution)
	r.tools["bash"] = &RegisteredTool{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "bash",
				Description: "Execute a shell command and return the output. Use for running tests, building, git commands, etc.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The shell command to execute",
						},
						"timeout": map[string]interface{}{
							"type":        "number",
							"description": "Timeout in seconds (default: 60)",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		Execute:     executeBash,
		ReadOnly:    false,
		Concurrency: "exclusive",
	}
}

// ─── Tool Result Budget ────────────────────────────────────────

// ToolResultBudget defines the maximum size for tool results.
type ToolResultBudget struct {
	MaxTokens  int // maximum tokens for a single tool result
	MaxChars   int // maximum characters (fallback if token estimation unavailable)
	Truncation string // truncation message
}

// DefaultToolResultBudget returns sensible defaults.
func DefaultToolResultBudget() ToolResultBudget {
	return ToolResultBudget{
		MaxTokens:  4000, // ~16KB of text
		MaxChars:   16000,
		Truncation: "\n\n... (truncated to fit context budget)",
	}
}

// ApplyToolResultBudget truncates a tool result to fit within the budget.
// Uses token-aware truncation when possible, falls back to character count.
func ApplyToolResultBudget(result string, budget ToolResultBudget) string {
	if result == "" {
		return result
	}

	// Estimate tokens (rough: 1 token ≈ 4 chars for English, ≈ 1.5 chars for CJK)
	estimatedTokens := estimateTokens(result)

	if estimatedTokens <= budget.MaxTokens {
		return result // within budget
	}

	// Need to truncate — calculate target character count
	// Use a conservative ratio: 1 token ≈ 3 chars (mixed content)
	targetChars := budget.MaxTokens * 3
	if targetChars > budget.MaxChars {
		targetChars = budget.MaxChars
	}

	if len(result) <= targetChars {
		return result
	}

	// Truncate at a reasonable boundary (prefer newline)
	truncated := result[:targetChars]

	// Try to truncate at last newline to avoid cutting mid-line
	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > targetChars/2 {
		truncated = truncated[:lastNewline]
	}

	return truncated + budget.Truncation
}

// estimateTokens provides a rough token estimate for a string.
// This is a simplified version — for production, use the CJK-aware estimator.
func estimateTokens(text string) int {
	// Count CJK characters
	cjk := 0
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF || // CJK Unified Ideographs
			r >= 0x3040 && r <= 0x309F || // Hiragana
			r >= 0x30A0 && r <= 0x30FF || // Katakana
			r >= 0xAC00 && r <= 0xD7AF { // Hangul Syllables
			cjk++
		}
	}

	other := len(text) - cjk*3 // CJK chars are 3 bytes in UTF-8

	// CJK: ~0.67 tokens per char, Other: ~0.25 tokens per char
	cjkTokens := float64(cjk) * 0.67
	otherTokens := float64(other) * 0.25

	return int(cjkTokens + otherTokens + 0.5)
}

// ─── Tool Implementations ──────────────────────────────────

func executeReadFile(cwd string, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	fullPath := filepath.Join(cwd, path)
	cleaned := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleaned, filepath.Clean(cwd)) {
		return "", fmt.Errorf("path is outside working directory")
	}

	data, err := os.ReadFile(cleaned)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	content := string(data)

	// Apply tool result budget
	budget := DefaultToolResultBudget()
	content = ApplyToolResultBudget(content, budget)

	return content, nil
}

func executeWriteFile(cwd string, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	fullPath := filepath.Join(cwd, path)
	cleaned := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleaned, filepath.Clean(cwd)) {
		return "", fmt.Errorf("path is outside working directory")
	}

	// Create directories if needed
	dir := filepath.Dir(cleaned)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(cleaned, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return fmt.Sprintf("File written successfully: %s (%d bytes)", path, len(content)), nil
}

func executeEditFile(cwd string, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	oldText, _ := args["old_text"].(string)
	newText, _ := args["new_text"].(string)
	if path == "" || oldText == "" {
		return "", fmt.Errorf("path and old_text are required")
	}

	fullPath := filepath.Join(cwd, path)
	cleaned := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleaned, filepath.Clean(cwd)) {
		return "", fmt.Errorf("path is outside working directory")
	}

	data, err := os.ReadFile(cleaned)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	content := string(data)
	count := strings.Count(content, oldText)
	if count == 0 {
		return "", fmt.Errorf("old_text not found in file")
	}
	if count > 1 {
		return "", fmt.Errorf("old_text found %d times — must be unique", count)
	}

	newContent := strings.Replace(content, oldText, newText, 1)
	if err := os.WriteFile(cleaned, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return fmt.Sprintf("File edited successfully: %s", path), nil
}

func executeListDir(cwd string, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	fullPath := filepath.Join(cwd, path)
	cleaned := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleaned, filepath.Clean(cwd)) {
		return "", fmt.Errorf("path is outside working directory")
	}

	entries, err := os.ReadDir(cleaned)
	if err != nil {
		return "", fmt.Errorf("listing directory: %w", err)
	}

	var b strings.Builder
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		suffix := ""
		if e.IsDir() {
			suffix = "/"
		}
		b.WriteString(fmt.Sprintf("%-40s %8d bytes%s\n", e.Name(), size, suffix))
	}

	result := b.String()
	if result == "" {
		return "(empty directory)", nil
	}
	return result, nil
}

func executeSearchFiles(cwd string, args map[string]interface{}) (string, error) {
	pattern, _ := args["pattern"].(string)
	path, _ := args["path"].(string)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	searchDir := cwd
	if path != "" {
		searchDir = filepath.Join(cwd, path)
	}

	cleaned := filepath.Clean(searchDir)
	if !strings.HasPrefix(cleaned, filepath.Clean(cwd)) {
		return "", fmt.Errorf("path is outside working directory")
	}

	var matches []string
	filepath.Walk(cleaned, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "dist" || name == "vendor" {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
		}
		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			rel, _ := filepath.Rel(cwd, p)
			matches = append(matches, rel)
		}
		if len(matches) >= 50 {
			return filepath.SkipDir
		}
		return nil
	})

	if len(matches) == 0 {
		return "No files matched the pattern.", nil
	}
	return strings.Join(matches, "\n"), nil
}

func executeGrep(cwd string, args map[string]interface{}) (string, error) {
	pattern, _ := args["pattern"].(string)
	path, _ := args["path"].(string)
	include, _ := args["include"].(string)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// Use ripgrep if available, otherwise fall back to grep
	searchPath := cwd
	if path != "" {
		searchPath = filepath.Join(cwd, path)
	}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("rg"); err == nil {
		args := []string{"--line-number", "--no-heading", "-m", "20"}
		if include != "" {
			args = append(args, "--glob", include)
		}
		args = append(args, pattern, searchPath)
		cmd = exec.Command("rg", args...)
	} else if _, err := exec.LookPath("grep"); err == nil {
		args := []string{"-rn", "-m", "20"}
		if include != "" {
			args = append(args, "--include", include)
		}
		args = append(args, pattern, searchPath)
		cmd = exec.Command("grep", args...)
	} else {
		return "", fmt.Errorf("neither rg nor grep found in PATH")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// grep/rg return exit code 1 when no matches
		if len(output) == 0 {
			return "No matches found.", nil
		}
	}

	result := string(output)
	if len(result) == 0 {
		return "No matches found.", nil
	}

	// Apply tool result budget
	budget := DefaultToolResultBudget()
	result = ApplyToolResultBudget(result, budget)

	return result, nil
}

func executeBash(cwd string, args map[string]interface{}) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	timeout := 60.0
	if t, ok := args["timeout"].(float64); ok && t > 0 {
		timeout = t
	}

	// Use shell
	shell := "/bin/sh"
	if _, err := exec.LookPath("bash"); err == nil {
		shell = "bash"
	}

	cmd := exec.Command(shell, "-c", command)
	cmd.Dir = cwd

	// Set timeout with safe kill (process might have already exited)
	timer := time.AfterFunc(time.Duration(timeout)*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	output, err := cmd.CombinedOutput()
	result := string(output)

	// Apply tool result budget
	budget := DefaultToolResultBudget()
	result = ApplyToolResultBudget(result, budget)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return result + fmt.Sprintf("\n(exit code: %d)", exitErr.ExitCode()), nil
		}
		return result, fmt.Errorf("executing command: %w", err)
	}

	return result, nil
}
