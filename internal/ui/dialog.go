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
	DialogConfirmRestart
)

// dirEntry is one item in the dir picker's combined visible list.
type dirEntry struct {
	label    string // display label (may be ~ abbreviated)
	fullPath string // non-empty for recent dirs; empty means it is a subdirectory name
}

// dirPickState is a searchable directory picker with recent-dirs support.
type dirPickState struct {
	cwd        string
	recents    []string    // full absolute paths of recently used directories
	all        []string    // all visible subdirectory names in cwd
	visible    []dirEntry  // combined recents + subdirs, after filtering
	cursor     int
	filter     textinput.Model
	showHidden bool
}

func newDirPickState(startDir string, recents []string) dirPickState {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.Prompt = "> "
	ti.Focus()
	d := dirPickState{cwd: startDir, recents: recents, filter: ti}
	d.loadEntries()
	return d
}

func (d *dirPickState) loadEntries() {
	entries, err := os.ReadDir(d.cwd)
	if err != nil {
		d.all = nil
		d.applyFilter()
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
	home, _ := os.UserHomeDir()

	abbrev := func(p string) string {
		if home != "" && strings.HasPrefix(p, home) {
			return "~" + p[len(home):]
		}
		return p
	}

	var items []dirEntry

	// Recents first (match against their abbreviated label).
	for _, r := range d.recents {
		label := abbrev(r)
		if q == "" || strings.Contains(strings.ToLower(label), q) {
			items = append(items, dirEntry{label: label, fullPath: r})
		}
	}

	// Subdirectory names.
	for _, name := range d.all {
		if q == "" || strings.Contains(strings.ToLower(name), q) {
			items = append(items, dirEntry{label: name})
		}
	}

	d.visible = items
	if len(d.visible) == 0 {
		d.cursor = 0
	} else if d.cursor >= len(d.visible) {
		d.cursor = len(d.visible) - 1
	}
}

// descend navigates into the selected item.
// For a recent dir it jumps to that full path; for a subdirectory it descends normally.
func (d *dirPickState) descend() {
	if len(d.visible) == 0 || d.cursor >= len(d.visible) {
		return
	}
	item := d.visible[d.cursor]
	if item.fullPath != "" {
		d.cwd = item.fullPath
	} else {
		d.cwd = filepath.Join(d.cwd, item.label)
	}
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

// cursorOnRecent reports whether the cursor is currently on a recent-dir entry.
func (d *dirPickState) cursorOnRecent() bool {
	if d.cursor >= len(d.visible) {
		return false
	}
	return d.visible[d.cursor].fullPath != ""
}

// selectedPath returns the effective directory the user has chosen.
// When the cursor is on a recent-dir entry, that entry's full path is returned
// so a single Enter press selects it without requiring a separate descend step.
// Otherwise the current cwd is returned.
func (d *dirPickState) selectedPath() string {
	if d.cursorOnRecent() {
		return d.visible[d.cursor].fullPath
	}
	return d.cwd
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
			if d.cursor < len(d.visible)-1 {
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

	if len(d.visible) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("  (no matches)"))
		return strings.TrimRight(sb.String(), "\n")
	}

	start := 0
	if d.cursor >= dirPickerVisibleItems {
		start = d.cursor - dirPickerVisibleItems + 1
	}

	sel := lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")).Bold(true)
	recentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))

	shown := 0
	for i := start; i < len(d.visible) && shown < dirPickerVisibleItems; i++ {
		item := d.visible[i]
		if i == d.cursor {
			if item.fullPath != "" {
				sb.WriteString(sel.Render("▶ ★ " + item.label))
			} else {
				sb.WriteString(sel.Render("▶ " + item.label))
			}
		} else if item.fullPath != "" {
			sb.WriteString(recentStyle.Render("  ★ " + item.label))
		} else {
			sb.WriteString("    " + item.label)
		}
		sb.WriteString("\n")
		shown++
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
	dp                  dirPickState
	pendingName         string // session name saved during dir picking
	pendingSkipPerms    bool   // --dangerously-skip-permissions flag carried from new-session dialog
	dirChangeForWindow  string // when set, dir picker is changing an existing session's working dir

	// new session options
	skipPermissions bool
	scrollbackInput textinput.Model // second input: scrollback lines in new-session dialog
	newSessionField int             // 0=name, 1=scrollback, 2=permissions

	// settings dialog options
	settingsField  int    // 0=scrollback, 1=permissions, 2=working dir
	workingDir     string // displayed in settings field 2
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

func (d *DialogModel) ShowDirPicker(name, startDir string, recents []string) tea.Cmd {
	d.pendingName = name
	d.pendingSkipPerms = d.skipPermissions
	d.dirChangeForWindow = ""
	d.dialogType = DialogDirPicker
	d.title = "Choose Working Directory"
	d.dp = newDirPickState(startDir, recents)
	return nil
}

// ShowDirPickerForChange opens the dir picker in "change working directory" mode
// for an existing session identified by windowID.
func (d *DialogModel) ShowDirPickerForChange(windowID, sessionName, startDir string, recents []string) tea.Cmd {
	d.dirChangeForWindow = windowID
	d.pendingName = sessionName
	d.dialogType = DialogDirPicker
	d.title = "Change Working Directory"
	d.dp = newDirPickState(startDir, recents)
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

func (d *DialogModel) ShowConfirmRestart(id, name string) {
	d.dialogType = DialogConfirmRestart
	d.title = "Restart Session?"
	d.sessionID = id
	d.sessionName = name
	d.message = "Restart \"" + name + "\" to apply permissions change?"
	d.input.Blur()
}

func (d *DialogModel) ShowSettings(id, name string, scrollback int, skipPerms bool, workingDir string) {
	d.dialogType = DialogSettings
	d.title = "Session Settings"
	d.sessionID = id
	d.sessionName = name
	d.workingDir = workingDir
	d.input.Placeholder = "Scrollback lines (default 2000)"
	d.input.SetValue(strconv.Itoa(scrollback))
	d.input.Focus()
	d.skipPermissions = skipPerms
	d.settingsField = 0
}

// SettingsSkipPermissions returns the skip-permissions toggle state from the settings dialog.
func (d DialogModel) SettingsSkipPermissions() bool {
	return d.skipPermissions
}

func (d *DialogModel) Close() {
	d.dialogType = DialogNone
	d.dirChangeForWindow = ""
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

// PickerDir returns the directory the user has selected in the dir picker.
// When the cursor rests on a recent-dir entry, that entry's full path is returned
// directly (no descend required); otherwise the current cwd is returned.
func (d DialogModel) PickerDir() string {
	return d.dp.selectedPath()
}

// DirChangeForWindow returns the window ID this dir-picker session is changing,
// or "" when it is for a new session.
func (d DialogModel) DirChangeForWindow() string {
	return d.dirChangeForWindow
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
		if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyTab {
			d.settingsField = (d.settingsField + 1) % 3
			switch d.settingsField {
			case 0:
				d.input.Focus()
			case 1, 2:
				d.input.Blur()
			}
			return d, nil
		}
		switch d.settingsField {
		case 0:
			d.input, cmd = d.input.Update(msg)
		case 1:
			if k, ok := msg.(tea.KeyMsg); ok && k.String() == " " {
				d.skipPermissions = !d.skipPermissions
			}
		case 2:
			// Enter on field 2 is intercepted by confirmDialog in model.go.
		}
	default:
		d.input, cmd = d.input.Update(msg)
	}
	return d, cmd
}

func (d DialogModel) View(screenWidth, screenHeight int) string {
	var body string

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	focused := lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")).Bold(true)

	switch d.dialogType {
	case DialogNewSession:
		var checkbox string
		if d.newSessionField == 2 {
			checkmark := "[ ]"
			if d.skipPermissions {
				checkmark = "[✓]"
			}
			checkbox = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FBBF24")).
				Bold(true).
				Render(checkmark + " --dangerously-skip-permissions")
		} else if d.skipPermissions {
			checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("[✓] --dangerously-skip-permissions")
		} else {
			checkbox = "[ ] --dangerously-skip-permissions"
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

	case DialogConfirmRestart:
		body = lipgloss.JoinVertical(lipgloss.Left,
			d.styles.DialogTitle.Render(d.title),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")).Render(d.message),
			"",
			hint.Render("enter to restart now  esc to keep running"),
		)

	case DialogSettings:
		var checkbox string
		if d.settingsField == 1 {
			checkmark := "[ ]"
			if d.skipPermissions {
				checkmark = "[✓]"
			}
			checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true).
				Render(checkmark + " --dangerously-skip-permissions")
		} else if d.skipPermissions {
			checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).
				Render("[✓] --dangerously-skip-permissions")
		} else {
			checkbox = "[ ] --dangerously-skip-permissions"
		}

		dirLabel := d.workingDir
		if dirLabel == "" {
			dirLabel = "(not set)"
		} else if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dirLabel, home) {
			dirLabel = "~" + dirLabel[len(home):]
		}
		var dirRow string
		if d.settingsField == 2 {
			dirRow = focused.Render("▶ "+dirLabel) + hint.Render("  (enter to change)")
		} else {
			dirRow = "  " + dirLabel
		}

		body = lipgloss.JoinVertical(lipgloss.Left,
			d.styles.DialogTitle.Render(d.title),
			"",
			"Session: "+d.sessionName,
			"",
			"Scrollback lines:",
			d.input.View(),
			"",
			checkbox,
			"",
			"Working directory:",
			dirRow,
			"",
			hint.Render("tab to cycle fields  space to toggle  enter to confirm / change dir  esc to cancel"),
		)

	case DialogHelp:
		accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")).Bold(true)
		body = lipgloss.JoinVertical(lipgloss.Left,
			d.styles.DialogTitle.Render("Keybindings"),
			"",
			accent.Render("Sidebar (NORMAL mode)"),
			"  j/k        navigate sessions",
			"  1-9        jump to session by position",
			"  enter      open/resume session",
			"  n          new session",
			"  r          rename session",
			"  s          session settings (scrollback, dir, permissions)",
			"  x          stop session (keep metadata)",
			"  d          delete session",
			"  e          export session to clipboard",
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
			"  ctrl+f     search session output",
			"  (all other keys go to Claude)",
			"",
			accent.Render("Search"),
			"  type       filter matches",
			"  enter / n  next match",
			"  N          previous match",
			"  esc        close search",
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
