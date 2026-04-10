package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/merlijnmacgillavry/claudemux/internal/hooks"
	"github.com/merlijnmacgillavry/claudemux/internal/ui"
)

func main() {
	// Dispatch the notify subcommand before starting the TUI.
	if len(os.Args) > 1 && os.Args[1] == "notify" {
		runNotify(os.Args[2:])
		return
	}

	socketPath := hooks.SocketPath()
	listener, listenerErr := hooks.NewListener(socketPath)
	if listenerErr != nil {
		fmt.Fprintf(os.Stderr, "warning: hook listener unavailable: %v\n", listenerErr)
	}

	p := tea.NewProgram(
		ui.NewRootModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithoutSignalHandler(), // ctrl+c is forwarded to the active Claude window
	)

	// WithoutSignalHandler prevents Bubble Tea from catching OS signals, which is
	// intentional so that Ctrl+C (in raw mode: a key event, not SIGINT) forwards
	// to the active Claude window. However, SIGTERM and SIGHUP (external kill,
	// terminal close) would otherwise leave the terminal in mouse-reporting mode,
	// causing stray escape sequences to appear as garbage characters on mouse
	// movement in the host terminal after claudemux exits.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		if <-sigs != nil {
			p.Quit()
		}
	}()

	if listener != nil {
		listener.Start(p)
		if binary, err := os.Executable(); err == nil {
			_ = hooks.InstallHooks(binary, socketPath)
		}
	}

	_, runErr := p.Run()
	signal.Stop(sigs)

	// Always clean up hooks and the socket before exiting, even on error.
	if listener != nil {
		_ = hooks.UninstallHooks()
		listener.Stop()
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", runErr)
		os.Exit(1)
	}
}
