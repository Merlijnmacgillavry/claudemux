package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MainPaneModel is the right panel showing the active session output.
type MainPaneModel struct {
	viewport    viewport.Model
	width       int
	height      int
	styles      Styles
	sessionName string
	hasSession  bool
	content     string
}

func NewMainPaneModel(styles Styles) MainPaneModel {
	vp := viewport.New(0, 0)
	vp.SetContent("")
	return MainPaneModel{
		viewport: vp,
		styles:   styles,
	}
}

func (m *MainPaneModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w
	m.viewport.Height = h - 1 // leave room for title
}

func (m *MainPaneModel) SetSession(name string) {
	m.sessionName = name
	m.hasSession = true
}

func (m *MainPaneModel) ClearSession() {
	m.sessionName = ""
	m.hasSession = false
	m.content = ""
	m.viewport.SetContent("")
}

const maxContentBytes = 512 * 1024

func (m *MainPaneModel) AppendContent(data string) {
	m.content += data
	if len(m.content) > maxContentBytes {
		m.content = m.content[len(m.content)-maxContentBytes:]
	}
	m.viewport.SetContent(m.content)
	m.viewport.GotoBottom()
}

func (m *MainPaneModel) SetContent(data string) {
	// capture-pane always appends a trailing newline; strip it.
	m.content = strings.TrimRight(data, "\n")
	m.viewport.SetContent(m.content)
	m.viewport.GotoBottom()
}

func (m *MainPaneModel) ClearContent() {
	m.content = ""
	m.viewport.SetContent("")
}

func (m MainPaneModel) Update(msg tea.Msg) (MainPaneModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m MainPaneModel) View() string {
	if !m.hasSession {
		placeholder := m.styles.Timestamp.Copy().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Select a session from the sidebar\nor press n to create a new one")
		return placeholder
	}

	title := m.styles.Title.Padding(0, 1).Render(m.sessionName)
	return lipgloss.JoinVertical(lipgloss.Left, title, m.viewport.View())
}
