package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeDialog
)

func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "NORMAL"
	case ModeInsert:
		return "INSERT"
	case ModeDialog:
		return "DIALOG"
	default:
		return "NORMAL"
	}
}

type StatusBarModel struct {
	width        int
	mode         Mode
	sessionName  string
	errorMessage string
	searchActive bool
	styles       Styles
}

func NewStatusBarModel(styles Styles) StatusBarModel {
	return StatusBarModel{styles: styles}
}

func (s *StatusBarModel) SetWidth(w int) {
	s.width = w
}

func (s *StatusBarModel) SetMode(m Mode) {
	s.mode = m
}

func (s *StatusBarModel) SetSessionName(name string) {
	s.sessionName = name
}

func (s *StatusBarModel) SetError(msg string) {
	s.errorMessage = msg
}

func (s *StatusBarModel) ClearError() {
	s.errorMessage = ""
}

func (s *StatusBarModel) SetSearchActive(active bool) {
	s.searchActive = active
}

func (s StatusBarModel) View() string {
	mode := s.styles.StatusMode.Render(s.mode.String())

	if s.errorMessage != "" {
		errStr := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F87171")).
			Bold(true).
			Render(" error: " + s.errorMessage)
		content := lipgloss.JoinHorizontal(lipgloss.Top, mode, errStr)
		contentWidth := lipgloss.Width(content)
		if contentWidth < s.width {
			padding := strings.Repeat(" ", s.width-contentWidth)
			content = content + s.styles.StatusHints.Copy().UnsetPadding().Render(padding)
		}
		return content
	}

	var hints string
	if s.searchActive {
		hints = "type: search  enter/n: next  N: prev  esc: close"
	} else {
		switch s.mode {
		case ModeNormal:
			hints = "j/k: navigate  1-9: jump  enter: open  n: new  r: rename  s: settings  d: delete  e: export  ctrl+f: search  [/]: resize  ?: help  q: quit"
		case ModeInsert:
			hints = "shift+enter: newline  drag: copy  ctrl+f: search  (all other keys sent to Claude)"
		case ModeDialog:
			hints = "enter: confirm  esc: cancel"
		}
	}

	hintsStr := s.styles.StatusHints.Render(hints)

	var sessionStr string
	if s.sessionName != "" {
		sessionStr = s.styles.StatusHints.Render("  │  " + s.sessionName)
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, mode, hintsStr, sessionStr)

	// Pad to full width
	contentWidth := lipgloss.Width(content)
	if contentWidth < s.width {
		padding := strings.Repeat(" ", s.width-contentWidth)
		content = content + s.styles.StatusHints.Copy().UnsetPadding().Render(padding)
	}

	return content
}
