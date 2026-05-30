package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"reasonix/internal/tool"
)

func init() { tool.RegisterBuiltin(globTool{}) }

// globTool matches files by pattern. workDir, when non-empty, is the directory
// a relative pattern resolves against (see resolveIn).
type globTool struct{ workDir string }

func (globTool) Name() string { return "glob" }

func (globTool) Description() string {
	return "Find files matching a glob pattern (e.g. \"*.go\", \"internal/*/*.go\"). Supports the shell metacharacters * ? [ ]; does not support ** — use bash find for recursive matching."
}

func (globTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob pattern"}},"required":["pattern"]}`)
}

func (globTool) ReadOnly() bool { return true }

func (g globTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if p.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	p.Pattern = resolveIn(g.workDir, p.Pattern)

	matches, err := filepath.Glob(p.Pattern)
	if err != nil {
		return "", fmt.Errorf("glob %q: %w", p.Pattern, err)
	}
	if len(matches) == 0 {
		return "(no matches)", nil
	}
	return strings.Join(matches, "\n"), nil
}
