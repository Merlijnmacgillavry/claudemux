package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// hookEntry is a single entry in a Claude Code hooks list.
type hookEntry struct {
	Matcher string `json:"matcher"`
	Command string `json:"command"`
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
					if m, ok := item.(map[string]interface{}); ok {
						c, _ := m["command"].(string)
						if strings.Contains(c, "claudemux notify") {
							continue
						}
						ma, _ := m["matcher"].(string)
						entries = append(entries, hookEntry{Matcher: ma, Command: c})
					}
				}
			}
		}
		entries = append(entries, hookEntry{Matcher: "", Command: cmd})
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
			if m, ok := item.(map[string]interface{}); ok {
				c, _ := m["command"].(string)
				if !strings.Contains(c, "claudemux notify") {
					kept = append(kept, item)
				}
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
