package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps the SQLite database connection.
type DB struct {
	Conn *sql.DB
	Path string
}

// Open opens (or creates) the SQLite database at path and ensures the schema is applied.
func Open(path string) (*DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Single writer / WAL friendly settings for sub-20ms lookups
	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA cache_size = -32000;",
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			conn.Close()
			return nil, fmt.Errorf("exec %s: %w", p, err)
		}
	}

	if _, err := conn.Exec(schemaSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &DB{Conn: conn, Path: path}, nil
}

// Close closes the underlying SQLite connection.
func (d *DB) Close() error {
	if d.Conn == nil {
		return nil
	}
	return d.Conn.Close()
}

// SetBulkImportMode tunes PRAGMAs for fast bulk inserts during dictionary import.
func (d *DB) SetBulkImportMode(enabled bool) error {
	if enabled {
		_, err := d.Conn.Exec(`
			PRAGMA synchronous = OFF;
			PRAGMA temp_store = MEMORY;
			PRAGMA cache_size = -64000;
		`)
		return err
	}
	_, err := d.Conn.Exec(`
		PRAGMA synchronous = NORMAL;
		PRAGMA cache_size = -32000;
	`)
	return err
}
