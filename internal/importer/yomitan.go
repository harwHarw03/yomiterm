package importer

import "encoding/json"

// IndexJSON represents Yomitan's index.json metadata.
type IndexJSON struct {
	Title       string `json:"title"`
	Revision    string `json:"revision"`
	Format      int    `json:"format"`
	Sequenced   bool   `json:"sequenced"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Attribution string `json:"attribution"`
}

// TagEntry represents a 5-element tuple from tag_bank_*.json:
// [name, category, sortOrder, notes, score]
type TagEntry struct {
	Name      string
	Category  string
	SortOrder int
	Notes     string
	Score     int
}

func (t *TagEntry) UnmarshalJSON(data []byte) error {
	var raw [5]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	_ = json.Unmarshal(raw[0], &t.Name)
	_ = json.Unmarshal(raw[1], &t.Category)
	_ = json.Unmarshal(raw[2], &t.SortOrder)
	_ = json.Unmarshal(raw[3], &t.Notes)
	_ = json.Unmarshal(raw[4], &t.Score)
	return nil
}

// TermEntry represents an 8-element tuple from term_bank_*.json (format 3):
// [expression, reading, definitionTags, rules, score, glossary, sequence, termTags]
type TermEntry struct {
	Expression     string
	Reading        string
	DefinitionTags string
	Rules          string
	Score          int
	GlossaryRaw    string
	Sequence       int64
	TermTags       string
}

func (t *TermEntry) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw[0], &t.Expression)
	}
	if len(raw) > 1 {
		_ = json.Unmarshal(raw[1], &t.Reading)
	}
	if len(raw) > 2 {
		_ = json.Unmarshal(raw[2], &t.DefinitionTags)
	}
	if len(raw) > 3 {
		_ = json.Unmarshal(raw[3], &t.Rules)
	}
	if len(raw) > 4 {
		_ = json.Unmarshal(raw[4], &t.Score)
	}
	if len(raw) > 5 {
		t.GlossaryRaw = string(raw[5])
	}
	if len(raw) > 6 {
		_ = json.Unmarshal(raw[6], &t.Sequence)
	}
	if len(raw) > 7 {
		_ = json.Unmarshal(raw[7], &t.TermTags)
	}
	return nil
}

// KanjiEntry represents a 6-element tuple from kanji_bank_*.json (format 3):
// [character, onyomi, kunyomi, tags, meanings, stats]
type KanjiEntry struct {
	Character   string
	Onyomi      string
	Kunyomi     string
	Tags        string
	MeaningsRaw string
	StatsRaw    string
}

func (k *KanjiEntry) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw[0], &k.Character)
	}
	if len(raw) > 1 {
		_ = json.Unmarshal(raw[1], &k.Onyomi)
	}
	if len(raw) > 2 {
		_ = json.Unmarshal(raw[2], &k.Kunyomi)
	}
	if len(raw) > 3 {
		_ = json.Unmarshal(raw[3], &k.Tags)
	}
	if len(raw) > 4 {
		k.MeaningsRaw = string(raw[4])
	}
	if len(raw) > 5 {
		k.StatsRaw = string(raw[5])
	}
	return nil
}

// MetaEntry represents a 3-element tuple from term_meta_bank_*.json or kanji_meta_bank_*.json:
// [expressionOrCharacter, mode, data]
type MetaEntry struct {
	Key     string
	Mode    string
	DataRaw string
}

func (m *MetaEntry) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw[0], &m.Key)
	}
	if len(raw) > 1 {
		_ = json.Unmarshal(raw[1], &m.Mode)
	}
	if len(raw) > 2 {
		m.DataRaw = string(raw[2])
	}
	return nil
}
