package ui

import (
	"fmt"
	"hash/crc32"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	claudepkg "github.com/merlijnmacgillavry/claudemux/internal/claude"
	"github.com/merlijnmacgillavry/claudemux/internal/config"
	"github.com/merlijnmacgillavry/claudemux/internal/hooks"
	"github.com/merlijnmacgillavry/claudemux/internal/session"
	tmuxpkg "github.com/merlijnmacgillavry/claudemux/internal/tmux"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[\x30-\x3f]*[\x40-\x7e]|\x1b[^[]`)

// idlePrompt matches a line where ❯ appears with nothing but whitespace after
// it — the Claude Code idle prompt. This distinguishes it from the submitted-
// message line (❯ some text the user typed) which also contains ❯ but is
// followed by non-whitespace content.
var idlePrompt = regexp.MustCompile(`❯\s*$`)

// isWaitingForInput returns true when the captured pane content suggests
// Claude is idle at its prompt, ready for the user to type.
// We match only lines where ❯ is at the end of the line with no trailing
// text, so that a just-submitted message (❯ fix the bug…) is not a false
// positive regardless of how many lines of response Claude has written.
func isWaitingForInput(content string) bool {
	clean := ansiEscape.ReplaceAllString(content, "")
	lines := strings.Split(strings.TrimRight(clean, "\n"), "\n")
	start := len(lines) - 5
	if start < 0 {
		start = 0
	}
	for i := len(lines) - 1; i >= start; i-- {
		if idlePrompt.MatchString(lines[i]) {
			return true
		}
	}
	return false
}

// noopMsg is a no-op message used by async commands that produce no meaningful result.
type noopMsg struct{}

// sendKeyDoneMsg is returned by sendKeyCmd once the tmux send-keys call completes.
type sendKeyDoneMsg struct{ err error }

// paneDeadMsg is returned by pollDeadCheck once IsPaneDead resolves.
type paneDeadMsg struct {
	windowName string
	isDead     bool
	generation uint64
}

// FocusedPane indicates which panel currently has keyboard focus.
type FocusedPane int

const (
	SidebarFocused FocusedPane = iota
	MainFocused
)

// --- Tea messages ---

type tmuxReadyMsg struct{}

type windowsDiscoveredMsg struct {
	sessions []session.SessionInfo
}

type tmuxOutputMsg struct {
	windowName string
	content    string
	isDead     bool
	background bool   // true when sent by a background poll; must not restart the active tick chain
	generation uint64 // generation at issue time; 0 for background polls (no check)
}

type sessionIDDetectedMsg struct {
	windowName string
	sessionID  string
}

type tmuxTickMsg struct{ generation uint64 }
type bgPollTickMsg struct{}

type windowCreatedMsg struct {
	windowName  string
	displayName string
	cwd         string
	startedAt   time.Time
}

type errorMsg struct {
	err error
}

// --- Commands ---

func ensureTmuxSession(client *tmuxpkg.Client) tea.Cmd {
	return func() tea.Msg {
		_, err := client.EnsureSession()
		if err != nil {
			return errorMsg{err: fmt.Errorf("tmux: %w", err)}
		}
		return tmuxReadyMsg{}
	}
}

func discoverWindows(client *tmuxpkg.Client, store *session.Store) tea.Cmd {
	return func() tea.Msg {
		tmuxWindows, err := client.ListWindows()
		if err != nil {
			// Session may not exist yet — return empty rather than crashing.
			return windowsDiscoveredMsg{sessions: []session.SessionInfo{}}
		}
		stored := store.AllWindows()
		infos := make([]session.SessionInfo, 0, len(tmuxWindows))
		for _, w := range tmuxWindows {
			meta, ok := stored[w.Name]
			if !ok {
				// Not a window we created — skip the initial session shell, etc.
				continue
			}
			name := meta.DisplayName
			if name == "" {
				name = "claudemux session " + w.Name
			}
			status := session.StatusRunning
			if w.IsDead {
				status = session.StatusStopped
			}
			infos = append(infos, session.SessionInfo{
				ID:         w.Name,
				Name:       name,
				Project:    meta.WorkingDir,
				LastActive: meta.CreatedAt,
				Status:     status,
			})
		}
		return windowsDiscoveredMsg{sessions: infos}
	}
}

func pollCapture(client *tmuxpkg.Client, windowName string, height int, gen uint64) tea.Cmd {
	return func() tea.Msg {
		content, err := client.CapturePane(windowName, height)
		if err != nil {
			// Window is gone entirely (remain-on-exit off, or session killed).
			return tmuxOutputMsg{windowName: windowName, content: "", isDead: true, generation: gen}
		}
		return tmuxOutputMsg{windowName: windowName, content: content, generation: gen}
	}
}

// pollCaptureBg polls a background (non-active) window for its content only.
// It never checks IsPaneDead — background polls exist solely to update the
// waiting-for-input indicator in the sidebar.
func pollCaptureBg(client *tmuxpkg.Client, windowName string, height int) tea.Cmd {
	return func() tea.Msg {
		content, err := client.CapturePane(windowName, height)
		if err != nil {
			return tmuxOutputMsg{windowName: windowName, content: "", isDead: true, background: true}
		}
		return tmuxOutputMsg{windowName: windowName, content: content, background: true}
	}
}

// pollDeadCheck asks tmux whether a pane's process has exited without relying
// on CapturePane errors (which may not fire when remain-on-exit is enabled).
// gen is the tickGeneration at issue time; stale results are discarded.
func pollDeadCheck(client *tmuxpkg.Client, windowName string, gen uint64) tea.Cmd {
	return func() tea.Msg {
		isDead, _ := client.IsPaneDead(windowName)
		return paneDeadMsg{windowName: windowName, isDead: isDead, generation: gen}
	}
}

// sendKeyCmd sends a keystroke to tmux asynchronously so that the Bubble Tea
// event loop is never blocked by the tmux subprocess.
func sendKeyCmd(client *tmuxpkg.Client, windowName, key string, literal bool) tea.Cmd {
	return func() tea.Msg {
		err := client.SendKey(windowName, key, literal)
		return sendKeyDoneMsg{err: err}
	}
}

// detectSessionID waits briefly then looks for a new Claude session file in the
// project directory. Fires once after window creation to capture the session ID.
func detectSessionID(cwd, windowName string, after time.Time) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(4 * time.Second)
		id, err := claudepkg.LatestSessionID(cwd, after)
		if err != nil {
			return noopMsg{}
		}
		return sessionIDDetectedMsg{windowName: windowName, sessionID: id}
	}
}

func startTick(gen uint64, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return tmuxTickMsg{generation: gen}
	})
}

// tickInterval returns the polling interval based on recent activity.
// Fast while output is changing or the user is typing, slow when idle.
func (m *RootModel) tickInterval() time.Duration {
	switch {
	case m.unchangedTicks < 5:
		return 16 * time.Millisecond // ~60fps; capture-pane at small depth is ~5ms
	case m.unchangedTicks < 20:
		return 250 * time.Millisecond
	default:
		return 1 * time.Second
	}
}

// captureDepth returns how many lines back to request from capture-pane.
// While content is actively changing, only capture what's visible — this is
// ~5ms vs ~100ms for a full 2000-line scrollback. Once content stabilises,
// switch to the full configured scrollback so history is available to scroll.
func (m *RootModel) captureDepth() int {
	if m.unchangedTicks < 5 {
		h := m.mainPane.viewport.Height
		if h < 20 {
			h = 20
		}
		return h + 15 // visible lines + a small buffer
	}
	return m.paneHeight()
}


func createWindow(client *tmuxpkg.Client, store *session.Store, claudeBinary, displayName, cwd string, skipPerms bool, scrollback int) tea.Cmd {
	startedAt := time.Now()
	command := claudeBinary
	if skipPerms {
		command = claudeBinary + " --dangerously-skip-permissions"
	}
	return func() tea.Msg {
		windowName := fmt.Sprintf("w-%d", time.Now().UnixMilli())
		if err := client.NewWindow(windowName, command, cwd); err != nil {
			// Session may have been destroyed since startup — recreate and retry once.
			if _, sessErr := client.EnsureSession(); sessErr == nil {
				if retryErr := client.NewWindow(windowName, command, cwd); retryErr == nil {
					goto created
				}
			}
			return errorMsg{err: fmt.Errorf("new tmux window: %w", err)}
		}
	created:
		store.SetWindow(windowName, displayName, cwd)
		if scrollback > 0 {
			store.SetScrollback(windowName, scrollback)
		}
		if skipPerms {
			store.SetSkipPermissions(windowName, true)
		}
		cfg := store.GetConfig()
		_ = cfg.Save()
		return windowCreatedMsg{windowName: windowName, displayName: displayName, cwd: cwd, startedAt: startedAt}
	}
}

// --- Root model ---

// RootModel is the top-level Bubble Tea model.
type RootModel struct {
	width     int
	height    int
	focus     FocusedPane
	mode      Mode
	styles    Styles
	sidebar   SidebarModel
	mainPane  MainPaneModel
	statusBar StatusBarModel
	dialog    DialogModel

	cfg          config.Config
	store        *session.Store
	claudeBinary string
	tmux         *tmuxpkg.Client

	activeWindowName string
	lastCaptureHash  uint32
	waitingWindows   map[string]bool // windowName → is waiting for input
	tickGeneration   uint64          // incremented each time a new tick chain is started
	unchangedTicks   int             // consecutive ticks where content did not change

	respawnCount    map[string]int       // consecutive respawn attempts per window
	lastRespawnTime map[string]time.Time // time of last respawn per window

	resizeSem      chan struct{} // limits concurrent resize-window subprocesses to 1
	sessionsLoaded bool
}

// NewRootModel creates the initial model.
func NewRootModel() *RootModel {
	styles := DefaultStyles()
	cfg, _ := config.Load()
	binary, _ := claudepkg.FindBinary()
	store := session.NewStore(cfg)

	return &RootModel{
		styles:          styles,
		sidebar:         NewSidebarModel(styles),
		mainPane:        NewMainPaneModel(styles),
		statusBar:       NewStatusBarModel(styles),
		dialog:          NewDialogModel(styles),
		focus:           SidebarFocused,
		mode:            ModeNormal,
		cfg:             cfg,
		store:           store,
		claudeBinary:    binary,
		tmux:            tmuxpkg.New(),
		waitingWindows:  make(map[string]bool),
		respawnCount:    make(map[string]int),
		lastRespawnTime: make(map[string]time.Time),
		resizeSem:       make(chan struct{}, 1),
	}
}

func (m *RootModel) Init() tea.Cmd {
	return tea.Batch(ensureTmuxSession(m.tmux), func() tea.Msg { return bgPollTickMsg{} })
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()

	case errorMsg:
		m.statusBar.SetError(msg.err.Error())
		m.mainPane.AppendContent(fmt.Sprintf("\n[error: %v]\n", msg.err))

	case tmuxReadyMsg:
		return m, discoverWindows(m.tmux, m.store)

	case windowsDiscoveredMsg:
		m.sessionsLoaded = true
		m.sessions(msg.sessions)

	case tmuxTickMsg:
		// Discard ticks from superseded chains.
		if msg.generation != m.tickGeneration {
			return m, nil
		}
		if m.activeWindowName != "" {
			return m, pollCapture(m.tmux, m.activeWindowName, m.captureDepth(), m.tickGeneration)
		}
		return m, startTick(m.tickGeneration, 1*time.Second)

	case bgPollTickMsg:
		// One-shot sweep on startup: poll all background windows to pick up any
		// sessions that went idle before hooks were installed this run.
		// After this, the hook path handles all further notifications.
		const maxBgPolls = 3
		var cmds []tea.Cmd
		pollCount := 0
		for _, item := range m.sidebar.list.Items() {
			if pollCount >= maxBgPolls {
				break
			}
			sess, ok := item.(SessionItem)
			if !ok || !sess.IsRunning || sess.ID == m.activeWindowName {
				continue
			}
			if m.waitingWindows[sess.ID] {
				continue
			}
			id := sess.ID
			cmds = append(cmds, pollCaptureBg(m.tmux, id, 5))
			pollCount++
		}
		return m, tea.Batch(cmds...)

	case tmuxOutputMsg:
		// Track waiting-for-input state for all polled windows.
		wasWaiting := m.waitingWindows[msg.windowName]
		nowWaiting := !msg.isDead && isWaitingForInput(msg.content)
		if nowWaiting != wasWaiting {
			m.waitingWindows[msg.windowName] = nowWaiting
			m.sidebar.SetWaiting(msg.windowName, nowWaiting)
		}

		// Background messages must not restart the active tick chain.
		if msg.windowName != m.activeWindowName {
			return m, nil
		}

		// Discard stale captures from a superseded generation — they would spawn
		// a parallel tick chain at the current generation if allowed through.
		if msg.generation != 0 && msg.generation != m.tickGeneration {
			return m, nil
		}

		// If the pane is completely gone (CapturePane error), auto-respawn Claude.
		if msg.isDead {
			meta, _ := m.store.GetWindow(msg.windowName)
			displayName := meta.DisplayName
			if displayName == "" {
				displayName = "claudemux session " + msg.windowName
			}
			if cmd := m.maybeRespawn(msg.windowName, displayName); cmd != nil {
				return m, cmd
			}
			return m, startTick(m.tickGeneration, 1*time.Second)
		}

		h := crc32.ChecksumIEEE([]byte(msg.content))
		if h != m.lastCaptureHash {
			m.lastCaptureHash = h
			m.unchangedTicks = 0
			m.mainPane.ClearSelection() // stale coords after content change
			m.mainPane.SetContent(msg.content)
			return m, startTick(m.tickGeneration, m.tickInterval())
		}
		// Content unchanged — the process may have exited with remain-on-exit.
		// Only fire a dead check every 5 unchanged ticks to avoid spawning two
		// tmux subprocesses per tick during idle periods.
		m.unchangedTicks++
		if m.unchangedTicks%5 == 0 {
			return m, tea.Batch(startTick(m.tickGeneration, m.tickInterval()), pollDeadCheck(m.tmux, msg.windowName, m.tickGeneration))
		}
		return m, startTick(m.tickGeneration, m.tickInterval())

	case paneDeadMsg:
		// Discard stale dead-checks (e.g. from a previous tick generation).
		if msg.generation != m.tickGeneration || !msg.isDead {
			return m, nil
		}
		if msg.windowName == m.activeWindowName {
			meta, _ := m.store.GetWindow(msg.windowName)
			displayName := meta.DisplayName
			if displayName == "" {
				displayName = "claudemux session " + msg.windowName
			}
			if cmd := m.maybeRespawn(msg.windowName, displayName); cmd != nil {
				return m, cmd
			}
		}

	case sendKeyDoneMsg:
		if msg.err != nil {
			m.statusBar.SetError(fmt.Sprintf("send key: %v", msg.err))
		}

	case sessionIDDetectedMsg:
		m.store.SetClaudeSessionID(msg.windowName, msg.sessionID)
		cfg := m.store.GetConfig()
		_ = cfg.Save()

	case hooks.HookNotificationMsg:
		// Instant notification from a Claude Code hook: mark the window as
		// waiting for input without waiting for the next background poll.
		m.waitingWindows[msg.WindowName] = true
		m.sidebar.SetWaiting(msg.WindowName, true)

	case noopMsg:
		// nothing to do

	case windowCreatedMsg:
		cmd := m.openWindow(msg.windowName, msg.displayName, true)
		var detectCmd tea.Cmd
		if msg.cwd != "" {
			detectCmd = detectSessionID(msg.cwd, msg.windowName, msg.startedAt)
		}
		return m, tea.Batch(cmd, discoverWindows(m.tmux, m.store), detectCmd)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *RootModel) sessions(infos []session.SessionInfo) {
	items := make([]SessionItem, len(infos))
	for i, s := range infos {
		items[i] = SessionItem{
			ID:            s.ID,
			Name:          s.Name,
			LastActive:    s.LastActive,
			IsRunning:     s.Status == session.StatusRunning,
			WorkingDir:    s.Project,
			WaitsForInput: m.waitingWindows[s.ID],
		}
	}
	m.sidebar.SetSessions(items)
}

func (m *RootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+q: universal quit from any mode
	if msg.String() == "ctrl+q" {
		return m, tea.Quit
	}

	// ctrl+c: forward to claude in insert mode, quit otherwise
	if msg.String() == "ctrl+c" {
		if m.mode == ModeInsert && m.activeWindowName != "" {
			return m, sendKeyCmd(m.tmux, m.activeWindowName, "C-c", false)
		}
		return m, tea.Quit
	}

	// alt+h/l: universal pane navigation (works from any non-dialog mode)
	if m.mode != ModeDialog {
		switch msg.String() {
		case "alt+h":
			m.focus = SidebarFocused
			m.mode = ModeNormal
			m.statusBar.SetMode(m.mode)
			return m, nil
		case "alt+l":
			if m.mainPane.hasSession {
				m.focus = MainFocused
				m.mode = ModeInsert
				m.statusBar.SetMode(m.mode)
			}
			return m, nil
		}
	}

	switch m.mode {
	case ModeDialog:
		return m.handleDialogKey(msg)
	case ModeInsert:
		return m.handleInsertKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m *RootModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.statusBar.ClearError()

	// While the sidebar list is accepting filter input, forward every key to it
	// rather than treating letters like n/r/d as global shortcuts.
	if m.sidebar.IsFiltering() {
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "?":
		m.dialog.ShowHelp()
		m.mode = ModeDialog
		m.statusBar.SetMode(m.mode)
		return m, nil

	case "enter":
		sel := m.sidebar.SelectedSession()
		if sel != nil {
			cmd := m.openWindow(sel.ID, sel.Name, sel.IsRunning)
			return m, cmd
		}
		return m, nil

	case "n":
		m.dialog.ShowNewSession()
		m.mode = ModeDialog
		m.statusBar.SetMode(m.mode)
		return m, nil

	case "r":
		sel := m.sidebar.SelectedSession()
		if sel != nil {
			m.dialog.ShowRename(sel.ID, sel.Name)
			m.mode = ModeDialog
			m.statusBar.SetMode(m.mode)
		}
		return m, nil

	case "d":
		sel := m.sidebar.SelectedSession()
		if sel != nil {
			m.dialog.ShowDelete(sel.ID, sel.Name)
			m.mode = ModeDialog
			m.statusBar.SetMode(m.mode)
		}
		return m, nil

	case "s":
		sel := m.sidebar.SelectedSession()
		if sel != nil {
			meta, _ := m.store.GetWindow(sel.ID)
			scrollback := meta.Scrollback
			if scrollback == 0 {
				scrollback = config.DefaultScrollbackLines
			}
			m.dialog.ShowSettings(sel.ID, sel.Name, scrollback, meta.SkipPermissions)
			m.mode = ModeDialog
			m.statusBar.SetMode(m.mode)
		}
		return m, nil

	case "[":
		newW := m.sidebarWidth() - 2
		if newW < 16 {
			newW = 16
		}
		m.cfg.UIPrefs.SidebarWidth = newW
		m.resize()
		_ = m.cfg.Save()
		return m, nil

	case "]":
		newW := m.sidebarWidth() + 2
		if newW > 60 {
			newW = 60
		}
		if m.width-newW < 20 {
			newW = m.width - 20
		}
		if newW < 16 {
			return m, nil
		}
		m.cfg.UIPrefs.SidebarWidth = newW
		m.resize()
		_ = m.cfg.Save()
		return m, nil

	default:
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		return m, cmd
	}
}

func (m *RootModel) handleInsertKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.activeWindowName != "" {
		// Shift+Enter → send kitty-protocol Shift+Enter sequence to Claude Code.
		// Requires a terminal that emits \x1b[13;2u (kitty keyboard protocol).
		if msg.String() == "shift+enter" {
			needsFastTick := m.unchangedTicks >= 5
			m.unchangedTicks = 0
			sendCmd := sendKeyCmd(m.tmux, m.activeWindowName, "\x1b[13;2u", true)
			if needsFastTick {
				m.tickGeneration++
				return m, tea.Batch(sendCmd, startTick(m.tickGeneration, 16*time.Millisecond))
			}
			return m, sendCmd
		}
		if evt := tmuxpkg.KeyMsgToTmux(msg); evt != nil {
			// Only restart the tick chain if we were in slow-poll mode (idle).
			// If already polling at 100ms, let the existing tick fire naturally —
			// resetting it on every keystroke starves the display during fast typing.
			needsFastTick := m.unchangedTicks >= 5
			m.unchangedTicks = 0
			cmd := sendKeyCmd(m.tmux, m.activeWindowName, evt.Key, evt.Literal)
			if needsFastTick {
				m.tickGeneration++
				return m, tea.Batch(cmd, startTick(m.tickGeneration, 16*time.Millisecond))
			}
			return m, cmd
		}
	}
	return m, nil
}

func (m *RootModel) handleDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dialog.Close()
		m.mode = ModeNormal
		m.statusBar.SetMode(m.mode)
		return m, nil

	case "enter":
		return m.confirmDialog()

	default:
		if m.dialog.Type() == DialogHelp {
			m.dialog.Close()
			m.mode = ModeNormal
			m.statusBar.SetMode(m.mode)
			return m, nil
		}
		var cmd tea.Cmd
		m.dialog, cmd = m.dialog.Update(msg)
		return m, cmd
	}
}

func (m *RootModel) confirmDialog() (tea.Model, tea.Cmd) {
	switch m.dialog.Type() {
	case DialogNewSession:
		name := m.dialog.InputValue()
		if name == "" {
			name = fmt.Sprintf("claudemux session %s", time.Now().Format("Jan 2 15:04"))
		}
		if m.claudeBinary == "" {
			m.dialog.Close()
			m.mode = ModeNormal
			m.statusBar.SetMode(m.mode)
			m.statusBar.SetError("claude binary not found in PATH")
			return m, nil
		}
		cwd := m.cfg.DefaultWorkingDir
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		initCmd := m.dialog.ShowDirPicker(name, cwd)
		// stay in ModeDialog
		return m, initCmd

	case DialogDirPicker:
		cwd := m.dialog.PickerDir()
		name := m.dialog.PendingName()
		skipPerms := m.dialog.PendingSkipPermissions()
		scrollback := m.dialog.PendingScrollback()
		m.dialog.Close()
		m.mode = ModeNormal
		m.statusBar.SetMode(m.mode)
		return m, createWindow(m.tmux, m.store, m.claudeBinary, name, cwd, skipPerms, scrollback)

	case DialogRename:
		id := m.dialog.SessionID()
		name := m.dialog.InputValue()
		if name != "" && id != "" {
			m.store.SetWindow(id, name, "")
			cfg := m.store.GetConfig()
			_ = cfg.Save()
			if id == m.activeWindowName {
				m.mainPane.SetSession(name)
				m.statusBar.SetSessionName(name)
			}
		}
		m.dialog.Close()
		m.mode = ModeNormal
		m.statusBar.SetMode(m.mode)
		return m, discoverWindows(m.tmux, m.store)

	case DialogDelete:
		id := m.dialog.SessionID()
		if id != "" {
			_ = m.tmux.KillWindow(id)
			m.store.RemoveWindow(id)
			cfg := m.store.GetConfig()
			_ = cfg.Save()
			delete(m.waitingWindows, id)
			delete(m.respawnCount, id)
			delete(m.lastRespawnTime, id)
			if id == m.activeWindowName {
				m.activeWindowName = ""
				m.lastCaptureHash = 0
				m.mainPane.ClearSession()
				m.statusBar.SetSessionName("")
			}
		}
		m.dialog.Close()
		m.mode = ModeNormal
		m.statusBar.SetMode(m.mode)
		return m, discoverWindows(m.tmux, m.store)

	case DialogHelp:
		m.dialog.Close()
		m.mode = ModeNormal
		m.statusBar.SetMode(m.mode)
		return m, nil

	case DialogSettings:
		id := m.dialog.SessionID()
		name := m.dialog.sessionName
		if id != "" {
			lines, err := strconv.Atoi(strings.TrimSpace(m.dialog.InputValue()))
			if err != nil || lines < 100 {
				lines = 100
			}
			if lines > 50000 {
				lines = 50000
			}
			oldMeta, _ := m.store.GetWindow(id)
			oldSkip := oldMeta.SkipPermissions
			newSkip := m.dialog.SettingsSkipPermissions()
			m.store.SetScrollback(id, lines)
			m.store.SetSkipPermissions(id, newSkip)
			cfg := m.store.GetConfig()
			_ = cfg.Save()
			// Offer restart only when the permissions flag changed and the session is live.
			if newSkip != oldSkip {
				isActive := id == m.activeWindowName
				isRunning := false
				for _, item := range m.sidebar.list.Items() {
					if sess, ok := item.(SessionItem); ok && sess.ID == id {
						isRunning = sess.IsRunning
						break
					}
				}
				if isRunning || isActive {
					m.dialog.ShowConfirmRestart(id, name)
					// stay in ModeDialog
					return m, nil
				}
			}
		}
		m.dialog.Close()
		m.mode = ModeNormal
		m.statusBar.SetMode(m.mode)
		return m, nil

	case DialogConfirmRestart:
		id := m.dialog.SessionID()
		name := m.dialog.sessionName
		m.dialog.Close()
		m.mode = ModeNormal
		m.statusBar.SetMode(m.mode)
		if id != "" {
			return m, m.openWindow(id, name, false)
		}
		return m, nil
	}

	return m, nil
}

// maybeRespawn respawns the named window's Claude process unless it has already
// been respawned more than 3 times within the last 30 seconds — in which case
// it shows an error in the main pane and returns nil so the caller can simply
// restart the tick chain. Callers should not call openWindow themselves when
// using maybeRespawn.
func (m *RootModel) maybeRespawn(windowName, displayName string) tea.Cmd {
	const maxRespawns = 3
	const respawnWindow = 30 * time.Second

	now := time.Now()
	if last, ok := m.lastRespawnTime[windowName]; ok && now.Sub(last) < respawnWindow {
		if m.respawnCount[windowName] >= maxRespawns {
			m.mainPane.AppendContent(fmt.Sprintf(
				"\n[claudemux: session %q has crashed %d times in the last 30 seconds — stopped auto-respawning. Press enter on the session to retry.]\n",
				displayName, maxRespawns,
			))
			return nil
		}
	} else {
		// Outside the backoff window — reset the counter.
		m.respawnCount[windowName] = 0
	}
	m.respawnCount[windowName]++
	m.lastRespawnTime[windowName] = now
	return m.openWindow(windowName, displayName, false)
}

// openWindow switches the main pane to the given tmux window and starts polling.
// If the pane is dead (isRunning=false), it is respawned with claude resume <id>
// (or bare claude if no session ID is stored yet).
func (m *RootModel) openWindow(windowName, displayName string, isRunning bool) tea.Cmd {
	m.activeWindowName = windowName
	m.lastCaptureHash = 0
	m.unchangedTicks = 0
	m.mainPane.SetSession(displayName)
	m.mainPane.ClearContent()
	m.statusBar.SetSessionName(displayName)
	m.focus = MainFocused
	m.mode = ModeInsert
	m.statusBar.SetMode(m.mode)

	// User-initiated open: forgive prior crash history.
	if isRunning {
		delete(m.respawnCount, windowName)
		delete(m.lastRespawnTime, windowName)
	}

	if !isRunning {
		meta, _ := m.store.GetWindow(windowName)
		claudeCmd := m.claudeBinary
		if meta.SkipPermissions {
			claudeCmd += " --dangerously-skip-permissions"
		}
		if meta.ClaudeSessionID != "" {
			claudeCmd += " resume " + meta.ClaudeSessionID
		}
		_ = m.tmux.RespawnPane(windowName, claudeCmd, meta.WorkingDir)
	}

	// Increment the generation to invalidate any tick messages still in flight
	// from the previous active window's chain.
	m.tickGeneration++
	paneH := m.paneHeight()

	sw := m.sidebarWidth()
	mw := m.width - sw
	sh := 1
	ph := m.height - sh
	tw := mw - 3
	th := ph - 3
	if tw > 0 && th > 0 {
		select {
		case m.resizeSem <- struct{}{}:
			go func() {
				defer func() { <-m.resizeSem }()
				_ = m.tmux.ResizeWindow(windowName, tw, th)
			}()
		default:
			// A resize is already in flight; skip this one.
		}
	}

	return pollCapture(m.tmux, windowName, paneH, m.tickGeneration)
}

func (m *RootModel) paneHeight() int {
	if m.activeWindowName != "" {
		if meta, ok := m.store.GetWindow(m.activeWindowName); ok {
			if meta.Scrollback > 0 {
				return meta.Scrollback
			}
		}
	}
	return config.DefaultScrollbackLines
}

func (m *RootModel) resize() {
	statusBarHeight := 1
	sidebarWidth := m.sidebarWidth()
	mainWidth := m.width - sidebarWidth
	paneHeight := m.height - statusBarHeight

	m.sidebar.SetSize(sidebarWidth-2, paneHeight-2)
	m.mainPane.SetSize(mainWidth-2, paneHeight-2)
	m.statusBar.SetWidth(m.width)

	if m.activeWindowName != "" {
		tw := mainWidth - 3  // border(2) + scrollbar(1)
		th := paneHeight - 3 // border(2) + title(1)
		if tw > 0 && th > 0 {
			select {
			case m.resizeSem <- struct{}{}:
				wn := m.activeWindowName
				go func() {
					defer func() { <-m.resizeSem }()
					_ = m.tmux.ResizeWindow(wn, tw, th)
				}()
			default:
				// A resize is already in flight; skip this one.
			}
		}
	}
}

func (m *RootModel) sidebarWidth() int {
	if m.cfg.UIPrefs.SidebarWidth > 0 {
		w := m.cfg.UIPrefs.SidebarWidth
		if w < 16 {
			w = 16
		}
		if w > 60 {
			w = 60
		}
		// Don't let sidebar consume the entire terminal.
		if m.width-w < 20 {
			w = m.width - 20
		}
		if w < 16 {
			w = 16
		}
		return w
	}
	w := m.width / 4
	if w < 20 {
		w = 20
	}
	if w > 40 {
		w = 40
	}
	return w
}

// screenToContent maps a screen (X, Y) coordinate to a content (row, col)
// coordinate within the main pane's viewport content buffer.
//
// Layout (0-indexed screen rows/cols):
//   col 0..sidebarW-1        : sidebar (border inclusive)
//   col sidebarW             : main pane left border
//   col sidebarW+1..          : main pane content
//   row 0                    : top border of both panes
//   row 1                    : main pane title bar
//   row 2..height-2           : viewport content rows
func (m *RootModel) screenToContent(screenX, screenY int) (row, col int) {
	sidebarW := m.sidebarWidth()
	col = screenX - sidebarW - 1
	row = (screenY - 2) + m.mainPane.viewport.YOffset
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	return row, col
}

func (m *RootModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode == ModeDialog {
		return m, nil
	}

	sidebarW := m.sidebarWidth()

	// Left-button press: begin selection in main pane, or click in sidebar.
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		if msg.X < sidebarW {
			m.mainPane.ClearSelection()
			m.focus = SidebarFocused
			m.mode = ModeNormal
			m.statusBar.SetMode(m.mode)
			idx := m.sidebar.IndexByClick(msg.Y)
			if idx >= 0 {
				m.sidebar.SelectIndex(idx)
				sel := m.sidebar.SelectedSession()
				if sel != nil {
					return m, m.openWindow(sel.ID, sel.Name, sel.IsRunning)
				}
			}
			return m, nil
		} else if m.mainPane.hasSession {
			row, col := m.screenToContent(msg.X, msg.Y)
			m.mainPane.StartSelection(row, col)
			return m, nil
		}
		return m, nil
	}

	// Mouse drag: update selection endpoint.
	if msg.Action == tea.MouseActionMotion && m.mainPane.selection.Active {
		row, col := m.screenToContent(msg.X, msg.Y)
		m.mainPane.UpdateSelection(row, col)
		return m, nil
	}

	// Left-button release: finalise selection or fall back to INSERT mode click.
	if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft && m.mainPane.selection.Active {
		sel := m.mainPane.selection
		isClick := sel.StartRow == sel.EndRow && sel.StartCol == sel.EndCol
		if !isClick {
			if text := m.mainPane.SelectedText(); text != "" {
				_ = clipboard.WriteAll(text)
			}
			m.mainPane.ClearSelection()
		} else {
			// Plain click with no drag → enter INSERT mode as before.
			m.mainPane.ClearSelection()
			m.focus = MainFocused
			m.mode = ModeInsert
			m.statusBar.SetMode(m.mode)
		}
		return m, nil
	}

	// Forward scroll and other events to the appropriate pane.
	var cmd tea.Cmd
	if msg.X < sidebarW {
		m.sidebar, cmd = m.sidebar.Update(msg)
	} else {
		m.mainPane, cmd = m.mainPane.Update(msg)
	}
	return m, cmd
}

func (m *RootModel) View() string {
	if m.width == 0 {
		return ""
	}

	sidebarWidth := m.sidebarWidth()
	mainWidth := m.width - sidebarWidth
	statusBarHeight := 1
	paneHeight := m.height - statusBarHeight

	sidebarBorder := m.styles.InactiveBorder
	mainBorder := m.styles.InactiveBorder
	if m.focus == SidebarFocused {
		sidebarBorder = m.styles.ActiveBorder
	} else {
		mainBorder = m.styles.ActiveBorder
	}

	sidebarView := sidebarBorder.
		Width(sidebarWidth - 2).
		Height(paneHeight - 2).
		Render(m.sidebar.View())

	mainView := mainBorder.
		Width(mainWidth - 2).
		Height(paneHeight - 2).
		Render(m.mainPane.View())

	panes := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, mainView)
	statusBar := m.statusBar.View()

	base := lipgloss.JoinVertical(lipgloss.Left, panes, statusBar)

	if m.dialog.IsVisible() {
		return m.dialog.View(m.width, m.height)
	}

	return base
}

