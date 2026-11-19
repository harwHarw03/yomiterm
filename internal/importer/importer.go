package importer

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/harw/yomiterm/internal/db"
)

var (
	reTagBank       = regexp.MustCompile(`^tag_bank_(\d+)\.json$`)
	reTermBank      = regexp.MustCompile(`^term_bank_(\d+)\.json$`)
	reKanjiBank     = regexp.MustCompile(`^kanji_bank_(\d+)\.json$`)
	reTermMetaBank  = regexp.MustCompile(`^term_meta_bank_(\d+)\.json$`)
	reKanjiMetaBank = regexp.MustCompile(`^kanji_meta_bank_(\d+)\.json$`)
)

// Options controls dictionary import behavior.
type Options struct {
	Priority int  // 0 means auto-assign based on title/type
	Replace  bool // Replace existing dictionary with the same title
}

// ImportResult summarizes an imported dictionary.
type ImportResult struct {
	Title      string
	Revision   string
	Tags       int
	Terms      int
	Kanji      int
	TermMeta   int
	KanjiMeta  int
}

type bankSource interface {
	ListFiles() ([]string, error)
	Open(name string) (io.ReadCloser, error)
	Close() error
}

type zipSource struct {
	r     *zip.ReadCloser
	files map[string]*zip.File
}

func openZipSource(path string) (*zipSource, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	files := make(map[string]*zip.File, len(r.File))
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		files[base] = f
	}
	return &zipSource{r: r, files: files}, nil
}

func (z *zipSource) ListFiles() ([]string, error) {
	out := make([]string, 0, len(z.files))
	for name := range z.files {
		out = append(out, name)
	}
	return out, nil
}

func (z *zipSource) Open(name string) (io.ReadCloser, error) {
	f, ok := z.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return f.Open()
}

func (z *zipSource) Close() error {
	return z.r.Close()
}

type dirSource struct {
	root string
}

func (d *dirSource) ListFiles() ([]string, error) {
	entries, err := os.ReadDir(d.root)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Ignore Windows/WSL Zone.Identifier alternate data stream files
		if strings.Contains(name, ":") {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func (d *dirSource) Open(name string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(d.root, name))
}

func (d *dirSource) Close() error { return nil }

// Import loads a Yomitan dictionary (.zip archive or extracted directory) into SQLite.
func Import(database *db.DB, path string, opts Options) (*ImportResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	var src bankSource
	if info.IsDir() {
		src = &dirSource{root: path}
	} else {
		zs, err := openZipSource(path)
		if err != nil {
			return nil, fmt.Errorf("open zip %s: %w", path, err)
		}
		src = zs
	}
	defer src.Close()

	// Read index.json
	idxReader, err := src.Open("index.json")
	if err != nil {
		return nil, fmt.Errorf("open index.json: %w", err)
	}
	var idx IndexJSON
	if err := json.NewDecoder(idxReader).Decode(&idx); err != nil {
		idxReader.Close()
		return nil, fmt.Errorf("decode index.json: %w", err)
	}
	idxReader.Close()

	if idx.Title == "" {
		return nil, fmt.Errorf("index.json missing required title")
	}

	existing, err := database.GetDictionaryByTitle(idx.Title)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !opts.Replace {
			return nil, fmt.Errorf("dictionary %q already imported (use --replace to overwrite)", idx.Title)
		}
		if err := database.DeleteDictionary(idx.Title); err != nil {
			return nil, fmt.Errorf("remove existing dictionary: %w", err)
		}
	}

	priority := opts.Priority
	if priority <= 0 {
		priority = inferDefaultPriority(idx.Title)
	}

	files, err := src.ListFiles()
	if err != nil {
		return nil, err
	}

	tagBanks := sortBankFiles(files, reTagBank)
	termBanks := sortBankFiles(files, reTermBank)
	kanjiBanks := sortBankFiles(files, reKanjiBank)
	termMetaBanks := sortBankFiles(files, reTermMetaBank)
	kanjiMetaBanks := sortBankFiles(files, reKanjiMetaBank)

	if err := database.SetBulkImportMode(true); err != nil {
		return nil, err
	}
	defer func() { _ = database.SetBulkImportMode(false) }()

	tx, err := database.Conn.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	seqInt := 0
	if idx.Sequenced {
		seqInt = 1
	}
	res, err := tx.Exec(`
		INSERT INTO dictionaries (title, revision, format, sequenced, author, description, priority, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)
	`, idx.Title, idx.Revision, idx.Format, seqInt, idx.Author, idx.Description, priority)
	if err != nil {
		return nil, fmt.Errorf("insert dictionary: %w", err)
	}
	dictID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	result := &ImportResult{
		Title:    idx.Title,
		Revision: idx.Revision,
	}

	// 1. Tag banks
	if len(tagBanks) > 0 {
		stmt, err := tx.Prepare(`INSERT OR IGNORE INTO tags (dict_id, name, category, sort_order, notes, score) VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return nil, err
		}
		for _, bf := range tagBanks {
			var entries []TagEntry
			if err := decodeBank(src, bf, &entries); err != nil {
				stmt.Close()
				return nil, fmt.Errorf("%s: %w", bf, err)
			}
			for _, e := range entries {
				if _, err := stmt.Exec(dictID, e.Name, e.Category, e.SortOrder, e.Notes, e.Score); err != nil {
					stmt.Close()
					return nil, err
				}
				result.Tags++
			}
		}
		stmt.Close()
	}

	// 2. Term banks
	if len(termBanks) > 0 {
		stmt, err := tx.Prepare(`
			INSERT INTO terms (dict_id, expression, reading, definition_tags, rules, score, glossary_json, sequence, term_tags)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return nil, err
		}
		for _, bf := range termBanks {
			var entries []TermEntry
			if err := decodeBank(src, bf, &entries); err != nil {
				stmt.Close()
				return nil, fmt.Errorf("%s: %w", bf, err)
			}
			for _, e := range entries {
				if _, err := stmt.Exec(
					dictID, e.Expression, e.Reading, e.DefinitionTags,
					e.Rules, e.Score, e.GlossaryRaw, e.Sequence, e.TermTags,
				); err != nil {
					stmt.Close()
					return nil, err
				}
				result.Terms++
			}
		}
		stmt.Close()
	}

	// 3. Kanji banks
	if len(kanjiBanks) > 0 {
		stmt, err := tx.Prepare(`
			INSERT INTO kanji (dict_id, character, onyomi, kunyomi, tags, meanings_json, stats_json)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return nil, err
		}
		for _, bf := range kanjiBanks {
			var entries []KanjiEntry
			if err := decodeBank(src, bf, &entries); err != nil {
				stmt.Close()
				return nil, fmt.Errorf("%s: %w", bf, err)
			}
			for _, e := range entries {
				if _, err := stmt.Exec(
					dictID, e.Character, e.Onyomi, e.Kunyomi,
					e.Tags, e.MeaningsRaw, e.StatsRaw,
				); err != nil {
					stmt.Close()
					return nil, err
				}
				result.Kanji++
			}
		}
		stmt.Close()
	}

	// 4. Term meta banks
	if len(termMetaBanks) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO term_meta (dict_id, expression, mode, data_json) VALUES (?, ?, ?, ?)`)
		if err != nil {
			return nil, err
		}
		for _, bf := range termMetaBanks {
			var entries []MetaEntry
			if err := decodeBank(src, bf, &entries); err != nil {
				stmt.Close()
				return nil, fmt.Errorf("%s: %w", bf, err)
			}
			for _, e := range entries {
				if _, err := stmt.Exec(dictID, e.Key, e.Mode, e.DataRaw); err != nil {
					stmt.Close()
					return nil, err
				}
				result.TermMeta++
			}
		}
		stmt.Close()
	}

	// 5. Kanji meta banks
	if len(kanjiMetaBanks) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO kanji_meta (dict_id, character, mode, data_json) VALUES (?, ?, ?, ?)`)
		if err != nil {
			return nil, err
		}
		for _, bf := range kanjiMetaBanks {
			var entries []MetaEntry
			if err := decodeBank(src, bf, &entries); err != nil {
				stmt.Close()
				return nil, fmt.Errorf("%s: %w", bf, err)
			}
			for _, e := range entries {
				if _, err := stmt.Exec(dictID, e.Key, e.Mode, e.DataRaw); err != nil {
					stmt.Close()
					return nil, err
				}
				result.KanjiMeta++
			}
		}
		stmt.Close()
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit import: %w", err)
	}
	return result, nil
}

func decodeBank(src bankSource, filename string, target any) error {
	r, err := src.Open(filename)
	if err != nil {
		return err
	}
	defer r.Close()
	return json.NewDecoder(r).Decode(target)
}

func sortBankFiles(files []string, re *regexp.Regexp) []string {
	type pair struct {
		name string
		num  int
	}
	var matched []pair
	for _, f := range files {
		m := re.FindStringSubmatch(f)
		if len(m) == 2 {
			n, _ := strconv.Atoi(m[1])
			matched = append(matched, pair{name: f, num: n})
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].num < matched[j].num
	})
	out := make([]string, len(matched))
	for i, p := range matched {
		out[i] = p.name
	}
	return out
}

func inferDefaultPriority(title string) int {
	low := strings.ToLower(title)
	switch {
	case strings.Contains(low, "jmdict") || strings.Contains(low, "jitendex") || strings.Contains(low, "cc-cedict"):
		return 10
	case strings.Contains(low, "文法") || strings.Contains(low, "grammar"):
		return 20
	case strings.Contains(low, "kanjidic"):
		return 30
	default:
		return 50
	}
}
