package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/merlijnmacgillavry/claudemux/internal/config"
)

// DialogType identifies which dialog is showing.
type DialogType int

const (
	DialogNone DialogType = iota
	DialogNewSession
	DialogDirPicker
	DialogRename
	DialogDelete
	DialogHelp
	DialogSettings
)

// dirPickState is a lightweight searchable directory picker.
type dirPickState struct {
	cwd        string
	all        []string // all visible subdirs in cwd
	filtered   []string // filtered by query
	cursor     int
	filter     textinput.Model
	showHidden bool
}

func newDirPickState(startDir string) dirPickState {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.Prompt = "> "
	ti.Focus()
	d := dirPickState{cwd: startDir, filter: ti}
	d.loadEntries()
	return d
}

func (d *dirPickState) loadEntries() {
	entries, err := os.ReadDir(d.cwd)
	if err != nil {
		d.all = nil
		d.filtered = nil
		d.cursor = 0
		return
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && (d.showHidden || !strings.HasPrefix(e.Name(), ".")) {
			dirs = append(dirs, e.Name())
		}
	}
	d.all = dirs
	d.applyFilter()
}

func (d *dirPickState) applyFilter() {
	q := strings.ToLower(d.filter.Value())
	if q == "" {
		cp := make([]string, len(d.all))
		copy(cp, d.all)
		d.filtered = cp
	} else {
		var out []string
		for _, name := range d.all {
			if strings.Contains(strings.ToLower(name), q) {
				out = append(out, name)
			}
		}
		d.filtered = out
	}
	if len(d.filtered) == 0 {
		d.cursor = 0
	} else if d.cursor >= len(d.filtered) {
		d.cursor = len(d.filtered) - 1
	}
}

func (d *dirPickState) descend() {
	if len(d.filtered) == 0 || d.cursor >= len(d.filtered) {
		return
	}
	d.cwd = filepath.Join(d.cwd, d.filtered[d.cursor])
	d.filter.SetValue("")
	d.cursor = 0
	d.loadEntries()
}

func (d *dirPickState) ascend() {
	parent := filepath.Dir(d.cwd)
	if parent == d.cwd {
		return // filesystem root
	}
	d.cwd = parent
	d.filter.SetValue("")
	d.cursor = 0
	d.loadEntries()
}

func (d dirPickState) Update(msg tea.Msg) (dirPickState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+h" {
			d.showHidden = !d.showHidden
			d.loadEntries()
			return d, nil
		}
		switch msg.Type {
		case tea.KeyUp:
			if d.cursor > 0 {
				d.cursor--
			}
			return d, nil
		case tea.KeyDown:
			if d.cursor < len(d.filtered)-1 {
				d.cursor++
			}
			return d, nil
		case tea.KeyRight:
			d.descend()
			return d, nil
		case tea.KeyBackspace:
			if d.filter.Value() == "" {
				d.ascend()
				return d, nil
			}
		}
		// Everything else (including backspace with text) goes to the filter input.
		prev := d.filter.Value()
		var cmd tea.Cmd
		d.filter, cmd = d.filter.Update(msg)
		if d.filter.Value() != prev {
			d.applyFilter()
		}
		return d, cmd
	}
	return d, nil
}

const dirPickerVisibleItems = 12

func (d dirPickState) View() string {
	var sb strings.Builder

	sb.WriteString(d.filter.View())
	sb.WriteString("\n")

	start := 0
	if d.cursor >= dirPickerVisibleItems {
		start = d.cursor - dirPickerVisibleItems + 1
	}

	shown := 0
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")).Bold(true)
	for i := start; i < len(d.filtered) && shown < dirPickerVisibleItems; i++ {
		if i == d.cursor {
			sb.WriteString(sel.Render("▶ " + d.filtered[i]))
		} else {
			sb.WriteString("  " + d.filtered[i])
		}
		sb.WriteString("\n")
		shown++
	}

	if len(d.filtered) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("  (no matches)"))
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// DialogModel handles modal dialog rendering and input.
type DialogModel struct {
	dialogType  DialogType
	styles      Styles
	input       textinput.Model
	title       string
	message     string
	sessionID   string
	sessionName string

	// dir picker state
	dp              dirPickState
	pendingName     string // session name saved during dir picking
	pendingSkipPerms bool   // --dangerously-skip-permissions flag carried from new-session dialog

	// new session options
	skipPermissions bool
	scrollbackInput textinput.Model // second input: scrollback lines in new-session dialog
	newSessionField int             // 0=name, 1=scrollback, 2=permissions
}

func NewDialogModel(styles Styles) DialogModel {
	ti := textinput.New()
	ti.CharLimit = 80
	sb := textinput.New()
	sb.CharLimit = 10
	sb.Placeholder = "2000"
	return DialogModel{
		styles:          styles,
		dialogType:      DialogNone,
		input:           ti,
		scrollbackInput: sb,
	}
}

func (d *DialogModel) ShowNewSession() {
	d.dialogType = DialogNewSession
	d.title = "New Session"
	d.message = ""
	d.input.Placeholder = "Session name (optional)"
	d.input.SetValue("")
	d.input.Focus()
	d.skipPermissions = false
	d.scrollbackInput.SetValue("")
	d.scrollbackInput.Blur()
	d.newSessionField = 0
}

func (d *DialogModel) ShowDirPicker(name, startDir string) tea.Cmd {
	d.pendingName = name
	d.pendingSkipPerms = d.skipPermissions
	d.dialogType = DialogDirPicker
	d.title = "Choose Working Directory"
	d.dp = newDirPickState(startDir)
	return nil
}

func (d *DialogModel) ShowRename(id, currentName string) {
	d.dialogType = DialogRename
	d.title = "Rename Session"
	d.message = ""
	d.sessionID = id
	d.input.Placeholder = "New name"
	d.input.SetValue(currentName)
	d.input.Focus()
}

func (d *DialogModel) ShowDelete(id, name string) {
	d.dialogType = DialogDelete
	d.title = "Delete Session"
	d.message = "Delete \"" + name + "\"? Press enter to confirm."
	d.sessionID = id
	d.sessionName = name
	d.input.Blur()
}

func (d *DialogModel) ShowHelp() {
	d.dialogType = DialogHelp
	d.title = "Keybindings"
	d.input.Blur()
}

func (d *DialogModel) ShowSettings(id, name string, scrollback int) {
	d.dialogType = DialogSettings
	d.title = "Session Settings"
	d.sessionID = id
	d.sessionName = name
	d.input.Placeholder = "Scrollback lines (default 2000)"
	d.input.SetValue(strconv.Itoa(scrollback))
	d.input.Focus()
}

func (d *DialogModel) Close() {
	d.dialogType = DialogNone
	d.input.Blur()
}

func (d DialogModel) IsVisible() bool {
	return d.dialogType != DialogNone
}

func (d DialogModel) Type() DialogType {
	return d.dialogType
}

func (d DialogModel) InputValue() string {
	return d.input.Value()
}

func (d DialogModel) SessionID() string {
	return d.sessionID
}

// PendingName returns the session name stored during the dir-picker step.
func (d DialogModel) PendingName() string {
	return d.pendingName
}

// PendingSkipPermissions returns the --dangerously-skip-permissions flag value.
func (d DialogModel) PendingSkipPermissions() bool {
	return d.pendingSkipPerms
}

// PickerDir returns the currently selected directory in the dir picker.
func (d DialogModel) PickerDir() string {
	return d.dp.cwd
}

// PendingScrollback returns the parsed scrollback line count from the new-session
// scrollback input. Falls back to config.DefaultScrollbackLines if empty or invalid.
func (d DialogModel) PendingScrollback() int {
	val, err := strconv.Atoi(strings.TrimSpace(d.scrollbackInput.Value()))
	if err != nil || val <= 0 {
		return config.DefaultScrollbackLines
	}
	return val
}

func (d DialogModel) Update(msg tea.Msg) (DialogModel, tea.Cmd) {
	var cmd tea.Cmd
	switch d.dialogType {
	case DialogDirPicker:
		d.dp, cmd = d.dp.Update(msg)
	case DialogNewSession:
		if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyTab {
			d.newSessionField = (d.newSessionField + 1) % 3
			switch d.newSessionField {
			case 0:
				d.input.Focus()
				d.scrollbackInput.Blur()
			case 1:
				d.input.Blur()
				d.scrollbackInput.Focus()
			case 2:
				d.input.Blur()
				d.scrollbackInput.Blur()
			}
			return d, nil
		}
		switch d.newSessionField {
		case 0:
			d.input, cmd = d.input.Update(msg)
		case 1:
			d.scrollbackInput, cmd = d.scrollbackInput.Update(msg)
		case 2:
			if k, ok := msg.(tea.KeyMsg); ok && k.String() == " " {
				d.skipPermissions = !d.skipPermissions
			}
		}
	case DialogSettings:
		d.input, cmd = d.input.Update(msg)
	default:
		d.input, cmd = d.input.Update(msg)
	}
	return d, cmd
}

func (d DialogModel) View(screenWidth, screenHeight int) string {
	var body string

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	switch d.dialogType {
	case DialogNewSession:
		checkbox := "[ ] --dangerously-skip-permissions"
		if d.skipPermissions {
			checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("[✓] --dangerously-skip-permissions")
		}
		body = lipgloss.JoinVertical(lipgloss.Left,
			d.styles.DialogTitle.Render(d.title),
			"",
			"Enter a name for the new session:",
			d.input.View(),
			"",
			"Scrollback lines:",
			d.scrollbackInput.View(),
			"",
			checkbox,
			"",
			hint.Render("tab to cycle fields  space to toggle permissions  enter to continue  esc to cancel"),
		)

	case DialogDirPicker:
		cwd := d.dp.cwd
		if home, err := os.UserHomeDir(); err == nil {
			if strings.HasPrefix(cwd, home) {
				cwd = "~" + cwd[len(home):]
			}
		}
		body = lipgloss.JoinVertical(lipgloss.Left,
			d.styles.DialogTitle.Render(d.title),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")).Render(cwd),
			"",
			d.dp.View(),
			"",
			hint.Render("type to filter  ↑↓ navigate  → descend  ⌫ go up  ctrl+h hidden  enter select  esc cancel"),
		)

	case DialogRename:
		body = lipgloss.JoinVertical(lipgloss.Left,
			d.styles.DialogTitle.Render(d.title),
			"",
			d.input.View(),
			"",
			hint.Render("enter to confirm  esc to cancel"),
		)

	case DialogDelete:
		body = lipgloss.JoinVertical(lipgloss.Left,
			d.styles.DialogTitle.Render(d.title),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")).Render(d.message),
			"",
			hint.Render("enter to confirm  esc to cancel"),
		)

	case DialogSettings:
		body = lipgloss.JoinVertical(lipgloss.Left,
			d.styles.DialogTitle.Render(d.title),
			"",
			"Session: "+d.sessionName,
			"",
			"Scrollback lines:",
			d.input.View(),
			"",
			hint.Render("enter to confirm  esc to cancel"),
		)

	case DialogHelp:
		accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")).Bold(true)
		body = lipgloss.JoinVertical(lipgloss.Left,
			d.styles.DialogTitle.Render("Keybindings"),
			"",
			accent.Render("Sidebar (NORMAL mode)"),
			"  j/k        navigate sessions",
			"  enter      open/resume session",
			"  n          new session",
			"  r          rename session",
			"  s          session settings",
			"  d          delete session",
			"  /          search sessions",
			"  [/]        resize sidebar",
			"  ?          toggle this help",
			"  q          quit",
			"",
			accent.Render("Pane navigation"),
			"  alt+l      focus main pane",
			"  alt+h      focus sidebar",
			"  click      switch pane / open session",
			"",
			accent.Render("Main pane (INSERT mode)"),
			"  (all keys go to Claude)",
			"",
			hint.Render("press any key to close"),
		)
	}

	dialog := d.styles.DialogBorder.Render(body)
	dialogWidth := lipgloss.Width(dialog)
	dialogHeight := lipgloss.Height(dialog)

	if dialogWidth > screenWidth-4 {
		dialogWidth = screenWidth - 4
	}

	return lipgloss.Place(
		screenWidth,
		screenHeight,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.NewStyle().
			Width(dialogWidth).
			Height(dialogHeight).
			Render(dialog),
	)
}
