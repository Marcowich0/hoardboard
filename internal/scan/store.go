package scan

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Marcowich0/hoardboard/internal/db"
	"github.com/Marcowich0/hoardboard/internal/tree"
)

// migrations are the scan module's tables. Append only (see db.Migrate).
//
//   - scans: one row per Scan, times in Unix milliseconds. A row stays
//     'running' only while its Scan runs; one found at startup was cut off
//     and becomes 'interrupted'. 'skipped' rows (array work in progress) carry
//     skip_reason.
//   - snapshot: the latest finished Scan's tree, files included, in
//     tree.Encode's format: one row, replaced by every Scan, so that a
//     restart serves the Explorer before the next Scan finishes.
var migrations = []string{
	`CREATE TABLE scans (
		id          INTEGER PRIMARY KEY,
		started     INTEGER NOT NULL,
		finished    INTEGER,
		status      TEXT NOT NULL CHECK (status IN ('running', 'finished', 'failed', 'interrupted', 'skipped')),
		skip_reason TEXT
	);
	CREATE TABLE snapshot (
		id      INTEGER PRIMARY KEY CHECK (id = 1),
		scan_id INTEGER NOT NULL REFERENCES scans (id),
		tree    BLOB NOT NULL
	);`,
}

// openStore brings the tables up to date and marks Scans that a stop cut
// off as interrupted.
func openStore(ctx context.Context, d *sql.DB, now time.Time) error {
	if err := db.Migrate(ctx, d, "scan", migrations); err != nil {
		return err
	}
	_, err := d.ExecContext(ctx, `UPDATE scans SET status = 'interrupted', finished = ? WHERE status = 'running'`, now.UnixMilli())
	return err
}

// insertRunning records a Scan that starts now and returns its id.
func insertRunning(ctx context.Context, d *sql.DB, started time.Time) (int64, error) {
	res, err := d.ExecContext(ctx, `INSERT INTO scans (started, status) VALUES (?, 'running')`, started.UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("record scan: %w", err)
	}
	return res.LastInsertId()
}

// insertSkipped records a Scan skipped at because of array work, and returns
// its id.
func insertSkipped(ctx context.Context, d *sql.DB, at time.Time, reason string) (int64, error) {
	res, err := d.ExecContext(ctx, `INSERT INTO scans (started, finished, status, skip_reason)
		VALUES (?, ?, 'skipped', ?)`, at.UnixMilli(), at.UnixMilli(), reason)
	if err != nil {
		return 0, fmt.Errorf("record skipped scan: %w", err)
	}
	return res.LastInsertId()
}

// lastSkipped is the latest skipped Scan, ok false if none was.
func lastSkipped(ctx context.Context, d *sql.DB) (Skip, bool, error) {
	var id, at int64
	var reason string
	err := d.QueryRowContext(ctx, `SELECT id, started, skip_reason FROM scans
		WHERE status = 'skipped' ORDER BY id DESC LIMIT 1`).Scan(&id, &at, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return Skip{}, false, nil
	}
	if err != nil {
		return Skip{}, false, err
	}
	return Skip{ID: id, At: time.UnixMilli(at), Reason: reason}, true, nil
}

// markFailed records that the Scan id ended without a tree.
func markFailed(ctx context.Context, d *sql.DB, id int64, finished time.Time) error {
	_, err := d.ExecContext(ctx, `UPDATE scans SET status = 'failed', finished = ? WHERE id = ?`, finished.UnixMilli(), id)
	return err
}

// lastFinished is the latest Scan that finished, ok false if none has.
func lastFinished(ctx context.Context, d *sql.DB) (Done, bool, error) {
	var id, started, finished int64
	err := d.QueryRowContext(ctx, `SELECT id, started, finished FROM scans
		WHERE status = 'finished' ORDER BY id DESC LIMIT 1`).Scan(&id, &started, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return Done{}, false, nil
	}
	if err != nil {
		return Done{}, false, err
	}
	return Done{ID: id, Started: time.UnixMilli(started), Finished: time.UnixMilli(finished)}, true, nil
}

// loadSnapshot reads the saved tree, nil if there is none.
func loadSnapshot(ctx context.Context, d *sql.DB) (*tree.Tree, error) {
	var blob []byte
	err := d.QueryRowContext(ctx, `SELECT tree FROM snapshot WHERE id = 1`).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return tree.Decode(bytes.NewReader(blob))
}

// saveFinished records the finished Scan and replaces the snapshot with its
// tree, then runs the hooks, all in one transaction.
func saveFinished(ctx context.Context, d *sql.DB, done Done, t *tree.Tree, hooks []AfterScan) error {
	var blob bytes.Buffer
	if err := tree.Encode(&blob, t); err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE scans SET status = 'finished', finished = ? WHERE id = ?`,
		done.Finished.UnixMilli(), done.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO snapshot (id, scan_id, tree) VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET scan_id = excluded.scan_id, tree = excluded.tree`,
		done.ID, blob.Bytes()); err != nil {
		return err
	}
	for _, hook := range hooks {
		if err := hook(ctx, tx, done, t); err != nil {
			return err
		}
	}
	return tx.Commit()
}
