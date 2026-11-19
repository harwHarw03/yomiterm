package lookup

import (
	"strings"
	"unicode/utf8"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// Candidate represents a dictionary query term derived from an input prefix.
type Candidate struct {
	Query      string
	Surface    string
	SurfaceLen int // in runes
	Reason     string
}

// NewJapaneseTokenizer creates a Kagome tokenizer backed by the embedded IPA dictionary.
func NewJapaneseTokenizer() (*tokenizer.Tokenizer, error) {
	return tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
}

type contractionRule struct {
	from   string
	to     string
	reason string
}

var colloquialRules = []contractionRule{
	{"ちゃった", "てしまった", "〜ちゃった"},
	{"じゃった", "でしまった", "〜じゃった"},
	{"ちゃう", "てしまう", "〜ちゃう"},
	{"じゃう", "でしまう", "〜じゃう"},
	{"てる", "ている", "〜てる"},
	{"でる", "でいる", "〜でる"},
	{"てた", "ていた", "〜てた"},
	{"でた", "でいた", "〜でた"},
	{"りゃ", "れば", "〜りゃ"},
	{"なきゃ", "なければ", "〜なきゃ"},
	{"なくちゃ", "なくては", "〜なくちゃ"},
}

// generateJapaneseCandidates produces ranked query candidates from the start of text.
func generateJapaneseCandidates(tok *tokenizer.Tokenizer, text string) []Candidate {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	if len(runes) > 24 {
		runes = runes[:24]
	}
	window := string(runes)

	var candidates []Candidate
	seen := make(map[string]bool)
	add := func(query, surface string, surfLen int, reason string) {
		if query == "" {
			return
		}
		key := query + "|" + string(rune(surfLen))
		if seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, Candidate{
			Query:      query,
			Surface:    surface,
			SurfaceLen: surfLen,
			Reason:     reason,
		})
	}

	// 1. Longest-prefix raw scan (up to 16 runes) for compounds and grammar items.
	maxPrefix := len(runes)
	if maxPrefix > 16 {
		maxPrefix = 16
	}
	for l := maxPrefix; l >= 1; l-- {
		sub := string(runes[:l])
		add(sub, sub, l, "")
		if hira := KatakanaToHiragana(sub); hira != sub {
			add(hira, sub, l, "")
		}
	}

	// 2. Kagome morphological analysis on the window
	if tok != nil {
		addKagomeCandidates(tok, window, runes, 0, "", add)

		// 3. Colloquial contraction expansion + Kagome re-analysis
		for _, rule := range colloquialRules {
			if idx := strings.Index(window, rule.from); idx > 0 && idx <= 24 {
				prefixRuneLen := utf8.RuneCountInString(window[:idx])
				fromRuneLen := utf8.RuneCountInString(rule.from)
				expanded := window[:idx] + rule.to + window[idx+len(rule.from):]
				origSpan := prefixRuneLen + fromRuneLen
				addKagomeCandidates(tok, expanded, runes, origSpan, rule.reason, add)
			}
		}
	}

	return candidates
}

func addKagomeCandidates(
	tok *tokenizer.Tokenizer,
	input string,
	origRunes []rune,
	overrideSpan int,
	overrideReason string,
	add func(query, surface string, surfLen int, reason string),
) {
	tokens := tok.Tokenize(input)
	if len(tokens) == 0 {
		return
	}

	first := tokens[0]
	firstLen := utf8.RuneCountInString(first.Surface)

	// Calculate conjugated span by merging trailing auxiliary verbs / particles (助動詞, 助詞 like て/で/ば)
	spanLen := firstLen
	for i := 1; i < len(tokens); i++ {
		pos := tokens[i].POS()
		if len(pos) == 0 {
			break
		}
		if pos[0] == "助動詞" || (pos[0] == "助詞" && len(pos) > 1 && pos[1] == "接続助詞") || (pos[0] == "動詞" && len(pos) > 1 && (pos[1] == "非自立" || pos[1] == "接尾")) {
			spanLen += utf8.RuneCountInString(tokens[i].Surface)
		} else {
			break
		}
	}

	if overrideSpan > 0 {
		// Only apply overrideSpan if the first token actually reaches into the contracted region
		if spanLen >= overrideSpan {
			spanLen = overrideSpan
		} else {
			return
		}
	}
	if spanLen > len(origRunes) {
		spanLen = len(origRunes)
	}
	surface := string(origRunes[:spanLen])

	base, ok := first.BaseForm()
	if ok && base != "" && base != "*" {
		reason := overrideReason
		if reason == "" && base != surface {
			reason = first.POS()[0]
		}
		add(base, surface, spanLen, reason)
		// Also try with firstLen in case auxiliaries were a separate grammar point
		if firstLen != spanLen && firstLen <= len(origRunes) && overrideSpan == 0 {
			add(base, string(origRunes[:firstLen]), firstLen, "")
		}
	}

	// Reading in hiragana for kana-only lookups
	if reading, ok := first.Reading(); ok && reading != "" && reading != "*" {
		hira := KatakanaToHiragana(reading)
		add(hira, surface, spanLen, overrideReason)
	}
}
