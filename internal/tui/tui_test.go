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

	// Simulate window resize
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Move right ('l') and back left ('h')
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.CursorIdx != 1 {
		t.Fatalf("expected CursorIdx 1 after 'l', got %d", m.CursorIdx)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.CursorIdx != 0 {
		t.Fatalf("expected CursorIdx 0 after 'h', got %d", m.CursorIdx)
	}

	// Open popup with Enter
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Popup.Open || m.Mode != ModePopup {
		t.Fatalf("expected popup open and ModePopup, got open=%v mode=%v", m.Popup.Open, m.Mode)
	}

	view := m.View()
	if !strings.Contains(view, "powder snow") {
		t.Fatalf("expected popup view to contain 'powder snow', got:\n%s", view)
	}

	// Dismiss popup with Esc
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Popup.Open || m.Mode != ModeNormal {
		t.Fatalf("expected popup closed after Esc")
	}
}
