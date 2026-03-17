package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// hookCmd is a single hook command within a hook group.
type hookCmd struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookEntry is a single entry in a Claude Code hooks list.
type hookEntry struct {
	Matcher string    `json:"matcher"`
	Hooks   []hookCmd `json:"hooks"`
}

// settingsPath returns ~/.claude/settings.json.
func settingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".claude", "settings.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// InstallHooks writes claudemux hook entries for the Stop and Notification
// events into ~/.claude/settings.json. Any existing user hooks are preserved;
// only entries whose command contains "claudemux notify" are replaced.
func InstallHooks(claudemuxBinary, socketPath string) error {
	settings, err := readSettings()
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	for event, flagEvent := range map[string]string{
		"Stop":         "stop",
		"Notification": "notification",
	} {
		cmd := claudemuxBinary + " notify --socket " + socketPath + " --event " + flagEvent

		// Collect existing entries, excluding any prior claudemux entries.
		var entries []hookEntry
		if raw, ok := hooks[event]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for _, item := range arr {
					m, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					// Check inner hooks array for any claudemux notify command.
					isClaudemux := false
					if inner, ok := m["hooks"].([]interface{}); ok {
						for _, h := range inner {
							if hm, ok := h.(map[string]interface{}); ok {
								if c, _ := hm["command"].(string); strings.Contains(c, "claudemux notify") {
									isClaudemux = true
									break
								}
							}
						}
					}
					// Also check legacy flat format during migration.
					if c, _ := m["command"].(string); strings.Contains(c, "claudemux notify") {
						isClaudemux = true
					}
					if isClaudemux {
						continue
					}
					ma, _ := m["matcher"].(string)
					// Re-marshal existing entry to preserve its structure.
					if data, err := json.Marshal(m); err == nil {
						var e hookEntry
						if json.Unmarshal(data, &e) == nil {
							e.Matcher = ma
							entries = append(entries, e)
						}
					}
				}
			}
		}
		entries = append(entries, hookEntry{
			Matcher: "",
			Hooks:   []hookCmd{{Type: "command", Command: cmd}},
		})
		hooks[event] = entries
	}

	settings["hooks"] = hooks
	return writeSettings(settings)
}

// UninstallHooks removes all entries containing "claudemux notify" from
// ~/.claude/settings.json, leaving all other hooks intact.
func UninstallHooks() error {
	settings, err := readSettings()
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		return nil
	}

	for event, raw := range hooks {
		arr, ok := raw.([]interface{})
		if !ok {
			continue
		}
		var kept []interface{}
		for _, item := range arr {
			m, ok := item.(map[string]interface{})
			if !ok {
				kept = append(kept, item)
				continue
			}
			isClaudemux := false
			// Check nested hooks array.
			if inner, ok := m["hooks"].([]interface{}); ok {
				for _, h := range inner {
					if hm, ok := h.(map[string]interface{}); ok {
						if c, _ := hm["command"].(string); strings.Contains(c, "claudemux notify") {
							isClaudemux = true
							break
						}
					}
				}
			}
			// Check legacy flat format.
			if c, _ := m["command"].(string); strings.Contains(c, "claudemux notify") {
				isClaudemux = true
			}
			if !isClaudemux {
				kept = append(kept, item)
			}
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	return writeSettings(settings)
}

func readSettings() (map[string]interface{}, error) {
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeSettings(settings map[string]interface{}) error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
