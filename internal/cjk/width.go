package cjk

import (
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// NewCondition returns a runewidth.Condition configured for East Asian ambiguous width.
func NewCondition(ambiguousWidth int) *runewidth.Condition {
	return &runewidth.Condition{
		EastAsianWidth: ambiguousWidth == 2,
	}
}

// GraphemeWidth returns the terminal column width of a single grapheme cluster.
func GraphemeWidth(cond *runewidth.Condition, g string) int {
	if g == "" {
		return 0
	}
	if g == "\t" {
		return 4
	}
	// Use uniseg.StringWidth or runewidth on first non-zero rune, handling ZWJ/emoji
	w := cond.StringWidth(g)
	if w < 0 {
		return 0
	}
	return w
}

// StringWidth returns the total display column width of s.
func StringWidth(cond *runewidth.Condition, s string) int {
	w := 0
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		w += GraphemeWidth(cond, gr.Str())
	}
	return w
}

// WrapPlain wraps plain text (no ANSI) to maxWidth columns without ever splitting
// a 2-column CJK grapheme or breaking inside a grapheme cluster.
func WrapPlain(cond *runewidth.Condition, text string, maxWidth int) []string {
	if maxWidth <= 2 {
		return []string{text}
	}
	paragraphs := strings.Split(text, "\n")
	var lines []string
	for _, p := range paragraphs {
		if p == "" {
			lines = append(lines, "")
			continue
		}
		var cur strings.Builder
		col := 0
		gr := uniseg.NewGraphemes(p)
		for gr.Next() {
			g := gr.Str()
			w := GraphemeWidth(cond, g)
			if col > 0 && col+w > maxWidth {
				lines = append(lines, cur.String())
				cur.Reset()
				col = 0
			}
			cur.WriteString(g)
			col += w
		}
		lines = append(lines, cur.String())
	}
	return lines
}
