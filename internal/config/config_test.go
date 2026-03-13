package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Windows == nil {
		t.Error("DefaultConfig should initialise Windows map")
	}
}

func TestConfigRoundtrip(t *testing.T) {
	tmp, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	cfg := DefaultConfig()
	cfg.ClaudeBinary = "/usr/local/bin/claude"
	cfg.Windows["w-123"] = WindowMeta{DisplayName: "My Session"}

	data, err := marshalConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp.Name(), data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadFromPath(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}

	if loaded.ClaudeBinary != cfg.ClaudeBinary {
		t.Errorf("ClaudeBinary: got %q, want %q", loaded.ClaudeBinary, cfg.ClaudeBinary)
	}
	if loaded.Windows["w-123"].DisplayName != "My Session" {
		t.Errorf("Windows[w-123].DisplayName: got %q, want %q", loaded.Windows["w-123"].DisplayName, "My Session")
	}
}

func TestDefaultWorkingDirRoundtrip(t *testing.T) {
	tmp, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	cfg := DefaultConfig()
	cfg.DefaultWorkingDir = "/Users/lord/Documents/projects"

	data, err := marshalConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp.Name(), data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadFromPath(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultWorkingDir != cfg.DefaultWorkingDir {
		t.Errorf("DefaultWorkingDir: got %q, want %q", loaded.DefaultWorkingDir, cfg.DefaultWorkingDir)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := loadFromPath("/nonexistent/path/config.json")
	if err != nil {
		t.Errorf("expected no error for missing file, got: %v", err)
	}
	if cfg.Windows == nil {
		t.Error("expected Windows to be initialised even on missing file")
	}
}
