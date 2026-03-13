package tmux

import tea "github.com/charmbracelet/bubbletea"

// KeyEvent describes a single send-keys invocation.
type KeyEvent struct {
	Key     string
	Literal bool // true = send with -l (raw characters), false = tmux key name
}

// KeyMsgToTmux converts a Bubble Tea key message to a tmux send-keys event.
// Returns nil if the key should not be forwarded.
func KeyMsgToTmux(msg tea.KeyMsg) *KeyEvent {
	switch msg.Type {
	case tea.KeyRunes:
		return &KeyEvent{Key: string(msg.Runes), Literal: true}
	case tea.KeySpace:
		return &KeyEvent{Key: " ", Literal: true}
	case tea.KeyEnter:
		return &KeyEvent{Key: "Enter"}
	case tea.KeyBackspace:
		return &KeyEvent{Key: "BSpace"}
	case tea.KeyDelete:
		return &KeyEvent{Key: "Delete"}
	case tea.KeyTab:
		return &KeyEvent{Key: "Tab"}
	case tea.KeyShiftTab:
		return &KeyEvent{Key: "BTab"}
	case tea.KeyUp:
		return &KeyEvent{Key: "Up"}
	case tea.KeyDown:
		return &KeyEvent{Key: "Down"}
	case tea.KeyLeft:
		return &KeyEvent{Key: "Left"}
	case tea.KeyRight:
		return &KeyEvent{Key: "Right"}
	case tea.KeyHome:
		return &KeyEvent{Key: "Home"}
	case tea.KeyEnd:
		return &KeyEvent{Key: "End"}
	case tea.KeyPgUp:
		return &KeyEvent{Key: "PPage"}
	case tea.KeyPgDown:
		return &KeyEvent{Key: "NPage"}
	case tea.KeyEsc:
		return &KeyEvent{Key: "Escape"}
	case tea.KeyCtrlA:
		return &KeyEvent{Key: "C-a"}
	case tea.KeyCtrlB:
		return &KeyEvent{Key: "C-b"}
	case tea.KeyCtrlC:
		return &KeyEvent{Key: "C-c"}
	case tea.KeyCtrlD:
		return &KeyEvent{Key: "C-d"}
	case tea.KeyCtrlE:
		return &KeyEvent{Key: "C-e"}
	case tea.KeyCtrlF:
		return &KeyEvent{Key: "C-f"}
	case tea.KeyCtrlG:
		return &KeyEvent{Key: "C-g"}
	case tea.KeyCtrlH:
		return &KeyEvent{Key: "C-h"}
	case tea.KeyCtrlJ:
		return &KeyEvent{Key: "C-j"}
	case tea.KeyCtrlK:
		return &KeyEvent{Key: "C-k"}
	case tea.KeyCtrlL:
		return &KeyEvent{Key: "C-l"}
	case tea.KeyCtrlN:
		return &KeyEvent{Key: "C-n"}
	case tea.KeyCtrlO:
		return &KeyEvent{Key: "C-o"}
	case tea.KeyCtrlP:
		return &KeyEvent{Key: "C-p"}
	case tea.KeyCtrlQ:
		return &KeyEvent{Key: "C-q"}
	case tea.KeyCtrlR:
		return &KeyEvent{Key: "C-r"}
	case tea.KeyCtrlS:
		return &KeyEvent{Key: "C-s"}
	case tea.KeyCtrlT:
		return &KeyEvent{Key: "C-t"}
	case tea.KeyCtrlU:
		return &KeyEvent{Key: "C-u"}
	case tea.KeyCtrlV:
		return &KeyEvent{Key: "C-v"}
	case tea.KeyCtrlW:
		return &KeyEvent{Key: "C-w"}
	case tea.KeyCtrlX:
		return &KeyEvent{Key: "C-x"}
	case tea.KeyCtrlY:
		return &KeyEvent{Key: "C-y"}
	case tea.KeyCtrlZ:
		return &KeyEvent{Key: "C-z"}
	}
	return nil
}
