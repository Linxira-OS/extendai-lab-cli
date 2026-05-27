package model

import (
	"os"
	"path/filepath"
	"runtime"
)

// ─── Cross-platform Path Resolution ──────────────────────────
//
// Follows XDG Base Directory Specification (adapted for Windows)
// Reference: studying/opencode-dev/packages/core/src/global.ts
//
// Windows:
//   Data:   %LOCALAPPDATA%/extendai-lab/
//   Config: %APPDATA%/extendai-lab/
//   Cache:  %LOCALAPPDATA%/extendai-lab/cache/
//   State:  %LOCALAPPDATA%/extendai-lab/state/
//
// Linux/macOS:
//   Data:   ~/.local/share/extendai-lab/
//   Config: ~/.config/extendai-lab/
//   Cache:  ~/.cache/extendai-lab/
//   State:  ~/.local/state/extendai-lab/

const appName = "extendai-lab"

// Paths holds all resolved paths for the application.
type Paths struct {
	// Home is the user's home directory
	Home string

	// Data is the data directory (sessions, memory, databases)
	Data string

	// Config is the configuration directory
	Config string

	// Cache is the cache directory
	Cache string

	// State is the state directory (runtime state)
	State string

	// Sessions is the sessions directory
	Sessions string

	// Memory is the memory directory
	Memory string

	// Backups is the backups directory
	Backups string

	// Logs is the logs directory
	Logs string
}

// resolvePaths returns the resolved paths for the current platform.
func resolvePaths() (*Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var dataRoot, configRoot string

	switch runtime.GOOS {
	case "windows":
		// Windows: Use %LOCALAPPDATA% for data, %APPDATA% for config
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		dataRoot = localAppData
		configRoot = appData

	case "darwin":
		// macOS: Use ~/Library/Application Support
		dataRoot = filepath.Join(home, "Library", "Application Support")
		configRoot = filepath.Join(home, "Library", "Application Support")

	default:
		// Linux/Unix: Follow XDG Base Directory Specification
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(home, ".local", "share")
		}
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(home, ".config")
		}
		dataRoot = xdgData
		configRoot = xdgConfig
	}

	// Resolve paths
	data := filepath.Join(dataRoot, appName)
	config := filepath.Join(configRoot, appName)

	paths := &Paths{
		Home:     home,
		Data:     data,
		Config:   config,
		Cache:    filepath.Join(data, "cache"),
		State:    filepath.Join(data, "state"),
		Sessions: filepath.Join(data, "sessions"),
		Memory:   filepath.Join(data, "memory"),
		Backups:  filepath.Join(data, "backups"),
		Logs:     filepath.Join(data, "logs"),
	}

	return paths, nil
}

// EnsureDirectories creates all required directories.
func (p *Paths) EnsureDirectories() error {
	dirs := []string{
		p.Data,
		p.Config,
		p.Cache,
		p.State,
		p.Sessions,
		p.Memory,
		p.Backups,
		p.Logs,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	return nil
}

// ─── Project-specific Paths ──────────────────────────────────

// ProjectPaths holds paths specific to a project.
type ProjectPaths struct {
	// Root is the project root directory
	Root string

	// ExtendAI is the .extendai directory in the project
	ExtendAI string

	// Sessions is the project-specific sessions directory
	Sessions string

	// Context is the project context directory
	Context string
}

// resolveProjectPaths returns paths for a specific project.
func resolveProjectPaths(projectRoot string) *ProjectPaths {
	extendAI := filepath.Join(projectRoot, ".extendai")

	return &ProjectPaths{
		Root:     projectRoot,
		ExtendAI: extendAI,
		Sessions: filepath.Join(extendAI, "sessions"),
		Context:  filepath.Join(extendAI, "context"),
	}
}

// EnsureProjectDirectories creates project-specific directories.
func (p *ProjectPaths) EnsureProjectDirectories() error {
	dirs := []string{
		p.ExtendAI,
		p.Sessions,
		p.Context,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	return nil
}

// ─── Global Paths Instance ───────────────────────────────────

var globalPaths *Paths

// GetPaths returns the global paths instance, creating it if needed.
func GetPaths() (*Paths, error) {
	if globalPaths != nil {
		return globalPaths, nil
	}

	paths, err := resolvePaths()
	if err != nil {
		return nil, err
	}

	if err := paths.EnsureDirectories(); err != nil {
		return nil, err
	}

	globalPaths = paths
	return paths, nil
}

// GetProjectPaths returns project-specific paths.
func GetProjectPaths(projectRoot string) (*ProjectPaths, error) {
	paths := resolveProjectPaths(projectRoot)
	if err := paths.EnsureProjectDirectories(); err != nil {
		return nil, err
	}
	return paths, nil
}
