package session

import (
	"sync"
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
	mu  sync.RWMutex
	cfg config.Config
}

// NewStore creates a Store backed by the given config.
func NewStore(cfg config.Config) *Store {
	return &Store{cfg: cfg}
}

// SetWindow stores or updates metadata for a tmux window.
func (s *Store) SetWindow(windowName, displayName, cwd string) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.cfg.Windows[windowName]
	return meta, ok
}

// SetClaudeSessionID updates the Claude session ID for an existing window.
func (s *Store) SetClaudeSessionID(windowName, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := s.cfg.Windows[windowName]
	meta.ClaudeSessionID = sessionID
	s.cfg.Windows[windowName] = meta
}

// RemoveWindow deletes a window's metadata from the store.
func (s *Store) RemoveWindow(windowName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cfg.Windows, windowName)
}

// AllWindows returns a snapshot of all stored window metadata.
// Returns a copy so callers can safely iterate without holding the lock.
func (s *Store) AllWindows() map[string]config.WindowMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]config.WindowMeta, len(s.cfg.Windows))
	for k, v := range s.cfg.Windows {
		cp[k] = v
	}
	return cp
}

// SetScrollback updates the scrollback line count for an existing window.
func (s *Store) SetScrollback(windowName string, lines int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := s.cfg.Windows[windowName]
	meta.Scrollback = lines
	s.cfg.Windows[windowName] = meta
}

// SetSkipPermissions updates the --dangerously-skip-permissions flag for an existing window.
func (s *Store) SetSkipPermissions(windowName string, skip bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := s.cfg.Windows[windowName]
	meta.SkipPermissions = skip
	s.cfg.Windows[windowName] = meta
}

// SetUIPrefs updates the UI preferences in the store's config so that saving
// through GetConfig() never overwrites window metadata with stale data.
func (s *Store) SetUIPrefs(prefs config.UIPrefs) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.UIPrefs = prefs
}

// GetConfig returns a copy of the underlying config (for saving).
func (s *Store) GetConfig() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// GetRecentDirs returns a copy of the recent-directories list.
func (s *Store) GetRecentDirs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]string, len(s.cfg.RecentDirs))
	copy(cp, s.cfg.RecentDirs)
	return cp
}

// AddRecentDir prepends dir to the recent-directories list, deduplicates, and
// caps it at 10 entries. No-op for empty strings.
func (s *Store) AddRecentDir(dir string) {
	if dir == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	const maxRecent = 10
	next := make([]string, 0, maxRecent)
	next = append(next, dir)
	for _, d := range s.cfg.RecentDirs {
		if d != dir {
			next = append(next, d)
		}
	}
	if len(next) > maxRecent {
		next = next[:maxRecent]
	}
	s.cfg.RecentDirs = next
}
