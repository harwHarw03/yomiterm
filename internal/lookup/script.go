package lookup

import (
	"strings"
	"unicode"
)

// IsKanji returns true if r is a CJK Unified Ideograph (or extension A).
func IsKanji(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0xF900 && r <= 0xFAFF)
}

// IsHiragana returns true if r is in the Hiragana Unicode block.
func IsHiragana(r rune) bool {
	return r >= 0x3040 && r <= 0x309F
}

// IsKatakana returns true if r is in the Katakana Unicode block (excluding prolonged sound mark? includes ー).
func IsKatakana(r rune) bool {
	return r >= 0x30A0 && r <= 0x30FF
}

// IsKana returns true if r is Hiragana or Katakana.
func IsKana(r rune) bool {
	return IsHiragana(r) || IsKatakana(r)
}

// IsCJK returns true if r is Kanji, Hiragana, Katakana, or CJK punctuation.
func IsCJK(r rune) bool {
	return IsKanji(r) || IsKana(r) || unicode.Is(unicode.Han, r)
}

// ContainsKana returns true if s contains any Hiragana or Katakana character.
func ContainsKana(s string) bool {
	for _, r := range s {
		if IsKana(r) {
			return true
		}
	}
	return false
}

// KatakanaToHiragana converts Katakana runes in s to Hiragana.
func KatakanaToHiragana(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x30A1 && r <= 0x30F6 {
			b.WriteRune(r - 0x60)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ExtractUniqueKanji returns unique kanji characters in s in order of appearance.
func ExtractUniqueKanji(s string) []string {
	seen := make(map[rune]bool)
	var out []string
	for _, r := range s {
		if IsKanji(r) && !seen[r] {
			seen[r] = true
			out = append(out, string(r))
		}
	}
	return out
}
