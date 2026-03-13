package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const SessionName = "claudemux"

// WindowInfo holds the state of one tmux window.
type WindowInfo struct {
	Name     string
	Index    int
	IsActive bool
	IsDead   bool
}

// Client wraps tmux CLI operations. All methods run tmux subcommands.
type Client struct{}

func New() *Client { return &Client{} }

func (c *Client) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", args...).Output()
	if err != nil {
		// Include tmux's stderr in the error so callers see the real reason.
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// HasSession returns true if the claudemux tmux session exists.
func (c *Client) HasSession() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "tmux", "has-session", "-t", SessionName).Run() == nil
}

// EnsureSession creates the session if it does not exist.
// Returns true if a new session was created.
func (c *Client) EnsureSession() (bool, error) {
	if c.HasSession() {
		return false, nil
	}
	_, err := c.run("new-session", "-d", "-s", SessionName)
	return err == nil, err
}

// ListWindows returns all windows in the claudemux session.
func (c *Client) ListWindows() ([]WindowInfo, error) {
	out, err := c.run(
		"list-windows", "-t", SessionName,
		"-F", "#{window_name}\t#{window_index}\t#{window_active}\t#{pane_dead}",
	)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var windows []WindowInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		idx, _ := strconv.Atoi(parts[1])
		windows = append(windows, WindowInfo{
			Name:     parts[0],
			Index:    idx,
			IsActive: parts[2] == "1",
			IsDead:   parts[3] == "1",
		})
	}
	return windows, nil
}

// NewWindow creates a new named window running the given command.
// The window is created in the background (-d flag).
// remain-on-exit is enabled so the window persists after the process exits,
// letting us capture the final output and detect the dead state cleanly.
func (c *Client) NewWindow(name, command, cwd string) error {
	args := []string{"new-window", "-t", SessionName, "-n", name, "-d"}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	args = append(args, command)
	if _, err := c.run(args...); err != nil {
		return err
	}
	_, _ = c.run("set-option", "-w", "-t", c.target(name), "remain-on-exit", "on")
	return nil
}

// KillWindow destroys the window with the given name.
func (c *Client) KillWindow(name string) error {
	_, err := c.run("kill-window", "-t", c.target(name))
	return err
}

// RenameWindow renames a tmux window. Lazyclaude uses window names as stable
// keys so this is only called to keep tmux in sync when the user renames.
func (c *Client) RenameWindow(oldName, newName string) error {
	_, err := c.run("rename-window", "-t", c.target(oldName), newName)
	return err
}

// IsPaneDead returns true if the pane's process has exited.
// Uses display-message which is lighter than list-panes.
func (c *Client) IsPaneDead(windowName string) (bool, error) {
	out, err := c.run("display-message", "-t", c.target(windowName), "-p", "#{pane_dead}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "1", nil
}

// RespawnPane restarts a dead pane with the given shell command.
// Uses -k to kill any existing process first.
func (c *Client) RespawnPane(windowName, command, cwd string) error {
	args := []string{"respawn-pane", "-k", "-t", c.target(windowName)}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	args = append(args, command)
	_, err := c.run(args...)
	return err
}

// CapturePane returns the current visible contents of a window with ANSI codes.
// height controls how many lines back from the bottom to capture.
func (c *Client) CapturePane(windowName string, height int) (string, error) {
	startLine := fmt.Sprintf("-%d", height)
	out, err := c.run(
		"capture-pane", "-p", "-e",
		"-t", c.target(windowName),
		"-S", startLine,
	)
	return out, err
}

// SendKey sends a keystroke to the named window.
// If literal is true, the key is sent as raw characters (-l flag).
// If literal is false, key is a tmux key name such as "Enter" or "C-c".
func (c *Client) SendKey(windowName, key string, literal bool) error {
	args := []string{"send-keys", "-t", c.target(windowName)}
	if literal {
		args = append(args, "-l", key)
	} else {
		args = append(args, key)
	}
	_, err := c.run(args...)
	return err
}

func (c *Client) target(windowName string) string {
	return SessionName + ":" + windowName
}
