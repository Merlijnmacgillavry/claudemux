package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
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

// colRange is a [start, end) visual-column span within a single content line.
type colRange struct{ start, end int }

// searchMatchLine records the matching column ranges for one content line.
type searchMatchLine struct {
	lineIdx int
	ranges  []colRange
}

// MainPaneModel is the right panel showing the active session output.
type MainPaneModel struct {
	viewport    viewport.Model
	width       int
	height      int
	styles      Styles
	sessionName string
	hasSession  bool
	gitBranch   string
	content     string
	totalLines  int // cached line count; updated with content
	selection   Selection
	thumbStyle  lipgloss.Style
	trackStyle  lipgloss.Style

	// search state
	searchInput      textinput.Model
	searchActive     bool
	searchMatchLines []searchMatchLine
	searchCurrent    int // index into searchMatchLines
}

func NewMainPaneModel(styles Styles) MainPaneModel {
	vp := viewport.New(0, 0)
	vp.SetContent("")

	si := textinput.New()
	si.Placeholder = "search…"
	si.Prompt = ""
	si.CharLimit = 120

	return MainPaneModel{
		viewport:   vp,
		styles:     styles,
		totalLines: 1,
		thumbStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")),
		trackStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#3F3F46")),
		searchInput: si,
	}
}

func (m *MainPaneModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w - 1 // leave 1 column for scrollbar
	if m.searchActive {
		m.viewport.Height = h - 2 // title row + search bar
	} else {
		m.viewport.Height = h - 1 // title row only
	}
}

func (m *MainPaneModel) SetSession(name string) {
	m.sessionName = name
	m.hasSession = true
	m.gitBranch = ""
}

func (m *MainPaneModel) SetGitBranch(branch string) {
	m.gitBranch = branch
}

func (m *MainPaneModel) AtBottom() bool {
	return m.viewport.YOffset >= max(0, m.totalLines-m.viewport.Height)
}

func (m *MainPaneModel) ClearSession() {
	m.sessionName = ""
	m.hasSession = false
	m.gitBranch = ""
	m.content = ""
	m.totalLines = 1
	m.selection = Selection{}
	m.viewport.SetContent("")
	m.StopSearch()
}

const maxContentBytes = 512 * 1024

func (m *MainPaneModel) AppendContent(data string) {
	m.content += data
	if len(m.content) > maxContentBytes {
		m.content = m.content[len(m.content)-maxContentBytes:]
	}
	m.totalLines = strings.Count(m.content, "\n") + 1
	m.viewport.SetContent(m.searchHighlightedContent())
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
	// Freeze content while the user is dragging a selection — updating the
	// buffer shifts line coordinates and breaks the highlight.
	if m.selection.Active {
		return
	}
	atBottom := m.viewport.YOffset >= max(0, m.totalLines-m.viewport.Height)
	savedYOffset := m.viewport.YOffset
	m.content = data
	m.totalLines = strings.Count(m.content, "\n") + 1
	if m.searchActive {
		m.rebuildSearchMatches()
	}
	m.viewport.SetContent(m.searchHighlightedContent())
	if atBottom {
		m.viewport.GotoBottom()
	} else {
		maxY := max(0, m.totalLines-m.viewport.Height)
		if savedYOffset > maxY {
			savedYOffset = maxY
		}
		m.viewport.YOffset = savedYOffset
	}
}

func (m *MainPaneModel) ClearContent() {
	m.content = ""
	m.totalLines = 1
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

// refreshViewport updates the viewport content to reflect the current selection/search state.
func (m *MainPaneModel) refreshViewport() {
	yoff := m.viewport.YOffset
	if m.selection.Active {
		m.viewport.SetContent(m.highlightedContent())
	} else {
		m.viewport.SetContent(m.searchHighlightedContent())
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

// --- Search methods ---

// IsSearching reports whether the inline search bar is active.
func (m *MainPaneModel) IsSearching() bool { return m.searchActive }

// SearchMatchCount returns the number of matching lines.
func (m *MainPaneModel) SearchMatchCount() int { return len(m.searchMatchLines) }

// StartSearch opens the inline search bar and focuses its input.
func (m *MainPaneModel) StartSearch() {
	m.searchActive = true
	m.searchCurrent = 0
	m.viewport.Height = m.height - 2
	m.searchInput.SetValue("")
	m.searchInput.Focus()
	m.searchMatchLines = nil
}

// StopSearch closes the search bar and restores the viewport.
func (m *MainPaneModel) StopSearch() {
	if !m.searchActive {
		return
	}
	m.searchActive = false
	m.searchMatchLines = nil
	m.searchCurrent = 0
	m.searchInput.Blur()
	m.viewport.Height = m.height - 1
	yoff := m.viewport.YOffset
	m.viewport.SetContent(m.content)
	m.viewport.YOffset = yoff
}

// SearchNext advances to the next match and scrolls to it.
func (m *MainPaneModel) SearchNext() {
	if len(m.searchMatchLines) == 0 {
		return
	}
	m.searchCurrent = (m.searchCurrent + 1) % len(m.searchMatchLines)
	m.scrollToCurrentMatch()
	m.refreshViewport()
}

// SearchPrev moves to the previous match and scrolls to it.
func (m *MainPaneModel) SearchPrev() {
	if len(m.searchMatchLines) == 0 {
		return
	}
	m.searchCurrent = (m.searchCurrent - 1 + len(m.searchMatchLines)) % len(m.searchMatchLines)
	m.scrollToCurrentMatch()
	m.refreshViewport()
}

// UpdateSearchInput forwards a key event to the search input and rebuilds matches.
func (m *MainPaneModel) UpdateSearchInput(msg tea.Msg) tea.Cmd {
	prev := m.searchInput.Value()
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	if m.searchInput.Value() != prev {
		m.searchCurrent = 0
		m.rebuildSearchMatches()
		if len(m.searchMatchLines) > 0 {
			m.scrollToCurrentMatch()
		}
		m.refreshViewport()
	}
	return cmd
}

// rebuildSearchMatches scans m.content for the current query and updates searchMatchLines.
func (m *MainPaneModel) rebuildSearchMatches() {
	query := m.searchInput.Value()
	m.searchMatchLines = nil
	if query == "" {
		return
	}
	queryRunes := []rune(strings.ToLower(query))
	qLen := len(queryRunes)
	if qLen == 0 {
		return
	}

	lines := strings.Split(m.content, "\n")
	for lineIdx, line := range lines {
		clean := []rune(strings.ToLower(ansiEscape.ReplaceAllString(line, "")))
		if len(clean) < qLen {
			continue
		}
		var ranges []colRange
		for i := 0; i <= len(clean)-qLen; i++ {
			match := true
			for j := 0; j < qLen; j++ {
				if clean[i+j] != queryRunes[j] {
					match = false
					break
				}
			}
			if match {
				ranges = append(ranges, colRange{i, i + qLen})
				i += qLen - 1 // skip past the match; loop increments by 1
			}
		}
		if len(ranges) > 0 {
			m.searchMatchLines = append(m.searchMatchLines, searchMatchLine{lineIdx: lineIdx, ranges: ranges})
		}
	}
}

// searchHighlightedContent returns m.content with search highlights injected.
// Returns m.content unchanged when search is inactive or has no matches.
func (m *MainPaneModel) searchHighlightedContent() string {
	if !m.searchActive || len(m.searchMatchLines) == 0 {
		return m.content
	}

	currentLineIdx := -1
	if m.searchCurrent >= 0 && m.searchCurrent < len(m.searchMatchLines) {
		currentLineIdx = m.searchMatchLines[m.searchCurrent].lineIdx
	}

	matchByLine := make(map[int]searchMatchLine, len(m.searchMatchLines))
	for _, ml := range m.searchMatchLines {
		matchByLine[ml.lineIdx] = ml
	}

	lines := strings.Split(m.content, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		if ml, ok := matchByLine[i]; ok {
			result[i] = highlightSearchMatches(line, ml.ranges, i == currentLineIdx)
		} else {
			result[i] = line
		}
	}
	return strings.Join(result, "\n")
}

// scrollToCurrentMatch scrolls the viewport so the current match is centred.
func (m *MainPaneModel) scrollToCurrentMatch() {
	if m.searchCurrent < 0 || m.searchCurrent >= len(m.searchMatchLines) {
		return
	}
	target := m.searchMatchLines[m.searchCurrent].lineIdx
	offset := target - m.viewport.Height/2
	if offset < 0 {
		offset = 0
	}
	if maxY := max(0, m.totalLines-m.viewport.Height); offset > maxY {
		offset = maxY
	}
	m.viewport.YOffset = offset
}

// highlightSearchMatches injects background-colour highlight at the given visual-column
// ranges within an ANSI-coded line. Uses a single ANSI-aware forward pass.
func highlightSearchMatches(line string, ranges []colRange, current bool) string {
	if len(ranges) == 0 {
		return line
	}
	const (
		otherBg  = "\x1b[48;5;58m" // dark olive — non-current matches
		curBg    = "\x1b[43m"       // bright yellow — current match
		resetBg  = "\x1b[49m"       // reset background only
	)
	onSeq := otherBg
	if current {
		onSeq = curBg
	}

	var out strings.Builder
	visualCol := 0
	ri := 0    // index of the next range to activate
	inHL := false

	i := 0
	for i < len(line) {
		// Close highlight when we've passed the end of the active range.
		if inHL && ri > 0 && visualCol >= ranges[ri-1].end {
			out.WriteString(resetBg)
			inHL = false
		}
		// Open the next highlight when we reach its start.
		if !inHL && ri < len(ranges) && visualCol >= ranges[ri].start {
			out.WriteString(onSeq)
			inHL = true
			ri++
		}

		b := line[i]
		// ANSI escape — copy verbatim without advancing visual column.
		if b == '\x1b' {
			if i+1 < len(line) && line[i+1] == '[' {
				j := i + 2
				for j < len(line) && (line[j] < 0x40 || line[j] > 0x7E) {
					j++
				}
				if j < len(line) {
					j++
				}
				out.WriteString(line[i:j])
				i = j
			} else {
				out.WriteByte(line[i])
				i++
				if i < len(line) {
					out.WriteByte(line[i])
					i++
				}
			}
			continue
		}

		_, size := utf8.DecodeRuneInString(line[i:])
		out.WriteString(line[i : i+size])
		visualCol++
		i += size
	}
	if inHL {
		out.WriteString(resetBg)
	}
	return out.String()
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

	titleText := m.sessionName
	if m.gitBranch != "" {
		branchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA"))
		titleText += "  " + branchStyle.Render(" "+m.gitBranch)
	}
	title := m.styles.Title.Padding(0, 1).Render(titleText)
	vpView := m.viewport.View()

	// Build scrollbar alongside the viewport lines.
	vpLines := strings.Split(vpView, "\n")
	visibleLines := m.viewport.Height
	totalLines := m.totalLines

	showScrollbar := totalLines > visibleLines
	var thumbStart, thumbEnd int
	if showScrollbar {
		fVis := float64(visibleLines)
		fTot := float64(totalLines)
		thumbSize := int(fVis * fVis / fTot)
		if thumbSize < 1 {
			thumbSize = 1
		}
		atBottom := m.viewport.YOffset >= totalLines-visibleLines
		if atBottom {
			thumbStart = visibleLines - thumbSize
		} else {
			scrollable := fTot - fVis
			thumbStart = int(float64(m.viewport.YOffset) * float64(visibleLines-thumbSize) / scrollable)
		}
		thumbEnd = thumbStart + thumbSize
	}

	var sb strings.Builder
	for i := 0; i < visibleLines; i++ {
		if i < len(vpLines) {
			sb.WriteString(vpLines[i])
		}
		if showScrollbar {
			if i >= thumbStart && i < thumbEnd {
				sb.WriteString(m.thumbStyle.Render("┃"))
			} else {
				sb.WriteString(m.trackStyle.Render("│"))
			}
		}
		if i < visibleLines-1 {
			sb.WriteByte('\n')
		}
	}

	if m.searchActive {
		count := len(m.searchMatchLines)
		var indicator string
		switch {
		case m.searchInput.Value() == "":
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("type to search")
		case count == 0:
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("no matches")
		default:
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC")).
				Render(fmt.Sprintf("%d/%d", m.searchCurrent+1, count))
		}
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
		searchBar := lipgloss.JoinHorizontal(lipgloss.Center,
			labelStyle.Render(" Find: "),
			m.searchInput.View(),
			labelStyle.Render("  "),
			indicator,
			labelStyle.Render("  enter/n: next  N: prev  esc: close"),
		)
		return lipgloss.JoinVertical(lipgloss.Left, title, sb.String(), searchBar)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, sb.String())
}
