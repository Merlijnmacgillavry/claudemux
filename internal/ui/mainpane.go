package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Selection tracks the user's click-and-drag text selection within the main pane.
// Coordinates are content-space (row = line index into content, col = visual column).
type Selection struct {
	Active   bool
	StartRow int
	StartCol int
	EndRow   int
	EndCol   int
}

// MainPaneModel is the right panel showing the active session output.
type MainPaneModel struct {
	viewport    viewport.Model
	width       int
	height      int
	styles      Styles
	sessionName string
	hasSession  bool
	content     string
	selection   Selection
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
	m.viewport.Width = w - 1 // leave 1 column for scrollbar
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
	m.selection = Selection{}
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
	data = strings.TrimRight(data, "\n")
	if len(data) > maxContentBytes {
		// Trim to a line boundary to avoid splitting a multi-byte sequence or ANSI code.
		cut := data[len(data)-maxContentBytes:]
		if idx := strings.IndexByte(cut, '\n'); idx >= 0 {
			cut = cut[idx+1:]
		}
		data = cut
	}
	m.content = data
	m.viewport.SetContent(m.content)
	m.viewport.GotoBottom()
}

func (m *MainPaneModel) ClearContent() {
	m.content = ""
	m.viewport.SetContent("")
}

// --- Selection methods ---

// StartSelection begins a new selection at the given content coordinates.
func (m *MainPaneModel) StartSelection(row, col int) {
	m.selection = Selection{Active: true, StartRow: row, StartCol: col, EndRow: row, EndCol: col}
	m.refreshViewport()
}

// UpdateSelection extends the current selection to (row, col).
func (m *MainPaneModel) UpdateSelection(row, col int) {
	if !m.selection.Active {
		return
	}
	m.selection.EndRow = row
	m.selection.EndCol = col
	m.refreshViewport()
}

// ClearSelection removes the active selection and restores plain content.
func (m *MainPaneModel) ClearSelection() {
	if !m.selection.Active {
		return
	}
	m.selection = Selection{}
	m.refreshViewport()
}

// SelectedText returns the plain (ANSI-stripped) text covered by the selection.
func (m *MainPaneModel) SelectedText() string {
	sel := m.selection
	if !sel.Active {
		return ""
	}

	// Normalise so start is always before end.
	startRow, startCol, endRow, endCol := normaliseSelection(sel)

	lines := strings.Split(m.content, "\n")
	if startRow >= len(lines) {
		return ""
	}
	if endRow >= len(lines) {
		endRow = len(lines) - 1
	}

	var parts []string
	for i := startRow; i <= endRow; i++ {
		clean := ansiEscape.ReplaceAllString(lines[i], "")
		runes := []rune(clean)

		sc, ec := 0, len(runes)
		if i == startRow {
			sc = startCol
		}
		if i == endRow {
			ec = endCol
		}
		if sc < 0 {
			sc = 0
		}
		if ec > len(runes) {
			ec = len(runes)
		}
		if sc > len(runes) {
			sc = len(runes)
		}
		parts = append(parts, string(runes[sc:ec]))
	}
	return strings.Join(parts, "\n")
}

// refreshViewport updates the viewport content to reflect the current selection state.
// It preserves the current scroll offset.
func (m *MainPaneModel) refreshViewport() {
	yoff := m.viewport.YOffset
	if m.selection.Active {
		m.viewport.SetContent(m.highlightedContent())
	} else {
		m.viewport.SetContent(m.content)
	}
	m.viewport.YOffset = yoff
}

// highlightedContent returns the content with ANSI reverse-video escape sequences
// injected around the selected range.
func (m *MainPaneModel) highlightedContent() string {
	sel := m.selection
	if !sel.Active {
		return m.content
	}

	startRow, startCol, endRow, endCol := normaliseSelection(sel)

	lines := strings.Split(m.content, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		if i < startRow || i > endRow {
			result[i] = line
			continue
		}
		sc, ec := 0, -1 // ec=-1 means "to end of line"
		if i == startRow {
			sc = startCol
		}
		if i == endRow {
			ec = endCol
		}
		result[i] = injectHighlight(line, sc, ec)
	}
	return strings.Join(result, "\n")
}

// normaliseSelection returns (startRow, startCol, endRow, endCol) with start <= end.
func normaliseSelection(sel Selection) (startRow, startCol, endRow, endCol int) {
	startRow, startCol, endRow, endCol = sel.StartRow, sel.StartCol, sel.EndRow, sel.EndCol
	if startRow > endRow || (startRow == endRow && startCol > endCol) {
		startRow, startCol, endRow, endCol = endRow, endCol, startRow, startCol
	}
	return
}

// injectHighlight inserts \x1b[7m (reverse video) and \x1b[27m (reset reverse)
// around the visual column range [startCol, endCol) in a single ANSI-coded line.
// endCol == -1 means "highlight to the end of the visible content on this line".
func injectHighlight(line string, startCol, endCol int) string {
	var result strings.Builder
	visualCol := 0
	highlightOn := false

	i := 0
	for i < len(line) {
		b := line[i]

		// Toggle highlight state before this position.
		if !highlightOn && visualCol >= startCol && (endCol < 0 || visualCol < endCol) {
			result.WriteString("\x1b[7m")
			highlightOn = true
		} else if highlightOn && endCol >= 0 && visualCol >= endCol {
			result.WriteString("\x1b[27m")
			highlightOn = false
		}

		// ANSI escape sequence — pass through without advancing visual column.
		if b == '\x1b' {
			if i+1 < len(line) && line[i+1] == '[' {
				// CSI sequence: scan to final byte (0x40–0x7E).
				j := i + 2
				for j < len(line) && (line[j] < 0x40 || line[j] > 0x7E) {
					j++
				}
				if j < len(line) {
					j++ // include the final byte
				}
				result.WriteString(line[i:j])
				i = j
			} else {
				// Two-byte escape sequence.
				result.WriteByte(line[i])
				i++
				if i < len(line) {
					result.WriteByte(line[i])
					i++
				}
			}
			continue
		}

		// Regular UTF-8 character — advance visual column.
		_, size := utf8.DecodeRuneInString(line[i:])
		result.WriteString(line[i : i+size])
		visualCol++
		i += size
	}

	if highlightOn {
		result.WriteString("\x1b[27m")
	}

	return result.String()
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
	vpView := m.viewport.View()

	// Build scrollbar alongside the viewport lines.
	vpLines := strings.Split(vpView, "\n")
	visibleLines := m.viewport.Height
	totalLines := strings.Count(m.content, "\n") + 1

	thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF"))
	trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3F3F46"))

	showScrollbar := totalLines > visibleLines
	var thumbStart, thumbEnd int
	if showScrollbar {
		thumbSize := visibleLines * visibleLines / totalLines
		if thumbSize < 1 {
			thumbSize = 1
		}
		scrollable := totalLines - visibleLines
		thumbStart = m.viewport.YOffset * (visibleLines - thumbSize) / scrollable
		thumbEnd = thumbStart + thumbSize
	}

	var sb strings.Builder
	for i, line := range vpLines {
		sb.WriteString(line)
		if showScrollbar && i < visibleLines {
			if i >= thumbStart && i < thumbEnd {
				sb.WriteString(thumbStyle.Render("┃"))
			} else {
				sb.WriteString(trackStyle.Render("│"))
			}
		}
		if i < len(vpLines)-1 {
			sb.WriteByte('\n')
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, sb.String())
}
