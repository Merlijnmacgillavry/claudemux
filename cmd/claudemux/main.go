package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/merlijnmacgillavry/claudemux/internal/ui"
)

func main() {
	p := tea.NewProgram(
		ui.NewRootModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithoutSignalHandler(), // ctrl+c is forwarded to the active Claude window
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
