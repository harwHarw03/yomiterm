package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/glamour"
	"github.com/harw/yomiterm/internal/cjk"
	"github.com/mattn/go-runewidth"
)

// Document holds the raw markdown and its Glamour-rendered visual lines.
type Document struct {
	RawMarkdown string
	Lines       []cjk.VisualLine
	Width       int
}

// RenderDocument renders markdown at targetWidth using Glamour and parses CJK cell maps.
func RenderDocument(markdown string, targetWidth int, theme string, cond *runewidth.Condition) (*Document, error) {
	if targetWidth < 20 {
		targetWidth = 20
	}
	styleOpt := glamour.WithAutoStyle()
	switch strings.ToLower(theme) {
	case "dark":
		styleOpt = glamour.WithStandardStyle("dark")
	case "light":
		styleOpt = glamour.WithStandardStyle("light")
	case "notty", "plain":
		styleOpt = glamour.WithStandardStyle("notty")
	}

	renderer, err := glamour.NewTermRenderer(
		styleOpt,
		glamour.WithWordWrap(targetWidth),
	)
	if err != nil {
		return nil, err
	}

	rendered, err := renderer.Render(markdown)
	if err != nil {
		return nil, err
	}

	rendered = strings.Trim(rendered, "\n")
	rawLines := strings.Split(rendered, "\n")
	vLines := make([]cjk.VisualLine, len(rawLines))
	for i, rl := range rawLines {
		vLines[i] = cjk.ParseANSILine(cond, rl)
	}
	if len(vLines) == 0 {
		vLines = []cjk.VisualLine{{}}
	}

	return &Document{
		RawMarkdown: markdown,
		Lines:       vLines,
		Width:       targetWidth,
	}, nil
}

// SliceFromCursor returns up to maxRunes of plain text starting at (row, graphemeIdx).
func (d *Document) SliceFromCursor(row, graphemeIdx, maxRunes int) string {
	if row < 0 || row >= len(d.Lines) {
		return ""
	}
	var b strings.Builder
	count := 0
	for r := row; r < len(d.Lines) && count < maxRunes; r++ {
		toks := d.Lines[r].Tokens
		start := 0
		if r == row {
			start = graphemeIdx
		}
		for i := start; i < len(toks) && count < maxRunes; i++ {
			b.WriteString(toks[i].Grapheme)
			count++
		}
		if r == row && len(toks) == 0 {
			break
		}
		if count > 0 && r+1 < len(d.Lines) && len(d.Lines[r+1].Tokens) == 0 {
			break
		}
	}
	return b.String()
}

// NextWord moves forward to the next non-whitespace word/token boundary.
func (d *Document) NextWord(row, gIdx int) (int, int) {
	if row >= len(d.Lines) {
		return row, gIdx
	}
	toks := d.Lines[row].Tokens
	if gIdx+1 < len(toks) {
		curClass := charClass(toks[gIdx].Grapheme)
		i := gIdx + 1
		// Skip current class (unless whitespace)
		if curClass != classSpace {
			for i < len(toks) && charClass(toks[i].Grapheme) == curClass {
				i++
			}
		}
		// Skip whitespace
		for i < len(toks) && charClass(toks[i].Grapheme) == classSpace {
			i++
		}
		if i < len(toks) {
			return row, i
		}
	}
	// Advance to next non-empty line
	for r := row + 1; r < len(d.Lines); r++ {
		for i, t := range d.Lines[r].Tokens {
			if charClass(t.Grapheme) != classSpace {
				return r, i
			}
		}
	}
	return row, gIdx
}

// PrevWord moves backward to the start of the previous word/token.
func (d *Document) PrevWord(row, gIdx int) (int, int) {
	if row < 0 || row >= len(d.Lines) {
		return 0, 0
	}
	toks := d.Lines[row].Tokens
	i := gIdx - 1
	for i >= 0 && charClass(toks[i].Grapheme) == classSpace {
		i--
	}
	if i >= 0 {
		cls := charClass(toks[i].Grapheme)
		for i > 0 && charClass(toks[i-1].Grapheme) == cls {
			i--
		}
		return row, i
	}
	for r := row - 1; r >= 0; r-- {
		lt := d.Lines[r].Tokens
		j := len(lt) - 1
		for j >= 0 && charClass(lt[j].Grapheme) == classSpace {
			j--
		}
		if j >= 0 {
			cls := charClass(lt[j].Grapheme)
			for j > 0 && charClass(lt[j-1].Grapheme) == cls {
				j--
			}
			return r, j
		}
	}
	return 0, 0
}

type wordClass int

const (
	classSpace wordClass = iota
	classASCII
	classKanji
	classHiragana
	classKatakana
	classPunct
)

func charClass(g string) wordClass {
	for _, r := range g {
		if unicode.IsSpace(r) {
			return classSpace
		}
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return classPunct
		}
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
			return classKanji
		}
		if r >= 0x3040 && r <= 0x309F {
			return classHiragana
		}
		if r >= 0x30A0 && r <= 0x30FF {
			return classKatakana
		}
		return classASCII
	}
	return classSpace
}
