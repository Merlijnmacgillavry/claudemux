package claude

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FindBinary locates the claude binary using PATH.
func FindBinary() (string, error) {
	return exec.LookPath("claude")
}

// ClaudeHomePath returns ~/.claude directory.
func ClaudeHomePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".claude")
	}
	return filepath.Join(home, ".claude")
}

// HistoryPath returns the path to ~/.claude/history.jsonl
func HistoryPath() string {
	return filepath.Join(ClaudeHomePath(), "history.jsonl")
}

// SettingsPath returns the path to ~/.claude/settings.json
func SettingsPath() string {
	return filepath.Join(ClaudeHomePath(), "settings.json")
}

// EncodePath encodes a path for use in Claude's directory naming convention.
func EncodePath(path string) string {
	return strings.ReplaceAll(path, "/", "-")
}

// LatestSessionID returns the UUID of the newest session file created after
// the given time in the Claude projects directory for the given working directory.
func LatestSessionID(cwd string, after time.Time) (string, error) {
	encoded := EncodePath(cwd)
	dir := filepath.Join(ClaudeHomePath(), "projects", encoded)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read claude projects dir: %w", err)
	}
	var newestTime time.Time
	var newestName string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(after) && info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newestName = strings.TrimSuffix(entry.Name(), ".jsonl")
		}
	}
	if newestName == "" {
		return "", fmt.Errorf("no new sessions found for %s", cwd)
	}
	return newestName, nil
}
