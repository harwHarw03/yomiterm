package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/harw/yomiterm/internal/cjk"
	"github.com/harw/yomiterm/internal/lookup"
	"github.com/mattn/go-runewidth"
)

type PopupTab int

const (
	TabTerms PopupTab = iota
	TabKanji
)

type PopupState struct {
	Tab          PopupTab
	Scroll       int
	ShowExamples bool
	Result       *lookup.LookupResult

	// In-popup CJK cursor & highlight state (for spawning child popups)
	CursorRow  int
	CursorIdx  int
	DesiredCol int
	HLStart    int
	HLEnd      int

	// Parsed visual lines for the current tab & width
	BodyLines []cjk.VisualLine
	BodyW     int
	BodyH     int

	// Screen-space coordinates for cascading child popups
	AnchorRow    int
	AnchorCol    int
	LastRow      int
	LastCol      int
	HeaderHeight int
}

// NewPopupState initializes a PopupState with default highlight cleared.
func NewPopupState(res *lookup.LookupResult, showExamples bool, anchorRow, anchorCol int) PopupState {
	tab := TabTerms
	if res != nil && len(res.Terms) == 0 && len(res.Kanji) > 0 {
		tab = TabKanji
	}
	return PopupState{
		Tab:          tab,
		ShowExamples: showExamples,
		Result:       res,
		HLStart:      -1,
		HLEnd:        -1,
		AnchorRow:    anchorRow,
		AnchorCol:    anchorCol,
	}
}

// RebuildBody re-renders and parses the popup's body lines into CJK VisualLines.
func (p *PopupState) RebuildBody(styles Styles, cond *runewidth.Condition, popupW int) {
	innerW := popupW - 4 // 2 border + 2 padding
	if innerW < 20 {
		innerW = 20
	}

	var rawLines []string
	if p.Tab == TabTerms {
		rawLines = renderTermsBody(p.Result, styles, cond, innerW, p.ShowExamples)
	} else {
		rawLines = renderKanjiBody(p.Result, styles, cond, innerW)
	}
	if len(rawLines) == 0 {
		rawLines = []string{styles.MutedText.Render("No entries found.")}
	}

	vLines := make([]cjk.VisualLine, len(rawLines))
	for i, rl := range rawLines {
		vLines[i] = cjk.ParseANSILine(cond, rl)
	}
	p.BodyLines = vLines
	p.BodyW = innerW
	p.ClampCursor()
}

// ClampCursor keeps CursorRow and CursorIdx within valid BodyLines bounds.
func (p *PopupState) ClampCursor() {
	if len(p.BodyLines) == 0 {
		p.CursorRow = 0
		p.CursorIdx = 0
		return
	}
	if p.CursorRow < 0 {
		p.CursorRow = 0
	}
	if p.CursorRow >= len(p.BodyLines) {
		p.CursorRow = len(p.BodyLines) - 1
	}
	toks := p.BodyLines[p.CursorRow].Tokens
	if len(toks) == 0 {
		p.CursorIdx = 0
	} else if p.CursorIdx >= len(toks) {
		p.CursorIdx = len(toks) - 1
	}
	if p.CursorIdx < 0 {
		p.CursorIdx = 0
	}
}

// UpdateDesiredCol syncs DesiredCol to the current grapheme's visual column.
func (p *PopupState) UpdateDesiredCol() {
	if p.CursorRow < 0 || p.CursorRow >= len(p.BodyLines) {
		p.DesiredCol = 0
		return
	}
	toks := p.BodyLines[p.CursorRow].Tokens
	if p.CursorIdx >= 0 && p.CursorIdx < len(toks) {
		p.DesiredCol = toks[p.CursorIdx].StartCol
	} else {
		p.DesiredCol = 0
	}
}

// ClearHighlight clears the child-spawn matched-span underline.
func (p *PopupState) ClearHighlight() {
	p.HLStart = -1
	p.HLEnd = -1
}

// EnsureCursorVisible adjusts Scroll so CursorRow stays inside the visible body window.
func (p *PopupState) EnsureCursorVisible() {
	bodyH := p.BodyH
	if bodyH < 3 {
		bodyH = 3
	}
	if p.CursorRow < p.Scroll {
		p.Scroll = p.CursorRow
	} else if p.CursorRow >= p.Scroll+bodyH {
		p.Scroll = p.CursorRow - bodyH + 1
	}
	if p.Scroll < 0 {
		p.Scroll = 0
	}
}

// SliceFromCursor returns up to maxRunes of plain text starting at the in-popup cursor.
func (p *PopupState) SliceFromCursor(maxRunes int) string {
	if p.CursorRow < 0 || p.CursorRow >= len(p.BodyLines) {
		return ""
	}
	var b strings.Builder
	count := 0
	for r := p.CursorRow; r < len(p.BodyLines) && count < maxRunes; r++ {
		toks := p.BodyLines[r].Tokens
		start := 0
		if r == p.CursorRow {
			start = p.CursorIdx
		}
		for i := start; i < len(toks) && count < maxRunes; i++ {
			b.WriteString(toks[i].Grapheme)
			count++
		}
		if r == p.CursorRow && len(toks) == 0 {
			break
		}
		// Don't cross separator or blank lines
		if count > 0 && r+1 < len(p.BodyLines) {
			nextPlain := strings.TrimSpace(p.BodyLines[r+1].PlainLine)
			if nextPlain == "" || strings.HasPrefix(nextPlain, "─") {
				break
			}
		}
	}
	return b.String()
}

// CursorScreenPos returns the approximate screen (row, col) of the in-popup cursor.
func (p *PopupState) CursorScreenPos() (int, int) {
	relRow := p.CursorRow - p.Scroll
	if relRow < 0 {
		relRow = 0
	}
	col := 0
	if p.CursorRow >= 0 && p.CursorRow < len(p.BodyLines) {
		toks := p.BodyLines[p.CursorRow].Tokens
		if p.CursorIdx >= 0 && p.CursorIdx < len(toks) {
			col = toks[p.CursorIdx].StartCol
		}
	}
	// +1 for top border, +HeaderHeight for header, +2 for left border & padding
	return p.LastRow + 1 + p.HeaderHeight + relRow, p.LastCol + 2 + col
}

// NextWord moves the in-popup cursor to the next word/token boundary.
func (p *PopupState) NextWord() {
	if p.CursorRow >= len(p.BodyLines) {
		return
	}
	toks := p.BodyLines[p.CursorRow].Tokens
	if p.CursorIdx+1 < len(toks) {
		curClass := charClass(toks[p.CursorIdx].Grapheme)
		i := p.CursorIdx + 1
		if curClass != classSpace {
			for i < len(toks) && charClass(toks[i].Grapheme) == curClass {
				i++
			}
		}
		for i < len(toks) && charClass(toks[i].Grapheme) == classSpace {
			i++
		}
		if i < len(toks) {
			p.CursorIdx = i
			p.UpdateDesiredCol()
			return
		}
	}
	for r := p.CursorRow + 1; r < len(p.BodyLines); r++ {
		for i, t := range p.BodyLines[r].Tokens {
			if charClass(t.Grapheme) != classSpace {
				p.CursorRow = r
				p.CursorIdx = i
				p.UpdateDesiredCol()
				return
			}
		}
	}
}

// PrevWord moves the in-popup cursor to the previous word/token boundary.
func (p *PopupState) PrevWord() {
	if p.CursorRow < 0 || p.CursorRow >= len(p.BodyLines) {
		return
	}
	toks := p.BodyLines[p.CursorRow].Tokens
	i := p.CursorIdx - 1
	for i >= 0 && charClass(toks[i].Grapheme) == classSpace {
		i--
	}
	if i >= 0 {
		cls := charClass(toks[i].Grapheme)
		for i > 0 && charClass(toks[i-1].Grapheme) == cls {
			i--
		}
		p.CursorIdx = i
		p.UpdateDesiredCol()
		return
	}
	for r := p.CursorRow - 1; r >= 0; r-- {
		lt := p.BodyLines[r].Tokens
		j := len(lt) - 1
		for j >= 0 && charClass(lt[j].Grapheme) == classSpace {
			j--
		}
		if j >= 0 {
			cls := charClass(lt[j].Grapheme)
			for j > 0 && charClass(lt[j-1].Grapheme) == cls {
				j--
			}
			p.CursorRow = r
			p.CursorIdx = j
			p.UpdateDesiredCol()
			return
		}
	}
}

func (p *PopupState) RenderLines(styles Styles, cond *runewidth.Condition, popupW, popupH, depth, maxDepth int, active bool) []string {
	innerW := popupW - 4 // 2 border + 2 padding
	if innerW < 20 {
		innerW = 20
	}

	// Ensure BodyLines are built for this width
	if len(p.BodyLines) == 0 || p.BodyW != innerW {
		p.RebuildBody(styles, cond, popupW)
	}

	// Header tabs + depth badge
	termsLabel := fmt.Sprintf("Terms (%d)", len(p.Result.Terms))
	kanjiLabel := fmt.Sprintf("Kanji (%d)", len(p.Result.Kanji))
	var tTab, kTab string
	if p.Tab == TabTerms {
		tTab = styles.TabActive.Render(termsLabel)
		kTab = styles.TabInactive.Render(kanjiLabel)
	} else {
		tTab = styles.TabInactive.Render(termsLabel)
		kTab = styles.TabActive.Render(kanjiLabel)
	}
	depthBadge := ""
	if maxDepth > 1 {
		depthBadge = " " + styles.DepthBadge.Render(fmt.Sprintf("[%d/%d]", depth, maxDepth))
	}
	hint := styles.MutedText.Render(" [Enter:dive] [Tab] [Esc]")
	header := lipgloss.JoinHorizontal(lipgloss.Top, tTab, " ", kTab, depthBadge, hint)
	headerBox := styles.PopupHeader.Width(innerW).Render(header)

	headerLines := strings.Split(headerBox, "\n")
	p.HeaderHeight = len(headerLines)

	bodyH := popupH - 2 - len(headerLines)
	if bodyH < 3 {
		bodyH = 3
	}
	p.BodyH = bodyH

	maxScroll := len(p.BodyLines) - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.Scroll > maxScroll {
		p.Scroll = maxScroll
	}
	if p.Scroll < 0 {
		p.Scroll = 0
	}

	end := p.Scroll + bodyH
	if end > len(p.BodyLines) {
		end = len(p.BodyLines)
	}

	visible := make([]string, 0, bodyH)
	for r := p.Scroll; r < end; r++ {
		vl := p.BodyLines[r]
		if r == p.CursorRow {
			cIdx := -1
			if active {
				cIdx = p.CursorIdx
			}
			if cIdx >= 0 || p.HLStart >= 0 {
				visible = append(visible, cjk.RenderLineWithHighlight(vl, cIdx, p.HLStart, p.HLEnd))
			} else {
				visible = append(visible, vl.RawLine)
			}
		} else {
			visible = append(visible, vl.RawLine)
		}
	}
	for len(visible) < bodyH {
		visible = append(visible, "")
	}

	content := headerBox + "\n" + strings.Join(visible, "\n")
	boxStyle := styles.PopupBox
	if !active {
		boxStyle = styles.PopupBoxInactive
	}
	rendered := boxStyle.Width(popupW - 2).Render(content)
	return strings.Split(rendered, "\n")
}

func renderTermsBody(res *lookup.LookupResult, styles Styles, cond *runewidth.Condition, innerW int, showEx bool) []string {
	if res == nil || len(res.Terms) == 0 {
		return nil
	}
	var lines []string
	for idx, t := range res.Terms {
		if idx > 0 {
			lines = append(lines, styles.MutedText.Render(strings.Repeat("─", minInt(innerW, 36))))
		}

		// Headword + Reading + Reason + Dict badge
		var head strings.Builder
		head.WriteString(styles.TermHeadword.Render(t.Expression))
		if t.Reading != "" && t.Reading != t.Expression {
			head.WriteString(" ")
			head.WriteString(styles.TermReading.Render("[" + t.Reading + "]"))
		}
		if t.Reason != "" {
			head.WriteString(" ")
			head.WriteString(styles.TermTag.Render("(" + t.Reason + ")"))
		}
		if len(t.PitchAccents) > 0 {
			head.WriteString(" ")
			head.WriteString(styles.TermTag.Render(fmt.Sprintf("⚡[%d]", t.PitchAccents[0])))
		}
		head.WriteString(" ")
		head.WriteString(styles.TermDictBadge.Render("· " + t.DictTitle))
		lines = append(lines, head.String())

		if t.TermTags != "" {
			lines = append(lines, styles.TermTag.Render("  "+t.TermTags))
		}

		for sIdx, s := range t.Senses {
			prefix := fmt.Sprintf("%d. ", sIdx+1)
			if s.DefinitionTags != "" {
				for _, wl := range cjk.WrapPlain(cond, "   ["+s.DefinitionTags+"]", innerW) {
					lines = append(lines, styles.TermTag.Render(wl))
				}
			}
			for dIdx, def := range s.Definitions {
				lead := "   • "
				if dIdx == 0 {
					lead = styles.SenseNum.Render(prefix)
				}
				for lineIdx, subLine := range strings.Split(def, "\n") {
					subLine = strings.TrimSpace(subLine)
					if subLine == "" {
						continue
					}
					wrapped := cjk.WrapPlain(cond, subLine, innerW-4)
					for wIdx, wl := range wrapped {
						if lineIdx == 0 && wIdx == 0 {
							lines = append(lines, lead+wl)
						} else {
							lines = append(lines, "   "+wl)
						}
					}
				}
			}
			if showEx && len(s.Examples) > 0 {
				for _, ex := range s.Examples {
					for _, wl := range cjk.WrapPlain(cond, "   » "+ex, innerW-2) {
						lines = append(lines, styles.ExampleText.Render(wl))
					}
				}
			}
			if len(s.Notes) > 0 {
				for _, n := range s.Notes {
					for _, wl := range cjk.WrapPlain(cond, "   ℹ "+n, innerW-2) {
						lines = append(lines, styles.MutedText.Render(wl))
					}
				}
			}
		}
	}
	return lines
}

func renderKanjiBody(res *lookup.LookupResult, styles Styles, cond *runewidth.Condition, innerW int) []string {
	if res == nil || len(res.Kanji) == 0 {
		return nil
	}
	var lines []string
	for idx, k := range res.Kanji {
		if idx > 0 {
			lines = append(lines, styles.MutedText.Render(strings.Repeat("─", minInt(innerW, 36))))
		}
		header := styles.KanjiChar.Render(k.Character) + "  " + styles.TermDictBadge.Render(k.DictTitle)
		if k.Tags != "" {
			header += " " + styles.TermTag.Render("["+k.Tags+"]")
		}
		lines = append(lines, header)

		if k.Onyomi != "" {
			for _, wl := range cjk.WrapPlain(cond, "On:  "+k.Onyomi, innerW) {
				lines = append(lines, styles.TermReading.Render(wl))
			}
		}
		if k.Kunyomi != "" {
			for _, wl := range cjk.WrapPlain(cond, "Kun: "+k.Kunyomi, innerW) {
				lines = append(lines, styles.TermReading.Render(wl))
			}
		}
		if len(k.Meanings) > 0 {
			meanings := strings.Join(k.Meanings, "; ")
			for _, wl := range cjk.WrapPlain(cond, "Meaning: "+meanings, innerW) {
				lines = append(lines, wl)
			}
		}
		var statParts []string
		for _, key := range []string{"jlpt", "grade", "strokes", "freq"} {
			if v, ok := k.Stats[key]; ok && v != "" {
				statParts = append(statParts, fmt.Sprintf("%s:%s", key, v))
			}
		}
		if len(statParts) > 0 {
			lines = append(lines, styles.MutedText.Render(strings.Join(statParts, "  ")))
		}
	}
	return lines
}

// ComputePopupPosition determines (startRow, startCol, popupW, popupH) in viewport coordinates.
// For nested child popups (depth >= 1), cascades slightly from the anchor while clamping to viewport bounds.
func ComputePopupPosition(anchorRow, anchorCol, viewportW, viewportH, depth int) (int, int, int, int) {
	popupW := clamp(viewportW-6, 42, 74)
	if popupW > viewportW-2 {
		popupW = maxInt(20, viewportW-2)
	}
	popupH := clamp(viewportH/2, 10, 18)
	if popupH > viewportH-2 {
		popupH = maxInt(6, viewportH-2)
	}

	var startRow int
	if anchorRow+1+popupH <= viewportH {
		startRow = anchorRow + 1
	} else if anchorRow-popupH >= 0 {
		startRow = anchorRow - popupH
	} else {
		startRow = clamp(anchorRow+1, 0, maxInt(0, viewportH-popupH))
	}

	startCol := anchorCol
	if depth > 0 {
		startCol += 2
	}
	if startCol+popupW > viewportW-1 {
		startCol = viewportW - popupW - 1
	}
	if startCol < 1 {
		startCol = 1
	}

	return startRow, startCol, popupW, popupH
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
