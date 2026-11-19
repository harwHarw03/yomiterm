package lookup

import (
	"testing"

	"github.com/harw/yomiterm/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	res, err := database.Conn.Exec(`INSERT INTO dictionaries (title, priority, enabled) VALUES ('JMdict Test', 10, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	dictID, _ := res.LastInsertId()

	terms := []struct {
		expr, read, gloss string
		seq               int64
	}{
		{"食べる", "たべる", `["to eat"]`, 1},
		{"見る", "みる", `["to see", "to look"]`, 2},
		{"繋がる", "つながる", `["to be connected"]`, 3},
		{"くれる", "くれる", `["to give"]`, 4},
		{"染める", "そめる", `["to dye"]`, 5},
		{"粉雪", "こなゆき", `["powder snow"]`, 6},
		{"生きがい", "いきがい", `["reason for living"]`, 7},
	}
	for _, tm := range terms {
		_, err := database.Conn.Exec(
			`INSERT INTO terms (dict_id, expression, reading, glossary_json, sequence) VALUES (?, ?, ?, ?, ?)`,
			dictID, tm.expr, tm.read, tm.gloss, tm.seq,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = database.Conn.Exec(
		`INSERT INTO kanji (dict_id, character, onyomi, kunyomi, tags, meanings_json, stats_json) VALUES (?, '粉', 'フン', 'こ こな', 'jouyou', '["flour","powder"]', '{"jlpt":"2"}')`,
		dictID,
	)
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func TestJapaneseDeinflectionAndLongestPrefix(t *testing.T) {
	database := setupTestDB(t)
	engine, err := NewEngine(database)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	cases := []struct {
		input      string
		wantExpr   string
		wantMinLen int
	}{
		{"食べていた", "食べる", 5},
		{"見てるのに", "見る", 3},
		{"繋がってたのに", "繋がる", 5},
		{"くれりゃいいんだ", "くれる", 4},
		{"染められたなら", "染める", 5},
		{"粉雪舞う季節", "粉雪", 2},
		{"生きがいだ", "生きがい", 4},
	}

	for _, tc := range cases {
		res, err := engine.Lookup(tc.input)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", tc.input, err)
		}
		if len(res.Terms) == 0 {
			t.Fatalf("Lookup(%q): expected term match, got 0", tc.input)
		}
		if res.Terms[0].Expression != tc.wantExpr {
			t.Errorf("Lookup(%q): got %q, want %q", tc.input, res.Terms[0].Expression, tc.wantExpr)
		}
		if res.Terms[0].MatchLen < tc.wantMinLen {
			t.Errorf("Lookup(%q): match len %d < %d", tc.input, res.Terms[0].MatchLen, tc.wantMinLen)
		}
	}

	// Verify kanji extraction on 粉雪
	res, err := engine.Lookup("粉雪舞う")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Kanji) == 0 || res.Kanji[0].Character != "粉" {
		t.Fatalf("expected kanji '粉' in result, got %+v", res.Kanji)
	}
}
