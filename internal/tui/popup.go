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
	Open         bool
	Focused      bool
	Tab          PopupTab
	Scroll       int
	ShowExamples bool
	Result       *lookup.LookupResult
}

func (p *PopupState) RenderLines(styles Styles, cond *runewidth.Condition, popupW, popupH int) []string {
	innerW := popupW - 4 // 2 border + 2 padding
	if innerW < 20 {
		innerW = 20
	}

	// Header tabs
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
	hint := styles.MutedText.Render(" [Tab] [J/K] [e:ex] [Esc]")
	header := lipgloss.JoinHorizontal(lipgloss.Top, tTab, " ", kTab, hint)
	headerBox := styles.PopupHeader.Width(innerW).Render(header)

	var bodyLines []string
	if p.Tab == TabTerms {
		bodyLines = renderTermsBody(p.Result, styles, cond, innerW, p.ShowExamples)
	} else {
		bodyLines = renderKanjiBody(p.Result, styles, cond, innerW)
	}

	if len(bodyLines) == 0 {
		bodyLines = []string{styles.MutedText.Render("No entries found.")}
	}

	// Calculate visible body height (popupH minus 2 border lines minus header height)
	headerLines := strings.Split(headerBox, "\n")
	bodyH := popupH - 2 - len(headerLines)
	if bodyH < 3 {
		bodyH = 3
	}

	maxScroll := len(bodyLines) - bodyH
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
	if end > len(bodyLines) {
		end = len(bodyLines)
	}
	visible := make([]string, 0, bodyH)
	visible = append(visible, bodyLines[p.Scroll:end]...)
	for len(visible) < bodyH {
		visible = append(visible, "")
	}

	content := headerBox + "\n" + strings.Join(visible, "\n")
	rendered := styles.PopupBox.Width(popupW - 2).Render(content)
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
				// Split multi-line definitions (e.g. DOJG)
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
		// Key stats
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
func ComputePopupPosition(cursorScreenRow, cursorScreenCol, viewportW, viewportH int) (int, int, int, int) {
	popupW := clamp(viewportW-6, 42, 74)
	if popupW > viewportW-2 {
		popupW = maxInt(20, viewportW-2)
	}
	popupH := clamp(viewportH/2, 10, 18)
	if popupH > viewportH-2 {
		popupH = maxInt(6, viewportH-2)
	}

	var startRow int
	if cursorScreenRow+1+popupH <= viewportH {
		startRow = cursorScreenRow + 1
	} else if cursorScreenRow-popupH >= 0 {
		startRow = cursorScreenRow - popupH
	} else {
		startRow = maxInt(0, viewportH-popupH)
	}

	startCol := cursorScreenCol
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
