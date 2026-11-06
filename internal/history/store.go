package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Marcowich0/hoardboard/internal/db"
	"github.com/Marcowich0/hoardboard/internal/scan"
	"github.com/Marcowich0/hoardboard/internal/tree"
)

// migrations are the history module's tables. Append only (see db.Migrate).
//
//   - folders: every Folder path a Scan has seen, stored once, kept forever.
//     "" is the root.
//   - folder_history: a Folder's Size and Shared bytes from day on (days
//     since 1970-01-01, see Day). A row is stored only when the value
//     differs from the Folder's previous stored day; a Folder a Scan no
//     longer finds gets a row of zeros. The primary key makes "one Folder's
//     rows in day order" an index range, whatever the number of Folders.
//   - disk_history: a Disk's Used space from day on, by the Disk's name,
//     with the same write rule (see disks.go).
//   - folder_history.single_bytes: the Folder's SingleBytes, written with
//     its Size; and link_sets and link_history, the hardlinked files (see
//     links.go). With them, a category's past is measured under the current
//     Assignments (docs/adr/0004-category-history.md).
var migrations = []string{
	`CREATE TABLE folders (
		id   INTEGER PRIMARY KEY,
		path TEXT NOT NULL UNIQUE
	);
	CREATE TABLE folder_history (
		folder_id    INTEGER NOT NULL REFERENCES folders (id),
		day          INTEGER NOT NULL,
		size         INTEGER NOT NULL,
		shared_bytes INTEGER NOT NULL,
		PRIMARY KEY (folder_id, day)
	) WITHOUT ROWID;`,
	`CREATE TABLE disk_history (
		disk TEXT    NOT NULL,
		day  INTEGER NOT NULL,
		used INTEGER NOT NULL,
		PRIMARY KEY (disk, day)
	) WITHOUT ROWID;`,
	`ALTER TABLE folder_history ADD COLUMN single_bytes INTEGER NOT NULL DEFAULT 0;
	CREATE TABLE link_sets (
		id      INTEGER PRIMARY KEY,
		folders TEXT NOT NULL UNIQUE
	);
	CREATE TABLE link_history (
		set_id INTEGER NOT NULL REFERENCES link_sets (id),
		day    INTEGER NOT NULL,
		bytes  INTEGER NOT NULL,
		PRIMARY KEY (set_id, day)
	) WITHOUT ROWID;`,
}

// Value is a Folder's figures on one day.
type Value struct {
	Size        int64
	SharedBytes int64
	// SingleBytes is the part of Size in files with one name (see
	// tree.Folder): what the category engine needs of a past day.
	SingleBytes int64
}

// Store keeps the History of every Folder.
type Store struct {
	db *sql.DB
}

// Open brings the history tables up to date.
func Open(ctx context.Context, d *sql.DB) (*Store, error) {
	if err := db.Migrate(ctx, d, "history", migrations); err != nil {
		return nil, err
	}
	return &Store{db: d}, nil
}

// AfterScan records the finished Scan's tree as the History of the day it
// finished (DayOf done.Finished): every Folder's Size, Shared bytes and
// SingleBytes, and the hardlinked files by the Folders of their names, and
// zero for every Folder History knows that the tree no longer holds. A later
// Scan on the same day replaces the day's values, so the last Scan of a day
// wins. It is a scan.AfterScan: it runs inside the transaction that records
// the Scan as finished.
func (s *Store) AfterScan(ctx context.Context, tx *sql.Tx, done scan.Done, t *tree.Tree) error {
	today := DayOf(done.Finished)
	now := map[string]Value{}
	var walk func(f *tree.Folder, path string)
	walk = func(f *tree.Folder, path string) {
		now[path] = Value{Size: f.Size, SharedBytes: f.SharedBytes, SingleBytes: f.SingleBytes}
		for _, c := range f.Folders {
			if path == "" {
				walk(c, c.Name)
			} else {
				walk(c, path+"/"+c.Name)
			}
		}
	}
	walk(t.Root, "")

	known, err := latest(ctx, tx, today)
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}
	upsert, err := tx.PrepareContext(ctx, `INSERT INTO folder_history (folder_id, day, size, shared_bytes, single_bytes)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT (folder_id, day) DO UPDATE
		SET size = excluded.size, shared_bytes = excluded.shared_bytes, single_bytes = excluded.single_bytes`)
	if err != nil {
		return err
	}
	defer upsert.Close()
	remove, err := tx.PrepareContext(ctx, `DELETE FROM folder_history WHERE folder_id = ? AND day = ?`)
	if err != nil {
		return err
	}
	defer remove.Close()
	addFolder, err := tx.PrepareContext(ctx, `INSERT INTO folders (path) VALUES (?)`)
	if err != nil {
		return err
	}
	defer addFolder.Close()

	for path, k := range known {
		v, present := now[path]
		delete(now, path)
		var err error
		switch changeOn(k.before, k.today, v, present) {
		case writeRow:
			_, err = upsert.ExecContext(ctx, k.id, today, v.Size, v.SharedBytes, v.SingleBytes)
		case removeRow:
			_, err = remove.ExecContext(ctx, k.id, today)
		}
		if err != nil {
			return fmt.Errorf("history: %w", err)
		}
	}
	// Folders History has never seen.
	for path, v := range now {
		res, err := addFolder.ExecContext(ctx, path)
		if err != nil {
			return fmt.Errorf("history: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := upsert.ExecContext(ctx, id, today, v.Size, v.SharedBytes, v.SingleBytes); err != nil {
			return fmt.Errorf("history: %w", err)
		}
	}
	return recordLinks(ctx, tx, today, t)
}

// stored is what History holds for one Folder around today.
type stored struct {
	id     int64
	before *Value // the last row before today, nil if none
	today  *Value // today's row, nil if none
}

// latest reads, for every Folder History knows, its last row before today
// and its row for today.
func latest(ctx context.Context, tx *sql.Tx, today Day) (map[string]stored, error) {
	rows, err := tx.QueryContext(ctx, `SELECT f.id, f.path, b.size, b.shared_bytes, b.single_bytes,
			t.size, t.shared_bytes, t.single_bytes
		FROM folders f
		LEFT JOIN folder_history b ON b.folder_id = f.id
			AND b.day = (SELECT max(day) FROM folder_history WHERE folder_id = f.id AND day < ?1)
		LEFT JOIN folder_history t ON t.folder_id = f.id AND t.day = ?1`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]stored{}
	for rows.Next() {
		var (
			path                    string
			k                       stored
			bSize, bShared, bSingle sql.NullInt64
			tSize, tShared, tSingle sql.NullInt64
		)
		if err := rows.Scan(&k.id, &path, &bSize, &bShared, &bSingle, &tSize, &tShared, &tSingle); err != nil {
			return nil, err
		}
		if bSize.Valid {
			k.before = &Value{Size: bSize.Int64, SharedBytes: bShared.Int64, SingleBytes: bSingle.Int64}
		}
		if tSize.Valid {
			k.today = &Value{Size: tSize.Int64, SharedBytes: tShared.Int64, SingleBytes: tSingle.Int64}
		}
		out[path] = k
	}
	return out, rows.Err()
}

// Read returns the History of each of paths that History knows, keyed by
// path. To keep reads short, a Series holds only the rows from the last one
// on or before from onward: its At is exact for from and every later day.
// Pass from 0 for a Folder's whole History.
//
// It reads every Folder in one query, so a category's History over many
// Folders costs one index range per Folder.
func (s *Store) Read(ctx context.Context, paths []string, from Day) (map[string]Series[Value], error) {
	list, err := json.Marshal(paths)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT f.path, h.day, h.size, h.shared_bytes, h.single_bytes
		FROM json_each(?1) p
		JOIN folders f ON f.path = p.value
		JOIN folder_history h ON h.folder_id = f.id
		WHERE h.day >= coalesce((SELECT max(day) FROM folder_history WHERE folder_id = f.id AND day <= ?2), ?2)
		ORDER BY f.id, h.day`, string(list), from)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	defer rows.Close()
	out := map[string]Series[Value]{}
	for rows.Next() {
		var (
			path string
			r    Row[Value]
		)
		if err := rows.Scan(&path, &r.Day, &r.Value.Size, &r.Value.SharedBytes, &r.Value.SingleBytes); err != nil {
			return nil, fmt.Errorf("history: %w", err)
		}
		series := out[path]
		series.Rows = append(series.Rows, r)
		out[path] = series
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	return out, nil
}

// Start is the first day of History, the day the first Scan finished; ok is
// false before any Scan has. Before Start, History knows nothing (At's zero
// does not mean "empty" there).
func (s *Store) Start(ctx context.Context) (d Day, ok bool, err error) {
	var day sql.NullInt64
	// The root is in every Scan's tree, so its first row is the first Scan's.
	err = s.db.QueryRowContext(ctx, `SELECT min(h.day) FROM folder_history h
		JOIN folders f ON f.id = h.folder_id WHERE f.path = ''`).Scan(&day)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !day.Valid) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("history: %w", err)
	}
	return Day(day.Int64), true, nil
}
