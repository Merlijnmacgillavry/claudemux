package session

import (
	"time"

	"github.com/merlijnmacgillavry/claudemux/internal/config"
)

// Status represents the running state of a session.
type Status int

const (
	StatusStopped Status = iota
	StatusRunning
	StatusStale
)

// SessionInfo holds metadata about a session shown in the sidebar.
type SessionInfo struct {
	ID         string // tmux window name (stable key)
	Name       string // user-visible display name
	Project    string // working directory
	LastActive time.Time
	Status     Status
}

// Store manages session metadata persistence.
type Store struct {
	cfg config.Config
}

// NewStore creates a Store backed by the given config.
func NewStore(cfg config.Config) *Store {
	return &Store{cfg: cfg}
}

// SetWindow stores or updates metadata for a tmux window.
func (s *Store) SetWindow(windowName, displayName, cwd string) {
	meta := s.cfg.Windows[windowName]
	if displayName != "" {
		meta.DisplayName = displayName
	}
	if cwd != "" {
		meta.WorkingDir = cwd
	}
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}
	s.cfg.Windows[windowName] = meta
}

// GetWindow retrieves metadata for a tmux window.
func (s *Store) GetWindow(windowName string) (config.WindowMeta, bool) {
	meta, ok := s.cfg.Windows[windowName]
	return meta, ok
}

// SetClaudeSessionID updates the Claude session ID for an existing window.
func (s *Store) SetClaudeSessionID(windowName, sessionID string) {
	meta := s.cfg.Windows[windowName]
	meta.ClaudeSessionID = sessionID
	s.cfg.Windows[windowName] = meta
}

// RemoveWindow deletes a window's metadata from the store.
func (s *Store) RemoveWindow(windowName string) {
	delete(s.cfg.Windows, windowName)
}

// AllWindows returns all stored window metadata.
func (s *Store) AllWindows() map[string]config.WindowMeta {
	return s.cfg.Windows
}

// SetScrollback updates the scrollback line count for an existing window.
func (s *Store) SetScrollback(windowName string, lines int) {
	meta := s.cfg.Windows[windowName]
	meta.Scrollback = lines
	s.cfg.Windows[windowName] = meta
}

// SetSkipPermissions updates the --dangerously-skip-permissions flag for an existing window.
func (s *Store) SetSkipPermissions(windowName string, skip bool) {
	meta := s.cfg.Windows[windowName]
	meta.SkipPermissions = skip
	s.cfg.Windows[windowName] = meta
}

// SetUIPrefs updates the UI preferences in the store's config so that saving
// through GetConfig() never overwrites window metadata with stale data.
func (s *Store) SetUIPrefs(prefs config.UIPrefs) {
	s.cfg.UIPrefs = prefs
}

// GetConfig returns the underlying config (for saving).
func (s *Store) GetConfig() config.Config {
	return s.cfg
}
