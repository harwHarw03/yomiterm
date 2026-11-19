package cjk

import (
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

var ansiSeqRe = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

// CellToken represents a single grapheme cluster on a rendered terminal line.
type CellToken struct {
	Grapheme       string
	Width          int
	StartCol       int
	Style          string // Active ANSI SGR sequence before this grapheme
	PlainByteStart int
}

// VisualLine stores both the ANSI-rendered line and its grapheme/column map.
type VisualLine struct {
	RawLine    string
	PlainLine  string
	Tokens     []CellToken
	TotalWidth int
}

// ParseANSILine parses a rendered line (possibly containing ANSI SGR sequences) into a VisualLine.
func ParseANSILine(cond *runewidth.Condition, raw string) VisualLine {
	vl := VisualLine{RawLine: raw}
	var plain strings.Builder

	activeStyle := ""
	col := 0
	pos := 0

	matches := ansiSeqRe.FindAllStringIndex(raw, -1)
	processVisible := func(seg string) {
		gr := uniseg.NewGraphemes(seg)
		for gr.Next() {
			g := gr.Str()
			w := GraphemeWidth(cond, g)
			byteStart := plain.Len()
			plain.WriteString(g)
			vl.Tokens = append(vl.Tokens, CellToken{
				Grapheme:       g,
				Width:          w,
				StartCol:       col,
				Style:          activeStyle,
				PlainByteStart: byteStart,
			})
			col += w
		}
	}

	for _, m := range matches {
		if m[0] > pos {
			processVisible(raw[pos:m[0]])
		}
		seq := raw[m[0]:m[1]]
		if seq == "\x1b[0m" || seq == "\x1b[m" {
			activeStyle = ""
		} else if strings.HasSuffix(seq, "m") {
			activeStyle += seq
		}
		pos = m[1]
	}
	if pos < len(raw) {
		processVisible(raw[pos:])
	}

	vl.PlainLine = plain.String()
	vl.TotalWidth = col
	return vl
}

// NearestGraphemeAtCol returns the grapheme index closest to targetCol on vl.
func NearestGraphemeAtCol(vl VisualLine, targetCol int) int {
	if len(vl.Tokens) == 0 {
		return 0
	}
	if targetCol <= 0 {
		return 0
	}
	for i, tok := range vl.Tokens {
		if targetCol >= tok.StartCol && targetCol < tok.StartCol+tok.Width {
			return i
		}
		if tok.StartCol > targetCol {
			if i > 0 {
				return i - 1
			}
			return 0
		}
	}
	return len(vl.Tokens) - 1
}

// RenderLineWithHighlight renders vl with cursor and/or matched-span highlighting.
func RenderLineWithHighlight(vl VisualLine, cursorIdx int, hlStart, hlEnd int) string {
	if len(vl.Tokens) == 0 {
		if cursorIdx == 0 {
			return "\x1b[7m \x1b[0m"
		}
		return vl.RawLine
	}

	var b strings.Builder
	for i, tok := range vl.Tokens {
		isCursor := i == cursorIdx
		isHL := hlStart >= 0 && i >= hlStart && i < hlEnd

		if isCursor {
			b.WriteString("\x1b[7;1m")
			b.WriteString(tok.Grapheme)
			b.WriteString("\x1b[0m")
		} else if isHL {
			b.WriteString("\x1b[4;33m")
			b.WriteString(tok.Grapheme)
			b.WriteString("\x1b[0m")
		} else {
			if tok.Style != "" {
				b.WriteString(tok.Style)
			}
			b.WriteString(tok.Grapheme)
			if tok.Style != "" {
				b.WriteString("\x1b[0m")
			}
		}
	}
	if cursorIdx >= len(vl.Tokens) {
		b.WriteString("\x1b[7m \x1b[0m")
	}
	return b.String()
}

// CompositePopup overlays popupLines onto baseLines at (startRow, startCol) using
// ANSI-aware column slicing so 2-column CJK characters on the border never drift.
func CompositePopup(cond *runewidth.Condition, baseLines []string, popupLines []string, startRow, startCol, viewportW int) []string {
	out := make([]string, len(baseLines))
	copy(out, baseLines)

	for i, pLine := range popupLines {
		r := startRow + i
		if r < 0 || r >= len(out) {
			continue
		}
		baseVL := ParseANSILine(cond, out[r])
		pVL := ParseANSILine(cond, pLine)
		rightStart := startCol + pVL.TotalWidth

		left := sliceColumns(baseVL, 0, startCol)
		right := ""
		if rightStart < viewportW && rightStart < baseVL.TotalWidth {
			right = sliceColumns(baseVL, rightStart, viewportW)
		}

		out[r] = left + "\x1b[0m" + pLine + "\x1b[0m" + right
	}
	return out
}

// sliceColumns extracts graphemes covering visual columns [fromCol, toCol),
// padding with spaces if a 2-column CJK character straddles a boundary.
func sliceColumns(vl VisualLine, fromCol, toCol int) string {
	if toCol <= fromCol {
		return ""
	}
	var b strings.Builder
	curCol := fromCol
	for _, tok := range vl.Tokens {
		endCol := tok.StartCol + tok.Width
		if endCol <= fromCol {
			continue
		}
		if tok.StartCol >= toCol {
			break
		}
		// Token straddles left boundary (2-col char starting before fromCol)
		if tok.StartCol < fromCol {
			pad := minInt(endCol, toCol) - fromCol
			if pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
				curCol += pad
			}
			continue
		}
		// Gap before token
		if tok.StartCol > curCol {
			b.WriteString(strings.Repeat(" ", tok.StartCol-curCol))
			curCol = tok.StartCol
		}
		// Token straddles right boundary
		if endCol > toCol {
			pad := toCol - tok.StartCol
			if pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
				curCol += pad
			}
			break
		}
		if tok.Style != "" {
			b.WriteString(tok.Style)
		}
		b.WriteString(tok.Grapheme)
		if tok.Style != "" {
			b.WriteString("\x1b[0m")
		}
		curCol = endCol
	}
	if curCol < toCol && fromCol == 0 {
		b.WriteString(strings.Repeat(" ", toCol-curCol))
	}
	return b.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
