package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Marcowich0/hoardboard/internal/tree"
)

// LinkSet is the History of the hardlinked files whose names are in exactly
// Folders: their bytes added up, per day.
type LinkSet struct {
	// Folders are the paths of the Folders their names are in, sorted.
	Folders []string
	Series  Series[int64]
}

// recordLinks records t's hardlinked files as the History of today, the
// part of a past day that Folders' figures can't tell: which categories
// share a file depends on the Folders of its names. Files whose names are in
// the same set of Folders are one row in link_history (their bytes added
// up), keyed by link_sets: the Folders' paths as a sorted JSON array. A set
// no Scan finds any more gets a row of zeros, as a Folder does (changeOn).
func recordLinks(ctx context.Context, tx *sql.Tx, today Day, t *tree.Tree) error {
	now := map[string]int64{}
	paths := map[*tree.Folder]string{}
	var set []string
	for _, l := range t.Hardlinks {
		set = set[:0]
		for _, f := range l.Folders {
			p, ok := paths[f]
			if !ok {
				p = f.Path()
				paths[f] = p
			}
			set = append(set, p)
		}
		slices.Sort(set)
		key, err := json.Marshal(slices.Compact(set))
		if err != nil {
			return err
		}
		now[string(key)] += l.Bytes
	}

	rows, err := tx.QueryContext(ctx, `SELECT s.id, s.folders, b.bytes, t.bytes
		FROM link_sets s
		LEFT JOIN link_history b ON b.set_id = s.id
			AND b.day = (SELECT max(day) FROM link_history WHERE set_id = s.id AND day < ?1)
		LEFT JOIN link_history t ON t.set_id = s.id AND t.day = ?1`, today)
	if err != nil {
		return fmt.Errorf("link history: %w", err)
	}
	type known struct {
		id            int64
		before, today *int64
	}
	byKey := map[string]known{}
	for rows.Next() {
		var (
			key       string
			k         known
			before, t sql.NullInt64
		)
		if err := rows.Scan(&k.id, &key, &before, &t); err != nil {
			rows.Close()
			return fmt.Errorf("link history: %w", err)
		}
		if before.Valid {
			k.before = &before.Int64
		}
		if t.Valid {
			k.today = &t.Int64
		}
		byKey[key] = k
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("link history: %w", err)
	}

	write := func(id, bytes int64) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO link_history (set_id, day, bytes) VALUES (?, ?, ?)
			ON CONFLICT (set_id, day) DO UPDATE SET bytes = excluded.bytes`, id, today, bytes)
		return err
	}
	for key, k := range byKey {
		bytes, found := now[key]
		delete(now, key)
		switch changeOn(k.before, k.today, bytes, found) {
		case writeRow:
			err = write(k.id, bytes)
		case removeRow:
			_, err = tx.ExecContext(ctx, `DELETE FROM link_history WHERE set_id = ? AND day = ?`, k.id, today)
		}
		if err != nil {
			return fmt.Errorf("link history: %w", err)
		}
	}
	for key, bytes := range now {
		res, err := tx.ExecContext(ctx, `INSERT INTO link_sets (folders) VALUES (?)`, key)
		if err != nil {
			return fmt.Errorf("link history: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := write(id, bytes); err != nil {
			return fmt.Errorf("link history: %w", err)
		}
	}
	return nil
}

// ReadLinks returns the History of every set of Folders that hardlinked
// files have had names in. As with Read, a Series holds only the rows from
// the last one on or before from onward: its At is exact for from and
// every later day.
func (s *Store) ReadLinks(ctx context.Context, from Day) ([]LinkSet, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT s.id, s.folders, h.day, h.bytes
		FROM link_sets s
		JOIN link_history h ON h.set_id = s.id
		WHERE h.day >= coalesce((SELECT max(day) FROM link_history WHERE set_id = s.id AND day <= ?1), ?1)
		ORDER BY s.id, h.day`, from)
	if err != nil {
		return nil, fmt.Errorf("link history: %w", err)
	}
	defer rows.Close()
	var (
		out  []LinkSet
		last int64 = -1
	)
	for rows.Next() {
		var (
			id      int64
			folders string
			r       Row[int64]
		)
		if err := rows.Scan(&id, &folders, &r.Day, &r.Value); err != nil {
			return nil, fmt.Errorf("link history: %w", err)
		}
		if id != last {
			last = id
			out = append(out, LinkSet{})
			if err := json.Unmarshal([]byte(folders), &out[len(out)-1].Folders); err != nil {
				return nil, fmt.Errorf("link history: set %d: %w", id, err)
			}
		}
		set := &out[len(out)-1]
		set.Series.Rows = append(set.Series.Rows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("link history: %w", err)
	}
	return out, nil
}
