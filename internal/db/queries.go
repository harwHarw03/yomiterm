package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// Dictionary represents a row in the dictionaries table.
type Dictionary struct {
	ID          int64
	Title       string
	Revision    string
	Format      int
	Sequenced   bool
	Author      string
	Description string
	Priority    int
	Enabled     bool
	ImportedAt  string
	TermCount   int64
	KanjiCount  int64
}

// TermRow represents a term entry joined with its dictionary metadata.
type TermRow struct {
	ID             int64
	DictID         int64
	DictTitle      string
	DictPriority   int
	Expression     string
	Reading        string
	DefinitionTags string
	Rules          string
	Score          int
	GlossaryJSON   string
	Sequence       int64
	TermTags       string
}

// KanjiRow represents a kanji entry joined with its dictionary metadata.
type KanjiRow struct {
	ID           int64
	DictID       int64
	DictTitle    string
	DictPriority int
	Character    string
	Onyomi       string
	Kunyomi      string
	Tags         string
	MeaningsJSON string
	StatsJSON    string
}

// TermMetaRow represents pitch/frequency metadata for an expression.
type TermMetaRow struct {
	DictID     int64
	DictTitle  string
	Expression string
	Mode       string
	DataJSON   string
}

// ListDictionaries returns all imported dictionaries ordered by priority ASC, id ASC.
func (d *DB) ListDictionaries() ([]Dictionary, error) {
	rows, err := d.Conn.Query(`
		SELECT
			d.id, d.title, d.revision, d.format, d.sequenced, d.author, d.description,
			d.priority, d.enabled, d.imported_at,
			(SELECT COUNT(*) FROM terms t WHERE t.dict_id = d.id) AS term_count,
			(SELECT COUNT(*) FROM kanji k WHERE k.dict_id = d.id) AS kanji_count
		FROM dictionaries d
		ORDER BY d.priority ASC, d.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Dictionary
	for rows.Next() {
		var dict Dictionary
		var seqInt, enabledInt int
		if err := rows.Scan(
			&dict.ID, &dict.Title, &dict.Revision, &dict.Format, &seqInt,
			&dict.Author, &dict.Description, &dict.Priority, &enabledInt,
			&dict.ImportedAt, &dict.TermCount, &dict.KanjiCount,
		); err != nil {
			return nil, err
		}
		dict.Sequenced = seqInt != 0
		dict.Enabled = enabledInt != 0
		out = append(out, dict)
	}
	return out, rows.Err()
}

// GetDictionaryByTitle fetches a dictionary by its exact title.
func (d *DB) GetDictionaryByTitle(title string) (*Dictionary, error) {
	var dict Dictionary
	var seqInt, enabledInt int
	err := d.Conn.QueryRow(`
		SELECT id, title, revision, format, sequenced, author, description, priority, enabled, imported_at
		FROM dictionaries WHERE title = ?
	`, title).Scan(
		&dict.ID, &dict.Title, &dict.Revision, &dict.Format, &seqInt,
		&dict.Author, &dict.Description, &dict.Priority, &enabledInt, &dict.ImportedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dict.Sequenced = seqInt != 0
	dict.Enabled = enabledInt != 0
	return &dict, nil
}

// SetDictionaryEnabled enables or disables a dictionary by numeric ID or exact/prefix title.
func (d *DB) SetDictionaryEnabled(idOrTitle string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	res, err := d.Conn.Exec(`UPDATE dictionaries SET enabled = ? WHERE CAST(id AS TEXT) = ? OR title = ?`, val, idOrTitle, idOrTitle)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("dictionary %q not found", idOrTitle)
	}
	return nil
}

// SetDictionaryPriority updates the display priority of a dictionary (lower = higher priority).
func (d *DB) SetDictionaryPriority(idOrTitle string, priority int) error {
	res, err := d.Conn.Exec(`UPDATE dictionaries SET priority = ? WHERE CAST(id AS TEXT) = ? OR title = ?`, priority, idOrTitle, idOrTitle)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("dictionary %q not found", idOrTitle)
	}
	return nil
}

// DeleteDictionary removes a dictionary and cascades to its terms, kanji, tags, and meta.
func (d *DB) DeleteDictionary(idOrTitle string) error {
	res, err := d.Conn.Exec(`DELETE FROM dictionaries WHERE CAST(id AS TEXT) = ? OR title = ?`, idOrTitle, idOrTitle)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("dictionary %q not found", idOrTitle)
	}
	return nil
}

// LookupTerms queries enabled dictionaries for any term whose expression or reading matches candidates.
func (d *DB) LookupTerms(candidates []string) ([]TermRow, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(candidates))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
		SELECT
			t.id, t.dict_id, d.title, d.priority,
			t.expression, t.reading, t.definition_tags, t.rules,
			t.score, t.glossary_json, t.sequence, t.term_tags
		FROM terms t
		JOIN dictionaries d ON d.id = t.dict_id
		WHERE d.enabled = 1
		  AND (t.expression IN (%s) OR t.reading IN (%s))
	`, placeholders, placeholders)

	args := make([]any, 0, len(candidates)*2)
	for _, c := range candidates {
		args = append(args, c)
	}
	for _, c := range candidates {
		args = append(args, c)
	}

	rows, err := d.Conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TermRow
	for rows.Next() {
		var r TermRow
		if err := rows.Scan(
			&r.ID, &r.DictID, &r.DictTitle, &r.DictPriority,
			&r.Expression, &r.Reading, &r.DefinitionTags, &r.Rules,
			&r.Score, &r.GlossaryJSON, &r.Sequence, &r.TermTags,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LookupKanji queries enabled dictionaries for the given kanji characters.
func (d *DB) LookupKanji(chars []string) ([]KanjiRow, error) {
	if len(chars) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(chars))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
		SELECT
			k.id, k.dict_id, d.title, d.priority,
			k.character, k.onyomi, k.kunyomi, k.tags,
			k.meanings_json, k.stats_json
		FROM kanji k
		JOIN dictionaries d ON d.id = k.dict_id
		WHERE d.enabled = 1 AND k.character IN (%s)
		ORDER BY d.priority ASC, k.id ASC
	`, placeholders)

	args := make([]any, len(chars))
	for i, c := range chars {
		args[i] = c
	}

	rows, err := d.Conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KanjiRow
	for rows.Next() {
		var r KanjiRow
		if err := rows.Scan(
			&r.ID, &r.DictID, &r.DictTitle, &r.DictPriority,
			&r.Character, &r.Onyomi, &r.Kunyomi, &r.Tags,
			&r.MeaningsJSON, &r.StatsJSON,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LookupTermMeta queries enabled dictionaries for pitch/frequency metadata.
func (d *DB) LookupTermMeta(expressions []string) ([]TermMetaRow, error) {
	if len(expressions) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(expressions))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
		SELECT m.dict_id, d.title, m.expression, m.mode, m.data_json
		FROM term_meta m
		JOIN dictionaries d ON d.id = m.dict_id
		WHERE d.enabled = 1 AND m.expression IN (%s)
		ORDER BY d.priority ASC
	`, placeholders)

	args := make([]any, len(expressions))
	for i, e := range expressions {
		args[i] = e
	}

	rows, err := d.Conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TermMetaRow
	for rows.Next() {
		var r TermMetaRow
		if err := rows.Scan(&r.DictID, &r.DictTitle, &r.Expression, &r.Mode, &r.DataJSON); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
