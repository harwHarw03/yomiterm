PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS dictionaries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL UNIQUE,
    revision    TEXT NOT NULL DEFAULT '',
    format      INTEGER NOT NULL DEFAULT 3,
    sequenced   INTEGER NOT NULL DEFAULT 0,
    author      TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    priority    INTEGER NOT NULL DEFAULT 100,
    enabled     INTEGER NOT NULL DEFAULT 1,
    imported_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tags (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    dict_id     INTEGER NOT NULL REFERENCES dictionaries(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT '',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    notes       TEXT NOT NULL DEFAULT '',
    score       INTEGER NOT NULL DEFAULT 0,
    UNIQUE(dict_id, name)
);

CREATE TABLE IF NOT EXISTS terms (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    dict_id         INTEGER NOT NULL REFERENCES dictionaries(id) ON DELETE CASCADE,
    expression      TEXT NOT NULL,
    reading         TEXT NOT NULL,
    definition_tags TEXT NOT NULL DEFAULT '',
    rules           TEXT NOT NULL DEFAULT '',
    score           INTEGER NOT NULL DEFAULT 0,
    glossary_json   TEXT NOT NULL,
    sequence        INTEGER NOT NULL DEFAULT 0,
    term_tags       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS kanji (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    dict_id       INTEGER NOT NULL REFERENCES dictionaries(id) ON DELETE CASCADE,
    character     TEXT NOT NULL,
    onyomi        TEXT NOT NULL DEFAULT '',
    kunyomi       TEXT NOT NULL DEFAULT '',
    tags          TEXT NOT NULL DEFAULT '',
    meanings_json TEXT NOT NULL,
    stats_json    TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS term_meta (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    dict_id     INTEGER NOT NULL REFERENCES dictionaries(id) ON DELETE CASCADE,
    expression  TEXT NOT NULL,
    mode        TEXT NOT NULL,
    data_json   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS kanji_meta (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    dict_id     INTEGER NOT NULL REFERENCES dictionaries(id) ON DELETE CASCADE,
    character   TEXT NOT NULL,
    mode        TEXT NOT NULL,
    data_json   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_terms_expression ON terms(expression);
CREATE INDEX IF NOT EXISTS idx_terms_reading    ON terms(reading);
CREATE INDEX IF NOT EXISTS idx_terms_dict_seq   ON terms(dict_id, sequence);
CREATE INDEX IF NOT EXISTS idx_kanji_character  ON kanji(character);
CREATE INDEX IF NOT EXISTS idx_term_meta_expr   ON term_meta(expression);
CREATE INDEX IF NOT EXISTS idx_kanji_meta_char  ON kanji_meta(character);
