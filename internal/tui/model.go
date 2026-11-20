package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/harw/yomiterm/internal/cjk"
	"github.com/harw/yomiterm/internal/config"
	"github.com/harw/yomiterm/internal/lookup"
	"github.com/mattn/go-runewidth"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModePopup
	ModeSearch
)

type autoScanMsg struct {
	seq int
}

type Model struct {
	FileName string
	RawMD    string
	Doc      *Document
	Engine   *lookup.Engine
	Cfg      *config.Config
	Cond     *runewidth.Condition
	Styles   Styles

	Width  int
	Height int

	Mode       Mode
	CursorRow  int
	CursorIdx  int
	DesiredCol int
	TopRow     int

	PendingG bool
	AutoScan bool
	ScanSeq  int
	ShowHelp bool

	// Highlight span on CursorRow (grapheme indices)
	HLStart int
	HLEnd   int

	Popups        []PopupState
	MaxPopupDepth int

	// Search state
	SearchQuery   string
	SearchMatches [][2]int // (row, graphemeIdx)
	SearchPos     int
	StatusMsg     string
}

// NewModel creates the Bubble Tea TUI model for reading a Markdown file.
func NewModel(fileName, rawMD string, engine *lookup.Engine, cfg *config.Config) (*Model, error) {
	cond := cjk.NewCondition(cfg.EastAsianAmbiguousWidth)
	doc, err := RenderDocument(rawMD, 80, cfg.Theme, cond)
	if err != nil {
		return nil, err
	}
	maxDepth := cfg.MaxPopupDepth
	if maxDepth < 1 {
		maxDepth = 5
	}
	return &Model{
		FileName:      fileName,
		RawMD:         rawMD,
		Doc:           doc,
		Engine:        engine,
		Cfg:           cfg,
		Cond:          cond,
		Styles:        DefaultStyles(),
		Width:         80,
		Height:        24,
		AutoScan:      cfg.AutoScan,
		HLStart:       -1,
		HLEnd:         -1,
		MaxPopupDepth: maxDepth,
	}, nil
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		wrapW := m.Width - 2
		if wrapW < 20 {
			wrapW = 20
		}
		if doc, err := RenderDocument(m.RawMD, wrapW, m.Cfg.Theme, m.Cond); err == nil {
			m.Doc = doc
			m.clampCursor()
			m.ensureVisible()
		}
		return m, nil

	case autoScanMsg:
		if m.AutoScan && msg.seq == m.ScanSeq && m.Mode == ModeNormal {
			m.triggerLookup(false)
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.Mode {
		case ModeSearch:
			return m.handleSearchKey(msg)
		case ModePopup:
			return m.handlePopupKey(key)
		default:
			return m.handleNormalKey(key)
		}
	}
	return m, nil
}

func (m *Model) handleNormalKey(key string) (tea.Model, tea.Cmd) {
	m.StatusMsg = ""

	if key == "g" {
		if m.PendingG {
			m.PendingG = false
			m.CursorRow = 0
			m.CursorIdx = 0
			m.DesiredCol = 0
			m.clearHighlight()
			m.ensureVisible()
			return m, m.scheduleAutoScan()
		}
		m.PendingG = true
		return m, nil
	}
	m.PendingG = false

	switch key {
	case "q":
		if len(m.Popups) > 0 && !m.AutoScan {
			m.closeAllPopups()
			return m, nil
		}
		return m, tea.Quit
	case "esc":
		m.closeAllPopups()
		return m, nil
	case "?":
		m.ShowHelp = !m.ShowHelp
		return m, nil
	case "a":
		m.AutoScan = !m.AutoScan
		if m.AutoScan {
			m.StatusMsg = "Auto-Scan ON"
			m.triggerLookup(false)
		} else {
			m.StatusMsg = "Auto-Scan OFF"
			m.closeAllPopups()
		}
		return m, nil
	case "enter", " ":
		m.triggerLookup(true)
		return m, nil
	case "/":
		m.Mode = ModeSearch
		m.SearchQuery = ""
		return m, nil
	case "n":
		m.jumpSearch(1)
		return m, m.scheduleAutoScan()
	case "N":
		m.jumpSearch(-1)
		return m, m.scheduleAutoScan()
	case "h", "left":
		if m.CursorIdx > 0 {
			m.CursorIdx--
			m.updateDesiredCol()
			m.clearHighlight()
			return m, m.scheduleAutoScan()
		}
	case "l", "right":
		toks := m.Doc.Lines[m.CursorRow].Tokens
		if m.CursorIdx+1 < len(toks) {
			m.CursorIdx++
			m.updateDesiredCol()
			m.clearHighlight()
			return m, m.scheduleAutoScan()
		}
	case "j", "down":
		if m.CursorRow+1 < len(m.Doc.Lines) {
			m.CursorRow++
			m.CursorIdx = cjk.NearestGraphemeAtCol(m.Doc.Lines[m.CursorRow], m.DesiredCol)
			m.clearHighlight()
			m.ensureVisible()
			return m, m.scheduleAutoScan()
		}
	case "k", "up":
		if m.CursorRow > 0 {
			m.CursorRow--
			m.CursorIdx = cjk.NearestGraphemeAtCol(m.Doc.Lines[m.CursorRow], m.DesiredCol)
			m.clearHighlight()
			m.ensureVisible()
			return m, m.scheduleAutoScan()
		}
	case "w":
		r, i := m.Doc.NextWord(m.CursorRow, m.CursorIdx)
		m.CursorRow, m.CursorIdx = r, i
		m.updateDesiredCol()
		m.clearHighlight()
		m.ensureVisible()
		return m, m.scheduleAutoScan()
	case "b":
		r, i := m.Doc.PrevWord(m.CursorRow, m.CursorIdx)
		m.CursorRow, m.CursorIdx = r, i
		m.updateDesiredCol()
		m.clearHighlight()
		m.ensureVisible()
		return m, m.scheduleAutoScan()
	case "0":
		m.CursorIdx = 0
		m.DesiredCol = 0
		m.clearHighlight()
		return m, m.scheduleAutoScan()
	case "$":
		toks := m.Doc.Lines[m.CursorRow].Tokens
		if len(toks) > 0 {
			m.CursorIdx = len(toks) - 1
			m.updateDesiredCol()
		}
		m.clearHighlight()
		return m, m.scheduleAutoScan()
	case "G":
		m.CursorRow = len(m.Doc.Lines) - 1
		m.CursorIdx = cjk.NearestGraphemeAtCol(m.Doc.Lines[m.CursorRow], m.DesiredCol)
		m.clearHighlight()
		m.ensureVisible()
		return m, m.scheduleAutoScan()
	case "ctrl+d":
		step := maxInt(1, m.viewportHeight()/2)
		m.CursorRow = minInt(len(m.Doc.Lines)-1, m.CursorRow+step)
		m.CursorIdx = cjk.NearestGraphemeAtCol(m.Doc.Lines[m.CursorRow], m.DesiredCol)
		m.clearHighlight()
		m.ensureVisible()
		return m, m.scheduleAutoScan()
	case "ctrl+u":
		step := maxInt(1, m.viewportHeight()/2)
		m.CursorRow = maxInt(0, m.CursorRow-step)
		m.CursorIdx = cjk.NearestGraphemeAtCol(m.Doc.Lines[m.CursorRow], m.DesiredCol)
		m.clearHighlight()
		m.ensureVisible()
		return m, m.scheduleAutoScan()
	}
	return m, nil
}

func (m *Model) handlePopupKey(key string) (tea.Model, tea.Cmd) {
	if len(m.Popups) == 0 {
		m.Mode = ModeNormal
		return m, nil
	}
	m.StatusMsg = ""
	top := &m.Popups[len(m.Popups)-1]

	switch key {
	case "esc":
		if len(m.Popups) > 1 {
			// Pop topmost child popup and return focus to its parent
			m.Popups = m.Popups[:len(m.Popups)-1]
			m.Popups[len(m.Popups)-1].ClearHighlight()
			return m, nil
		}
		if m.AutoScan {
			m.Mode = ModeNormal
		} else {
			m.closeAllPopups()
		}
		return m, nil
	case "q":
		if m.AutoScan && len(m.Popups) == 1 {
			m.Mode = ModeNormal
		} else {
			m.closeAllPopups()
		}
		return m, nil
	case "enter", " ":
		m.triggerChildLookup()
		return m, nil
	case "tab":
		if top.Tab == TabTerms {
			top.Tab = TabKanji
		} else {
			top.Tab = TabTerms
		}
		top.Scroll = 0
		top.CursorRow = 0
		top.CursorIdx = 0
		top.DesiredCol = 0
		top.ClearHighlight()
		top.BodyLines = nil
	case "e":
		top.ShowExamples = !top.ShowExamples
		top.BodyLines = nil
	case "h", "left":
		if top.CursorIdx > 0 {
			top.CursorIdx--
			top.UpdateDesiredCol()
			top.ClearHighlight()
		}
	case "l", "right":
		if top.CursorRow < len(top.BodyLines) {
			toks := top.BodyLines[top.CursorRow].Tokens
			if top.CursorIdx+1 < len(toks) {
				top.CursorIdx++
				top.UpdateDesiredCol()
				top.ClearHighlight()
			}
		}
	case "j", "down":
		if top.CursorRow+1 < len(top.BodyLines) {
			top.CursorRow++
			top.CursorIdx = cjk.NearestGraphemeAtCol(top.BodyLines[top.CursorRow], top.DesiredCol)
			top.ClearHighlight()
			top.EnsureCursorVisible()
		}
	case "k", "up":
		if top.CursorRow > 0 {
			top.CursorRow--
			top.CursorIdx = cjk.NearestGraphemeAtCol(top.BodyLines[top.CursorRow], top.DesiredCol)
			top.ClearHighlight()
			top.EnsureCursorVisible()
		}
	case "w":
		top.NextWord()
		top.ClearHighlight()
		top.EnsureCursorVisible()
	case "b":
		top.PrevWord()
		top.ClearHighlight()
		top.EnsureCursorVisible()
	case "0":
		top.CursorIdx = 0
		top.DesiredCol = 0
		top.ClearHighlight()
	case "$":
		if top.CursorRow < len(top.BodyLines) {
			toks := top.BodyLines[top.CursorRow].Tokens
			if len(toks) > 0 {
				top.CursorIdx = len(toks) - 1
				top.UpdateDesiredCol()
			}
		}
		top.ClearHighlight()
	case "J":
		top.Scroll++
		if top.CursorRow < top.Scroll && top.Scroll < len(top.BodyLines) {
			top.CursorRow = top.Scroll
			top.ClampCursor()
		}
	case "K":
		if top.Scroll > 0 {
			top.Scroll--
		}
	case "ctrl+d":
		top.Scroll += 5
		if top.CursorRow < top.Scroll && top.Scroll < len(top.BodyLines) {
			top.CursorRow = top.Scroll
			top.ClampCursor()
		}
	case "ctrl+u":
		top.Scroll = maxInt(0, top.Scroll-5)
	}
	return m, nil
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.Mode = ModeNormal
		return m, nil
	case tea.KeyEnter:
		m.Mode = ModeNormal
		m.runSearch()
		return m, m.scheduleAutoScan()
	case tea.KeyBackspace:
		runes := []rune(m.SearchQuery)
		if len(runes) > 0 {
			m.SearchQuery = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.SearchQuery += string(msg.Runes)
		}
	}
	return m, nil
}

func (m *Model) runSearch() {
	m.SearchMatches = nil
	m.SearchPos = 0
	if m.SearchQuery == "" {
		return
	}
	q := strings.ToLower(m.SearchQuery)
	for r, vl := range m.Doc.Lines {
		low := strings.ToLower(vl.PlainLine)
		offset := 0
		for {
			idx := strings.Index(low[offset:], q)
			if idx < 0 {
				break
			}
			bytePos := offset + idx
			gIdx := 0
			for gi, tok := range vl.Tokens {
				if tok.PlainByteStart >= bytePos {
					gIdx = gi
					break
				}
			}
			m.SearchMatches = append(m.SearchMatches, [2]int{r, gIdx})
			offset = bytePos + len(q)
		}
	}
	if len(m.SearchMatches) > 0 {
		m.CursorRow = m.SearchMatches[0][0]
		m.CursorIdx = m.SearchMatches[0][1]
		m.updateDesiredCol()
		m.ensureVisible()
		m.StatusMsg = fmt.Sprintf("Match 1/%d", len(m.SearchMatches))
	} else {
		m.StatusMsg = fmt.Sprintf("Not found: %s", m.SearchQuery)
	}
}

func (m *Model) jumpSearch(delta int) {
	if len(m.SearchMatches) == 0 {
		return
	}
	m.SearchPos = (m.SearchPos + delta + len(m.SearchMatches)) % len(m.SearchMatches)
	match := m.SearchMatches[m.SearchPos]
	m.CursorRow = match[0]
	m.CursorIdx = match[1]
	m.updateDesiredCol()
	m.ensureVisible()
	m.StatusMsg = fmt.Sprintf("Match %d/%d", m.SearchPos+1, len(m.SearchMatches))
}

func (m *Model) triggerLookup(focusPopup bool) {
	slice := m.Doc.SliceFromCursor(m.CursorRow, m.CursorIdx, 24)
	if strings.TrimSpace(slice) == "" {
		return
	}
	res, err := m.Engine.Lookup(slice)
	if err != nil {
		m.StatusMsg = "Lookup error: " + err.Error()
		return
	}

	vh := m.viewportHeight()
	cursorScreenRow := m.CursorRow - m.TopRow
	cursorScreenCol := 0
	if m.CursorRow < len(m.Doc.Lines) && m.CursorIdx < len(m.Doc.Lines[m.CursorRow].Tokens) {
		cursorScreenCol = m.Doc.Lines[m.CursorRow].Tokens[m.CursorIdx].StartCol
	}

	p := NewPopupState(res, m.Cfg.ShowExamples, cursorScreenRow, cursorScreenCol)
	pRow, pCol, pW, pH := ComputePopupPosition(cursorScreenRow, cursorScreenCol, m.Width, vh, 0)
	p.LastRow = pRow
	p.LastCol = pCol
	p.BodyH = maxInt(3, pH-4)
	p.RebuildBody(m.Styles, m.Cond, pW)

	m.Popups = []PopupState{p}
	if focusPopup {
		m.Mode = ModePopup
	}
	if res.MatchRuneLen > 0 {
		m.HLStart = m.CursorIdx
		m.HLEnd = m.CursorIdx + res.MatchRuneLen
	} else {
		m.clearHighlight()
	}
}

func (m *Model) triggerChildLookup() {
	if len(m.Popups) == 0 {
		return
	}
	if len(m.Popups) >= m.MaxPopupDepth {
		m.StatusMsg = fmt.Sprintf("Max popup depth (%d) reached", m.MaxPopupDepth)
		return
	}

	parent := &m.Popups[len(m.Popups)-1]
	vh := m.viewportHeight()
	if len(parent.BodyLines) == 0 {
		_, _, pW, _ := ComputePopupPosition(parent.AnchorRow, parent.AnchorCol, m.Width, vh, len(m.Popups)-1)
		parent.RebuildBody(m.Styles, m.Cond, pW)
	}

	slice := parent.SliceFromCursor(24)
	if strings.TrimSpace(slice) == "" {
		return
	}
	res, err := m.Engine.Lookup(slice)
	if err != nil {
		m.StatusMsg = "Lookup error: " + err.Error()
		return
	}
	if len(res.Terms) == 0 && len(res.Kanji) == 0 {
		m.StatusMsg = "No entry at cursor"
		return
	}

	if res.MatchRuneLen > 0 {
		parent.HLStart = parent.CursorIdx
		parent.HLEnd = parent.CursorIdx + res.MatchRuneLen
	} else {
		parent.ClearHighlight()
	}

	anchorRow, anchorCol := parent.CursorScreenPos()
	depth := len(m.Popups)
	child := NewPopupState(res, parent.ShowExamples, anchorRow, anchorCol)
	pRow, pCol, pW, pH := ComputePopupPosition(anchorRow, anchorCol, m.Width, vh, depth)
	child.LastRow = pRow
	child.LastCol = pCol
	child.BodyH = maxInt(3, pH-4)
	child.RebuildBody(m.Styles, m.Cond, pW)

	m.Popups = append(m.Popups, child)
}

func (m *Model) closeAllPopups() {
	m.Popups = nil
	m.Mode = ModeNormal
	m.clearHighlight()
}

func (m *Model) clearHighlight() {
	m.HLStart = -1
	m.HLEnd = -1
}

func (m *Model) scheduleAutoScan() tea.Cmd {
	if !m.AutoScan {
		return nil
	}
	m.ScanSeq++
	seq := m.ScanSeq
	return tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg {
		return autoScanMsg{seq: seq}
	})
}

func (m *Model) updateDesiredCol() {
	toks := m.Doc.Lines[m.CursorRow].Tokens
	if m.CursorIdx >= 0 && m.CursorIdx < len(toks) {
		m.DesiredCol = toks[m.CursorIdx].StartCol
	} else {
		m.DesiredCol = 0
	}
}

func (m *Model) clampCursor() {
	if m.CursorRow >= len(m.Doc.Lines) {
		m.CursorRow = maxInt(0, len(m.Doc.Lines)-1)
	}
	toks := m.Doc.Lines[m.CursorRow].Tokens
	if len(toks) == 0 {
		m.CursorIdx = 0
	} else if m.CursorIdx >= len(toks) {
		m.CursorIdx = len(toks) - 1
	}
}

func (m *Model) viewportHeight() int {
	h := m.Height - 1 // status bar
	if m.ShowHelp {
		h--
	}
	if h < 5 {
		h = 5
	}
	return h
}

func (m *Model) ensureVisible() {
	vh := m.viewportHeight()
	if m.CursorRow < m.TopRow {
		m.TopRow = m.CursorRow
	} else if m.CursorRow >= m.TopRow+vh {
		m.TopRow = m.CursorRow - vh + 1
	}
}

func (m *Model) View() string {
	vh := m.viewportHeight()
	viewLines := make([]string, vh)

	for i := 0; i < vh; i++ {
		docRow := m.TopRow + i
		if docRow >= len(m.Doc.Lines) {
			viewLines[i] = "~"
			continue
		}
		vl := m.Doc.Lines[docRow]
		if docRow == m.CursorRow {
			viewLines[i] = cjk.RenderLineWithHighlight(vl, m.CursorIdx, m.HLStart, m.HLEnd)
		} else {
			viewLines[i] = vl.RawLine
		}
	}

	// Composite floating popup stack (bottom-to-top)
	for i := range m.Popups {
		p := &m.Popups[i]
		if p.Result == nil {
			continue
		}
		anchorRow, anchorCol := p.AnchorRow, p.AnchorCol
		if i == 0 {
			anchorRow = m.CursorRow - m.TopRow
			anchorCol = 0
			if m.CursorRow < len(m.Doc.Lines) && m.CursorIdx < len(m.Doc.Lines[m.CursorRow].Tokens) {
				anchorCol = m.Doc.Lines[m.CursorRow].Tokens[m.CursorIdx].StartCol
			}
			p.AnchorRow = anchorRow
			p.AnchorCol = anchorCol
		}
		pRow, pCol, pW, pH := ComputePopupPosition(anchorRow, anchorCol, m.Width, vh, i)
		p.LastRow = pRow
		p.LastCol = pCol
		active := (i == len(m.Popups)-1) && (m.Mode == ModePopup)
		popupLines := p.RenderLines(m.Styles, m.Cond, pW, pH, i+1, m.MaxPopupDepth, active)
		viewLines = cjk.CompositePopup(m.Cond, viewLines, popupLines, pRow, pCol, m.Width)
	}

	var out strings.Builder
	out.WriteString(strings.Join(viewLines, "\n"))
	out.WriteString("\n")

	if m.ShowHelp {
		out.WriteString(m.Styles.HelpBar.Width(m.Width).Render("hjkl:move  w/b:word  Enter/Space:lookup/dive  a:auto-scan  /:search  Esc:pop  q:quit"))
		out.WriteString("\n")
	}

	if m.Mode == ModeSearch {
		out.WriteString(m.Styles.SearchPrompt.Width(m.Width).Render("/" + m.SearchQuery + "█"))
	} else {
		modeStr := "NORMAL"
		if m.Mode == ModePopup {
			modeStr = fmt.Sprintf("POPUP [%d/%d]", len(m.Popups), m.MaxPopupDepth)
		} else if m.AutoScan {
			modeStr = "AUTO-SCAN"
		}
		badge := m.Styles.StatusBadge.Render("YomiTerm · " + modeStr)
		rightInfo := fmt.Sprintf("%s  Ln %d/%d", m.FileName, m.CursorRow+1, len(m.Doc.Lines))
		if m.StatusMsg != "" {
			rightInfo = m.StatusMsg + "  |  " + rightInfo
		}
		pad := m.Width - lipgloss.Width(badge) - lipgloss.Width(rightInfo) - 2
		if pad < 1 {
			pad = 1
		}
		bar := badge + strings.Repeat(" ", pad) + rightInfo
		out.WriteString(m.Styles.StatusBar.Width(m.Width).Render(bar))
	}

	return out.String()
}
