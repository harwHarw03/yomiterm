package cjk

import (
	"testing"
)

func TestStringWidthAndWrapPlain(t *testing.T) {
	cond := NewCondition(1)
	if w := StringWidth(cond, "粉雪abc"); w != 7 {
		t.Fatalf("expected width 7, got %d", w)
	}

	wrapped := WrapPlain(cond, "粉雪abc舞う", 5)
	// "粉雪" (4) + 'a' (1) = 5; "bc" (2) + "舞" (2) = 4; "う" (2)
	if len(wrapped) != 3 {
		t.Fatalf("expected 3 wrapped lines, got %d: %#v", len(wrapped), wrapped)
	}
	if wrapped[0] != "粉雪a" || wrapped[1] != "bc舞" || wrapped[2] != "う" {
		t.Fatalf("unexpected wrap result: %#v", wrapped)
	}
}

func TestParseANSILineAndNearestCol(t *testing.T) {
	cond := NewCondition(1)
	raw := "\x1b[1m粉雪\x1b[0m a"
	vl := ParseANSILine(cond, raw)
	if vl.PlainLine != "粉雪 a" {
		t.Fatalf("expected plain '粉雪 a', got %q", vl.PlainLine)
	}
	if vl.TotalWidth != 6 {
		t.Fatalf("expected total width 6, got %d", vl.TotalWidth)
	}
	// Column 2 and 3 both belong to '雪' (index 1)
	if idx := NearestGraphemeAtCol(vl, 3); idx != 1 {
		t.Fatalf("expected grapheme index 1 at col 3, got %d", idx)
	}
	// Column 5 belongs to 'a' (index 3)
	if idx := NearestGraphemeAtCol(vl, 5); idx != 3 {
		t.Fatalf("expected grapheme index 3 at col 5, got %d", idx)
	}
}

func TestCompositePopupStraddlingCJK(t *testing.T) {
	cond := NewCondition(1)
	base := []string{"粉雪舞う季節はいつもすれ違い"}
	popup := []string{"+----+"}
	out := CompositePopup(cond, base, popup, 0, 3, 28)
	// Col 3 is the second half of '雪' (cols 2..3), so left should be "粉 " (width 3)
	vl := ParseANSILine(cond, out[0])
	if vl.Tokens[0].Grapheme != "粉" || vl.Tokens[1].Grapheme != " " {
		t.Fatalf("expected straddled CJK replaced with space, got %q", vl.PlainLine)
	}
}
