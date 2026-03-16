package main

import (
	"encoding/json"
	"flag"
	"net"
	"os"
	"os/exec"
	"strings"
)

// runNotify is the entry point for the `claudemux notify` subcommand.
// It is invoked by Claude Code hooks on Stop and Notification events.
// It connects to the claudemux Unix socket and sends the tmux window name
// and event type so the TUI can update the sidebar instantly.
func runNotify(args []string) {
	fs := flag.NewFlagSet("notify", flag.ExitOnError)
	socket := fs.String("socket", "", "unix socket path")
	event := fs.String("event", "", "event name")
	_ = fs.Parse(args)

	if *socket == "" {
		os.Exit(0)
	}

	// Resolve the tmux window name from the pane in which the hook is running.
	var windowName string
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		out, err := exec.Command("tmux", "display-message", "-p", "-t", pane, "#{window_name}").Output()
		if err == nil {
			windowName = strings.TrimSpace(string(out))
		}
	}

	conn, err := net.Dial("unix", *socket)
	if err != nil {
		// claudemux is not running — exit silently so the hook doesn't error.
		os.Exit(0)
	}
	defer conn.Close()

	_ = json.NewEncoder(conn).Encode(map[string]string{
		"window": windowName,
		"event":  *event,
	})
}
