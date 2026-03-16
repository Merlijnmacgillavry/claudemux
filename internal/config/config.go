package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
)

// UIPrefs holds user interface preferences.
type UIPrefs struct {
	SidebarWidth int `json:"sidebarWidth,omitempty"`
}

// DefaultScrollbackLines is the default number of lines captured from a tmux pane.
const DefaultScrollbackLines = 2000

// WindowMeta stores metadata for a tmux-managed Claude window.
type WindowMeta struct {
	DisplayName     string    `json:"displayName"`
	WorkingDir      string    `json:"workingDir,omitempty"`
	ClaudeSessionID string    `json:"claudeSessionID,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	Scrollback      int       `json:"scrollback,omitempty"`      // 0 means use DefaultScrollback
	SkipPermissions bool      `json:"skipPermissions,omitempty"` // pass --dangerously-skip-permissions on respawn
}

// Config is the top-level application configuration.
type Config struct {
	ClaudeBinary      string                `json:"claudeBinary,omitempty"`
	LastWindowName    string                `json:"lastWindowName,omitempty"`
	Windows           map[string]WindowMeta `json:"windows,omitempty"`
	UIPrefs           UIPrefs               `json:"uiPrefs,omitempty"`
	DefaultWorkingDir string                `json:"defaultWorkingDir,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Windows: make(map[string]WindowMeta),
	}
}

// ConfigPath returns the XDG config path for claudemux.
func ConfigPath() (string, error) {
	return xdg.ConfigFile("claudemux/config.json")
}

// Load reads the config from disk. Returns DefaultConfig if the file doesn't exist.
func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}
	return loadFromPath(path)
}

func loadFromPath(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return DefaultConfig(), err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}

	if cfg.Windows == nil {
		cfg.Windows = make(map[string]WindowMeta)
	}

	return cfg, nil
}

func marshalConfig(cfg Config) ([]byte, error) {
	return json.MarshalIndent(cfg, "", "  ")
}

// Save writes the config to disk.
func (c Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := marshalConfig(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
