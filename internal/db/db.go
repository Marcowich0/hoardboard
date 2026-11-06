// Package db opens the application database: one SQLite file,
// hoardboard.db, in the data folder, so a backup is one file. It uses the
// pure-Go modernc.org/sqlite driver (no CGO).
//
// Each module owns its tables and brings them up to date with Migrate under
// its own name, so modules add tables without touching each other's steps.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// FileName is the database file's name inside the data folder.
const FileName = "hoardboard.db"

// Open opens (creating if needed) dataDir/hoardboard.db, creating the data
// folder if it does not exist.
//
// The pool has one connection: writes are serialised and never meet
// SQLITE_BUSY. The other side of that is that a query must not run while
// another's rows are still open.
func Open(dataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("data folder: %w", err)
	}
	abs, err := filepath.Abs(filepath.Join(dataDir, FileName))
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Set("_txlock", "immediate")
	d, err := sql.Open("sqlite", "file:"+abs+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, fmt.Errorf("open %s: %w", abs, err)
	}
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_versions (
		module  TEXT PRIMARY KEY,
		version INTEGER NOT NULL
	)`); err != nil {
		d.Close()
		return nil, fmt.Errorf("open %s: %w", abs, err)
	}
	return d, nil
}

// Migrate brings module's tables up to date by running the steps it has not
// run yet, in order, each in its own transaction. Steps are append-only: never
// change or remove one that has shipped, add a new one instead.
func Migrate(ctx context.Context, d *sql.DB, module string, steps []string) error {
	var done int
	err := d.QueryRowContext(ctx, `SELECT version FROM schema_versions WHERE module = ?`, module).Scan(&done)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("migrate %s: %w", module, err)
	}
	if done > len(steps) {
		return fmt.Errorf("migrate %s: database is at step %d, this build knows %d: it is from a newer hoardboard", module, done, len(steps))
	}
	for i := done; i < len(steps); i++ {
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("migrate %s step %d: %w", module, i+1, err)
		}
		if _, err := tx.ExecContext(ctx, steps[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate %s step %d: %w", module, i+1, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_versions (module, version) VALUES (?, ?)
			ON CONFLICT (module) DO UPDATE SET version = excluded.version`, module, i+1); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate %s step %d: %w", module, i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate %s step %d: %w", module, i+1, err)
		}
	}
	return nil
}
