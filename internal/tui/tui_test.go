package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harw/yomiterm/internal/config"
	"github.com/harw/yomiterm/internal/db"
	"github.com/harw/yomiterm/internal/lookup"
)

func TestTUIModelNavigationAndPopup(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	res, err := database.Conn.Exec(`INSERT INTO dictionaries (title, priority, enabled) VALUES ('JMdict Test', 10, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	dictID, _ := res.LastInsertId()
	_, err = database.Conn.Exec(
		`INSERT INTO terms (dict_id, expression, reading, glossary_json, sequence) VALUES (?, '粉雪', 'こなゆき', '["powder snow"]', 1)`,
		dictID,
	)
	if err != nil {
		t.Fatal(err)
	}

	engine, err := lookup.NewEngine(database)
	if err != nil {
		t.Fatal(err)
	}

	m, err := NewModel("test.md", "粉雪舞う季節\n", engine, config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.CursorIdx != 1 {
		t.Fatalf("expected CursorIdx 1 after 'l', got %d", m.CursorIdx)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.CursorIdx != 0 {
		t.Fatalf("expected CursorIdx 0 after 'h', got %d", m.CursorIdx)
	}

	// Open root popup with Enter
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.Popups) != 1 || m.Mode != ModePopup {
		t.Fatalf("expected 1 popup and ModePopup, got len=%d mode=%v", len(m.Popups), m.Mode)
	}

	view := m.View()
	if !strings.Contains(view, "powder snow") {
		t.Fatalf("expected popup view to contain 'powder snow', got:\n%s", view)
	}

	// Dismiss popup with Esc
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.Popups) != 0 || m.Mode != ModeNormal {
		t.Fatalf("expected popup stack empty after Esc, got len=%d mode=%v", len(m.Popups), m.Mode)
	}
}

func TestRecursiveChildPopupsUpToDepth5(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	res, err := database.Conn.Exec(`INSERT INTO dictionaries (title, priority, enabled) VALUES ('JMdict Chain', 10, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	dictID, _ := res.LastInsertId()

	// Create a 6-word chain where each word's definition contains the next word
	chain := []struct {
		expr, read, gloss string
		seq               int64
	}{
		{"粉雪", "こなゆき", `["細かい雪のこと"]`, 1},
		{"細かい", "こまかい", `["小さな粒子"]`, 2},
		{"小さな", "ちいさな", `["大きい反対"]`, 3},
		{"大きい", "おおきい", `["広い空間"]`, 4},
		{"広い", "ひろい", `["深い海"]`, 5},
		{"深い", "ふかい", `["deep"]`, 6},
	}
	for _, c := range chain {
		_, err := database.Conn.Exec(
			`INSERT INTO terms (dict_id, expression, reading, glossary_json, sequence) VALUES (?, ?, ?, ?, ?)`,
			dictID, c.expr, c.read, c.gloss, c.seq,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	engine, err := lookup.NewEngine(database)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	if cfg.MaxPopupDepth != 5 {
		t.Fatalf("expected default MaxPopupDepth 5, got %d", cfg.MaxPopupDepth)
	}

	m, err := NewModel("chain.md", "粉雪\n", engine, cfg)
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})

	// Open depth 1 (粉雪)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.Popups) != 1 {
		t.Fatalf("expected depth 1, got %d", len(m.Popups))
	}

	// Helper to position the active popup's cursor onto a target rune and press Enter
	diveInto := func(targetRune string) {
		t.Helper()
		top := &m.Popups[len(m.Popups)-1]
		found := false
		for r, vl := range top.BodyLines {
			for i, tok := range vl.Tokens {
				if tok.Grapheme == targetRune {
					top.CursorRow = r
					top.CursorIdx = i
					top.UpdateDesiredCol()
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Fatalf("could not find rune %q inside popup depth %d", targetRune, len(m.Popups))
		}
		m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}

	// Dive to depth 2 (細かい), 3 (小さな), 4 (大きい), 5 (広い)
	diveInto("細")
	if len(m.Popups) != 2 || m.Popups[1].Result.Terms[0].Expression != "細かい" {
		t.Fatalf("expected depth 2 '細かい', got depth=%d", len(m.Popups))
	}
	diveInto("小")
	if len(m.Popups) != 3 || m.Popups[2].Result.Terms[0].Expression != "小さな" {
		t.Fatalf("expected depth 3 '小さな', got depth=%d", len(m.Popups))
	}
	diveInto("大")
	if len(m.Popups) != 4 || m.Popups[3].Result.Terms[0].Expression != "大きい" {
		t.Fatalf("expected depth 4 '大きい', got depth=%d", len(m.Popups))
	}
	diveInto("広")
	if len(m.Popups) != 5 || m.Popups[4].Result.Terms[0].Expression != "広い" {
		t.Fatalf("expected depth 5 '広い', got depth=%d", len(m.Popups))
	}

	// Verify rendered View() includes depth [5/5] badge
	view := m.View()
	if !strings.Contains(view, "[5/5]") {
		t.Fatalf("expected View() to contain '[5/5]' depth indicator")
	}

	// Attempting a 6th popup must be capped at MaxPopupDepth (5)
	diveInto("深")
	if len(m.Popups) != 5 {
		t.Fatalf("expected depth to remain capped at 5, got %d", len(m.Popups))
	}
	if !strings.Contains(m.StatusMsg, "Max popup depth") {
		t.Fatalf("expected max depth status message, got %q", m.StatusMsg)
	}

	// Esc unwinds one level at a time (5 -> 4 -> 3)
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.Popups) != 4 || m.Popups[3].Result.Terms[0].Expression != "大きい" {
		t.Fatalf("expected unwind to depth 4 '大きい', got depth=%d", len(m.Popups))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.Popups) != 3 {
		t.Fatalf("expected unwind to depth 3, got %d", len(m.Popups))
	}

	// 'q' closes all remaining popups at once
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if len(m.Popups) != 0 || m.Mode != ModeNormal {
		t.Fatalf("expected 'q' to close all popups and return to ModeNormal, got len=%d mode=%v", len(m.Popups), m.Mode)
	}
}
