package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ─── Tool definitions for OpenAI function calling ──────────

// ToolDefinition represents an OpenAI-compatible function tool.
type ToolDefinition struct {
	Type     string         `json:"type"`
	Function ToolFunction   `json:"function"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ToolCall represents a tool call from the model response.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolResult represents the result of executing a tool.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Role       string `json:"role"`
	Content    string `json:"content"`
}

// ─── Built-in tools ────────────────────────────────────────

var FileTools = []ToolDefinition{
	{
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
	{
		Type: "function",
		Function: ToolFunction{
			Name:        "list_dir",
			Description: "List files and directories in a path. Returns a listing with file sizes.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Directory path to list (relative to working directory, default: '.')",
					},
				},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunction{
			Name:        "search_files",
			Description: "Search for files matching a pattern in the repository. Returns matching file paths.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Glob pattern to match (e.g. '*.go', '**/*.ts')",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Directory to search in (default: '.')",
					},
				},
				"required": []string{"pattern"},
			},
		},
	},
}

// ─── Tool execution ────────────────────────────────────────

// ExecuteTool executes a tool call and returns the result content.
func ExecuteTool(cwd string, call ToolCall) string {
	switch call.Function.Name {
	case "read_file":
		return executeReadFile(cwd, call.Function.Arguments)
	case "list_dir":
		return executeListDir(cwd, call.Function.Arguments)
	case "search_files":
		return executeSearchFiles(cwd, call.Function.Arguments)
	default:
		return fmt.Sprintf("Error: unknown tool %q", call.Function.Name)
	}
}

func executeReadFile(cwd string, argsJSON string) string {
	var args struct {
		Path string `json:"path"`
	}
	if err := jsonUnmarshal(argsJSON, &args); err != nil {
		return "Error: invalid arguments: " + err.Error()
	}
	if args.Path == "" {
		return "Error: path is required"
	}

	// Resolve path relative to cwd
	fullPath := filepath.Join(cwd, args.Path)

	// Security: prevent path traversal
	cleaned := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleaned, filepath.Clean(cwd)) {
		return "Error: path is outside working directory"
	}

	data, err := os.ReadFile(cleaned)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}

	content := string(data)
	// Truncate very large files
	if len(content) > 50000 {
		content = content[:50000] + "\n\n... (truncated, file too large)"
	}

	return content
}

func executeListDir(cwd string, argsJSON string) string {
	var args struct {
		Path string `json:"path"`
	}
	jsonUnmarshal(argsJSON, &args)
	if args.Path == "" {
		args.Path = "."
	}

	fullPath := filepath.Join(cwd, args.Path)
	cleaned := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleaned, filepath.Clean(cwd)) {
		return "Error: path is outside working directory"
	}

	entries, err := os.ReadDir(cleaned)
	if err != nil {
		return fmt.Sprintf("Error listing directory: %v", err)
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
		return "(empty directory)"
	}
	return result
}

func executeSearchFiles(cwd string, argsJSON string) string {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := jsonUnmarshal(argsJSON, &args); err != nil {
		return "Error: invalid arguments: " + err.Error()
	}
	if args.Pattern == "" {
		return "Error: pattern is required"
	}

	searchDir := cwd
	if args.Path != "" {
		searchDir = filepath.Join(cwd, args.Path)
	}

	cleaned := filepath.Clean(searchDir)
	if !strings.HasPrefix(cleaned, filepath.Clean(cwd)) {
		return "Error: path is outside working directory"
	}

	var matches []string
	filepath.Walk(cleaned, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden directories and node_modules
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "dist" || name == "vendor" {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
		}
		// Match pattern
		matched, _ := filepath.Match(args.Pattern, info.Name())
		if matched {
			rel, _ := filepath.Rel(cwd, path)
			matches = append(matches, rel)
		}
		if len(matches) >= 50 {
			return filepath.SkipDir
		}
		return nil
	})

	if len(matches) == 0 {
		return "No files matched the pattern."
	}
	return strings.Join(matches, "\n")
}

// jsonUnmarshal is a simple JSON unmarshal helper.
func jsonUnmarshal(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}
