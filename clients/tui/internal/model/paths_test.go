package model

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	paths, err := resolvePaths()
	if err != nil {
		t.Fatalf("resolvePaths() error = %v", err)
	}

	// Verify paths are not empty
	if paths.Home == "" {
		t.Error("Home path is empty")
	}
	if paths.Data == "" {
		t.Error("Data path is empty")
	}
	if paths.Config == "" {
		t.Error("Config path is empty")
	}
	if paths.Sessions == "" {
		t.Error("Sessions path is empty")
	}
	if paths.Memory == "" {
		t.Error("Memory path is empty")
	}

	// Verify platform-specific paths
	switch runtime.GOOS {
	case "windows":
		// Windows should use AppData
		if filepath.Base(paths.Data) != appName {
			t.Errorf("Windows Data path should end with %s, got %s", appName, paths.Data)
		}
	case "darwin":
		// macOS should use Library/Application Support
		if filepath.Base(paths.Data) != appName {
			t.Errorf("macOS Data path should end with %s, got %s", appName, paths.Data)
		}
	default:
		// Linux should use XDG paths
		if filepath.Base(paths.Data) != appName {
			t.Errorf("Linux Data path should end with %s, got %s", appName, paths.Data)
		}
	}

	t.Logf("Platform: %s", runtime.GOOS)
	t.Logf("Home: %s", paths.Home)
	t.Logf("Data: %s", paths.Data)
	t.Logf("Config: %s", paths.Config)
	t.Logf("Sessions: %s", paths.Sessions)
	t.Logf("Memory: %s", paths.Memory)
}

func TestEnsureDirectories(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()

	paths := &Paths{
		Home:     tmpDir,
		Data:     filepath.Join(tmpDir, "data"),
		Config:   filepath.Join(tmpDir, "config"),
		Cache:    filepath.Join(tmpDir, "cache"),
		State:    filepath.Join(tmpDir, "state"),
		Sessions: filepath.Join(tmpDir, "sessions"),
		Memory:   filepath.Join(tmpDir, "memory"),
		Backups:  filepath.Join(tmpDir, "backups"),
		Logs:     filepath.Join(tmpDir, "logs"),
	}

	if err := paths.EnsureDirectories(); err != nil {
		t.Fatalf("EnsureDirectories() error = %v", err)
	}

	// Verify all directories were created
	dirs := []string{
		paths.Data,
		paths.Config,
		paths.Cache,
		paths.State,
		paths.Sessions,
		paths.Memory,
		paths.Backups,
		paths.Logs,
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory not created: %s", dir)
		}
	}
}

func TestResolveProjectPaths(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")

	paths := resolveProjectPaths(projectRoot)

	if paths.Root != projectRoot {
		t.Errorf("Root = %s, want %s", paths.Root, projectRoot)
	}

	expectedExtendAI := filepath.Join(projectRoot, ".extendai")
	if paths.ExtendAI != expectedExtendAI {
		t.Errorf("ExtendAI = %s, want %s", paths.ExtendAI, expectedExtendAI)
	}
}

func TestEnsureProjectDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")

	paths := resolveProjectPaths(projectRoot)

	if err := paths.EnsureProjectDirectories(); err != nil {
		t.Fatalf("EnsureProjectDirectories() error = %v", err)
	}

	// Verify directories were created
	dirs := []string{
		paths.ExtendAI,
		paths.Sessions,
		paths.Context,
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory not created: %s", dir)
		}
	}
}
