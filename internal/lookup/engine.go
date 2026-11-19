package lookup

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/harw/yomiterm/internal/db"
	"github.com/harw/yomiterm/internal/importer"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// Sense represents a single numbered definition block within a dictionary entry.
type Sense struct {
	DefinitionTags string   `json:"definition_tags,omitempty"`
	Definitions    []string `json:"definitions"`
	Examples       []string `json:"examples,omitempty"`
	Notes          []string `json:"notes,omitempty"`
}

// TermResult represents a grouped, ranked dictionary entry.
type TermResult struct {
	DictTitle      string   `json:"dict_title"`
	DictPriority   int      `json:"dict_priority"`
	Expression     string   `json:"expression"`
	Reading        string   `json:"reading"`
	MatchedSurface string   `json:"matched_surface"`
	MatchLen       int      `json:"match_len"`
	Reason         string   `json:"reason,omitempty"`
	TermTags       string   `json:"term_tags,omitempty"`
	PitchAccents   []int    `json:"pitch_accents,omitempty"`
	Frequencies    []string `json:"frequencies,omitempty"`
	Senses         []Sense  `json:"senses"`
	ExactExpr      bool     `json:"-"`
	Score          int      `json:"-"`
}

// KanjiResult represents a kanji dictionary entry.
type KanjiResult struct {
	DictTitle string            `json:"dict_title"`
	Character string            `json:"character"`
	Onyomi    string            `json:"onyomi"`
	Kunyomi   string            `json:"kunyomi"`
	Tags      string            `json:"tags,omitempty"`
	Meanings  []string          `json:"meanings"`
	Stats     map[string]string `json:"stats,omitempty"`
}

// LookupResult holds both term and kanji matches for a query.
type LookupResult struct {
	Query        string        `json:"query"`
	MatchedSpan  string        `json:"matched_span"`
	MatchRuneLen int           `json:"match_rune_len"`
	Terms        []TermResult  `json:"terms"`
	Kanji        []KanjiResult `json:"kanji"`
}

// Engine performs offline morphological analysis, deinflection, and SQLite dictionary lookups.
type Engine struct {
	DB        *db.DB
	Tokenizer *tokenizer.Tokenizer
}

// NewEngine initializes the lookup engine with a Kagome Japanese tokenizer.
func NewEngine(database *db.DB) (*Engine, error) {
	tok, err := NewJapaneseTokenizer()
	if err != nil {
		return nil, fmt.Errorf("init japanese tokenizer: %w", err)
	}
	return &Engine{
		DB:        database,
		Tokenizer: tok,
	}, nil
}

// Lookup analyzes text starting at index 0 and returns ranked term and kanji results.
func (e *Engine) Lookup(text string) (*LookupResult, error) {
	text = strings.TrimLeft(text, " \t\r\n")
	if text == "" {
		return &LookupResult{}, nil
	}

	runes := []rune(text)
	if len(runes) > 24 {
		runes = runes[:24]
	}
	window := string(runes)

	var candidates []Candidate
	if ContainsKana(window) || hasJapaneseOrCJK(window) {
		candidates = generateJapaneseCandidates(e.Tokenizer, window)
	} else {
		// ASCII/other fallback: try exact word up to first whitespace
		word := strings.Fields(window)[0]
		candidates = []Candidate{{
			Query:      word,
			Surface:    word,
			SurfaceLen: utf8.RuneCountInString(word),
		}}
	}

	// Collect unique query strings
	querySet := make(map[string]bool, len(candidates))
	var queryList []string
	candByQuery := make(map[string][]Candidate)
	for _, c := range candidates {
		candByQuery[c.Query] = append(candByQuery[c.Query], c)
		if !querySet[c.Query] {
			querySet[c.Query] = true
			queryList = append(queryList, c.Query)
		}
	}

	rows, err := e.DB.LookupTerms(queryList)
	if err != nil {
		return nil, err
	}

	// Group rows by (dict_id, sequence) if sequence > 0, else unique row ID.
	type groupKey struct {
		dictID   int64
		sequence int64
		rowID    int64
	}
	groups := make(map[groupKey]*TermResult)
	var groupOrder []groupKey

	for _, r := range rows {
		// Find best matching candidate for this row
		bestCand, exactExpr := pickBestCandidate(r, candByQuery)
		if bestCand.SurfaceLen == 0 {
			continue
		}

		gk := groupKey{dictID: r.DictID, sequence: r.Sequence}
		if r.Sequence == 0 {
			gk.rowID = r.ID
		}

		gv := importer.ParseGlossary(r.GlossaryJSON)
		sense := Sense{
			DefinitionTags: r.DefinitionTags,
			Definitions:    gv.Definitions,
			Examples:       gv.Examples,
			Notes:          gv.Notes,
		}

		existing, ok := groups[gk]
		if !ok {
			groups[gk] = &TermResult{
				DictTitle:      r.DictTitle,
				DictPriority:   r.DictPriority,
				Expression:     r.Expression,
				Reading:        r.Reading,
				MatchedSurface: bestCand.Surface,
				MatchLen:       bestCand.SurfaceLen,
				Reason:         bestCand.Reason,
				TermTags:       r.TermTags,
				Senses:         []Sense{sense},
				ExactExpr:      exactExpr,
				Score:          r.Score,
			}
			groupOrder = append(groupOrder, gk)
		} else {
			existing.Senses = append(existing.Senses, sense)
			if bestCand.SurfaceLen > existing.MatchLen {
				existing.MatchLen = bestCand.SurfaceLen
				existing.MatchedSurface = bestCand.Surface
				existing.Reason = bestCand.Reason
			}
			if exactExpr {
				existing.ExactExpr = true
			}
			if r.Score > existing.Score {
				existing.Score = r.Score
			}
		}
	}

	terms := make([]TermResult, 0, len(groupOrder))
	for _, gk := range groupOrder {
		terms = append(terms, *groups[gk])
	}

	sort.Slice(terms, func(i, j int) bool {
		if terms[i].MatchLen != terms[j].MatchLen {
			return terms[i].MatchLen > terms[j].MatchLen
		}
		if terms[i].ExactExpr != terms[j].ExactExpr {
			return terms[i].ExactExpr
		}
		if terms[i].DictPriority != terms[j].DictPriority {
			return terms[i].DictPriority < terms[j].DictPriority
		}
		if terms[i].Score != terms[j].Score {
			return terms[i].Score > terms[j].Score
		}
		return terms[i].Expression < terms[j].Expression
	})

	if len(terms) > 20 {
		terms = terms[:20]
	}

	// Determine matched span from top result (or fallback to first 4 runes for kanji)
	matchedSpan := ""
	matchRuneLen := 0
	if len(terms) > 0 {
		matchedSpan = terms[0].MatchedSurface
		matchRuneLen = terms[0].MatchLen
	}

	// Attach pitch/freq meta if present
	if len(terms) > 0 {
		exprSet := make(map[string]bool)
		var exprs []string
		for _, t := range terms {
			if !exprSet[t.Expression] {
				exprSet[t.Expression] = true
				exprs = append(exprs, t.Expression)
			}
		}
		if metaRows, err := e.DB.LookupTermMeta(exprs); err == nil && len(metaRows) > 0 {
			attachMeta(terms, metaRows)
		}
	}

	// Kanji lookup: use matchedSpan if non-empty, else first 4 runes of window
	kanjiSource := matchedSpan
	if kanjiSource == "" {
		limit := len(runes)
		if limit > 4 {
			limit = 4
		}
		kanjiSource = string(runes[:limit])
	}
	kanjiChars := ExtractUniqueKanji(kanjiSource)
	var kanjiResults []KanjiResult
	if len(kanjiChars) > 0 {
		kRows, err := e.DB.LookupKanji(kanjiChars)
		if err == nil {
			// Preserve order of appearance in kanjiSource
			byChar := make(map[string][]db.KanjiRow)
			for _, kr := range kRows {
				byChar[kr.Character] = append(byChar[kr.Character], kr)
			}
			for _, ch := range kanjiChars {
				for _, kr := range byChar[ch] {
					var meanings []string
					_ = json.Unmarshal([]byte(kr.MeaningsJSON), &meanings)
					var stats map[string]string
					_ = json.Unmarshal([]byte(kr.StatsJSON), &stats)
					kanjiResults = append(kanjiResults, KanjiResult{
						DictTitle: kr.DictTitle,
						Character: kr.Character,
						Onyomi:    kr.Onyomi,
						Kunyomi:   kr.Kunyomi,
						Tags:      kr.Tags,
						Meanings:  meanings,
						Stats:     stats,
					})
				}
			}
		}
	}

	return &LookupResult{
		Query:        window,
		MatchedSpan:  matchedSpan,
		MatchRuneLen: matchRuneLen,
		Terms:        terms,
		Kanji:        kanjiResults,
	}, nil
}

func pickBestCandidate(row db.TermRow, candByQuery map[string][]Candidate) (Candidate, bool) {
	var best Candidate
	bestExact := false
	consider := func(cands []Candidate, exact bool) {
		for _, c := range cands {
			if c.SurfaceLen > best.SurfaceLen || (c.SurfaceLen == best.SurfaceLen && exact && !bestExact) {
				best = c
				bestExact = exact
			}
		}
	}
	if cands, ok := candByQuery[row.Expression]; ok {
		consider(cands, true)
	}
	if cands, ok := candByQuery[row.Reading]; ok {
		consider(cands, false)
	}
	return best, bestExact
}

func attachMeta(terms []TermResult, metaRows []db.TermMetaRow) {
	for _, mr := range metaRows {
		for i := range terms {
			if terms[i].Expression != mr.Expression {
				continue
			}
			switch mr.Mode {
			case "pitch":
				var p struct {
					Reading string `json:"reading"`
					Pitches []struct {
						Position int `json:"position"`
					} `json:"pitches"`
				}
				if json.Unmarshal([]byte(mr.DataJSON), &p) == nil {
					if p.Reading == "" || p.Reading == terms[i].Reading {
						for _, pt := range p.Pitches {
							terms[i].PitchAccents = append(terms[i].PitchAccents, pt.Position)
						}
					}
				}
			case "freq":
				var f any
				if json.Unmarshal([]byte(mr.DataJSON), &f) == nil {
					terms[i].Frequencies = append(terms[i].Frequencies, fmt.Sprintf("%v", f))
				}
			}
		}
	}
}

func hasJapaneseOrCJK(s string) bool {
	for _, r := range s {
		if IsCJK(r) {
			return true
		}
	}
	return false
}
