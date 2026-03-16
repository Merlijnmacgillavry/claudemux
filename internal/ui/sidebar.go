package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SessionItem represents a session in the sidebar list.
type SessionItem struct {
	ID              string
	Name            string
	Preview         string
	LastActive      time.Time
	IsRunning       bool
	WorkingDir      string
	WaitsForInput   bool
}

func (s SessionItem) FilterValue() string { return s.Name + " " + s.Preview }
func (s SessionItem) Title() string       { return s.Name }
func (s SessionItem) Description() string { return s.Preview }

// SessionDelegate renders a session list item.
type SessionDelegate struct {
	styles Styles
}

func (d SessionDelegate) Height() int                              { return 2 }
func (d SessionDelegate) Spacing() int                            { return 1 }
func (d SessionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d SessionDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	s, ok := item.(SessionItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	availWidth := m.Width()

	icon := d.styles.StoppedIcon.Render("○")
	if s.IsRunning {
		icon = d.styles.RunningIcon.Render("●")
	}

	name := s.Name
	if s.WaitsForInput {
		name = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("! ") + name
	}
	preview := truncate(s.Preview, 50)
	timeStr := relativeTime(s.LastActive)

	var line1, line2 string
	if isSelected {
		selBg := d.styles.SelectedItem.GetBackground()
		bg := lipgloss.NewStyle().Background(selBg).Width(availWidth - 1)
		boldName := d.styles.SessionName.Copy().Background(selBg).Bold(true).Render(name)
		accent := d.styles.AccentBar.Render("▎")
		line1 = accent + bg.Render(fmt.Sprintf("%s %s", icon, boldName))
		line2 = " " + bg.Render(fmt.Sprintf("  %s %s",
			d.styles.SessionPreview.Copy().Background(selBg).Render(preview),
			d.styles.Timestamp.Copy().Background(selBg).Render(timeStr),
		))
	} else {
		bg := lipgloss.NewStyle().Width(availWidth - 1)
		line1 = " " + bg.Render(fmt.Sprintf("%s %s", icon, d.styles.NormalItem.Render(name)))
		line2 = " " + bg.Render(fmt.Sprintf("  %s %s",
			d.styles.SessionPreview.Render(preview),
			d.styles.Timestamp.Render(timeStr),
		))
	}

	fmt.Fprintf(w, "%s\n%s", line1, line2)
}

// SidebarModel is the sidebar panel displaying the session list.
type SidebarModel struct {
	list        list.Model
	width       int
	height      int
	styles      Styles
	renameInput textinput.Model
	renaming    bool
	renameID    string
}

func NewSidebarModel(styles Styles) SidebarModel {
	delegate := SessionDelegate{styles: styles}
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "claudemux"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = styles.Title

	ri := textinput.New()
	ri.Prompt = "Rename: "
	ri.CharLimit = 60

	return SidebarModel{
		list:        l,
		styles:      styles,
		renameInput: ri,
	}
}

func (s *SidebarModel) SetSize(w, h int) {
	s.width = w
	s.height = h
	s.list.SetSize(w, h-1) // reserve 1 row for pane-switch hint
}

func (s *SidebarModel) SetSessions(sessions []SessionItem) {
	items := make([]list.Item, len(sessions))
	for i, sess := range sessions {
		items[i] = sess
	}
	s.list.SetItems(items)
}

// SetWaiting updates the WaitsForInput flag for the given window ID.
func (s *SidebarModel) SetWaiting(windowID string, waiting bool) {
	items := s.list.Items()
	for i, item := range items {
		sess, ok := item.(SessionItem)
		if !ok {
			continue
		}
		if sess.ID == windowID && sess.WaitsForInput != waiting {
			sess.WaitsForInput = waiting
			s.list.SetItem(i, sess)
			break
		}
	}
}

// IndexByClick maps a screen-absolute Y coordinate to a list item index.
// Layout: row 0 = top border, row 1 = list title, rows 2+ = items (2 rows each).
// Returns -1 if the click falls outside the item area.
func (s *SidebarModel) IndexByClick(screenY int) int {
	idx := (screenY - 2) / 3
	if idx < 0 || idx >= len(s.list.Items()) {
		return -1
	}
	return idx
}

// SelectIndex moves the list cursor to the given item index.
func (s *SidebarModel) SelectIndex(idx int) {
	s.list.Select(idx)
}

func (s *SidebarModel) SelectedSession() *SessionItem {
	item := s.list.SelectedItem()
	if item == nil {
		return nil
	}
	sess, ok := item.(SessionItem)
	if !ok {
		return nil
	}
	return &sess
}

func (s *SidebarModel) StartRename() {
	sel := s.SelectedSession()
	if sel == nil {
		return
	}
	s.renaming = true
	s.renameID = sel.ID
	s.renameInput.SetValue(sel.Name)
	s.renameInput.Focus()
}

func (s *SidebarModel) CancelRename() {
	s.renaming = false
	s.renameID = ""
	s.renameInput.Blur()
}

func (s *SidebarModel) FinishRename() (id, name string) {
	id = s.renameID
	name = s.renameInput.Value()
	s.renaming = false
	s.renameID = ""
	s.renameInput.Blur()
	return id, name
}

func (s SidebarModel) IsRenaming() bool {
	return s.renaming
}

// IsFiltering reports whether the sidebar list is currently in filter-input mode.
func (s SidebarModel) IsFiltering() bool {
	return s.list.FilterState() == list.Filtering
}

func (s SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	var cmd tea.Cmd
	if s.renaming {
		s.renameInput, cmd = s.renameInput.Update(msg)
		return s, cmd
	}
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s SidebarModel) View() string {
	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render("alt+l: open  alt+h: back")

	if s.renaming {
		return lipgloss.JoinVertical(lipgloss.Left,
			s.list.View(),
			s.renameInput.View(),
			hint,
		)
	}
	if len(s.list.Items()) == 0 {
		empty := s.styles.Timestamp.Copy().
			Padding(1, 2).
			Render("No sessions.\nPress n to create one.")
		title := s.styles.Title.Padding(0, 1).Render("claudemux")
		return lipgloss.JoinVertical(lipgloss.Left, title, empty, hint)
	}
	return lipgloss.JoinVertical(lipgloss.Left, s.list.View(), hint)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}
