package history

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Rename records that the Folder at Path was the Folder at Was before Day:
// the owner re-pointed an Assignment from Was, Missing since a Scan, to
// Path. Day is the day Path appeared (Store.Rename), so Path's past is
// Was's for the days before Path existed (docs/adr/0006-re-point-carries-history.md).
// It renames everything below Path too: Path/x was Was/x.
type Rename struct {
	Path, Was string
	Day       Day
}

// Renames is every Rename, in the order they take effect: by Day, then in
// the order the owner made them.
type Renames []Rename

// Rename records, inside tx, that the Folder at path was the Folder at was
// before path last appeared: the day of path's first row after its last
// row of zeros (a Folder a Scan no longer found), or of its first row.
// History must know path (it is in the latest Scan's tree).
func (s *Store) Rename(ctx context.Context, tx *sql.Tx, path, was string) error {
	rows, err := tx.QueryContext(ctx, `SELECT h.day, h.size FROM folder_history h
		JOIN folders f ON f.id = h.folder_id WHERE f.path = ? ORDER BY h.day`, path)
	if err != nil {
		return fmt.Errorf("history: rename: %w", err)
	}
	var (
		appeared   Day
		known, gap bool
	)
	for rows.Next() {
		var (
			d    Day
			size int64
		)
		if err := rows.Scan(&d, &size); err != nil {
			rows.Close()
			return fmt.Errorf("history: rename: %w", err)
		}
		if !known || gap {
			appeared, known = d, true
		}
		gap = size == 0
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("history: rename: %w", err)
	}
	if !known {
		return fmt.Errorf("history: rename: no History of %s", path)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO renames (path, was, day) VALUES (?, ?, ?)`,
		path, was, appeared); err != nil {
		return fmt.Errorf("history: rename: %w", err)
	}
	return nil
}

// Renames reads every Rename.
func (s *Store) Renames(ctx context.Context) (Renames, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, was, day FROM renames ORDER BY day, id`)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	defer rows.Close()
	var out Renames
	for rows.Next() {
		var r Rename
		if err := rows.Scan(&r.Path, &r.Was, &r.Day); err != nil {
			return nil, fmt.Errorf("history: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// At is the path the Folder at path had on day d.
func (rs Renames) At(path string, d Day) string {
	return rs.at(path, d, len(rs))
}

// at follows only rs[:limit]: going back in time, a Rename leads to the
// path before it, where only the Renames that took effect before it apply.
// So a Folder renamed back and forth still ends.
func (rs Renames) at(path string, d Day, limit int) string {
	for i := limit - 1; i >= 0; i-- {
		r := rs[i]
		if d >= r.Day {
			continue
		}
		if rest, ok := below(path, r.Path); ok {
			return rs.at(r.Was+rest, d, i)
		}
	}
	return path
}

// span is one of the paths a Folder had, from day from on.
type span struct {
	from Day
	path string
	// first is the earliest span, which has no start.
	first bool
}

// spans are the paths the Folder at path had, oldest first: the same answer
// as At, for every day at once.
func (rs Renames) spans(path string) []span {
	return rs.spansBefore(path, len(rs), 0, false)
}

// spansBefore is spans over rs[:limit], for the days before end (every
// day if !bounded).
func (rs Renames) spansBefore(path string, limit int, end Day, bounded bool) []span {
	for i := limit - 1; i >= 0; i-- {
		r := rs[i]
		rest, ok := below(path, r.Path)
		if !ok {
			continue
		}
		if bounded && r.Day >= end {
			// Every day before end is before r: path was r.Was+rest.
			return rs.spansBefore(r.Was+rest, i, end, true)
		}
		return append(rs.spansBefore(r.Was+rest, i, r.Day, true), span{from: r.Day, path: path})
	}
	return []span{{path: path, first: true}}
}

// splice is the Series of a Folder that had the paths of spans, out of each
// path's own stored rows (raw).
func splice(spans []span, raw map[string]Series[Value]) Series[Value] {
	var out Series[Value]
	for i, sp := range spans {
		own := raw[sp.path]
		start := 0
		if !sp.first {
			// The value at the span's start, unless nothing came before
			// and there is nothing yet.
			if v := own.At(sp.from); len(out.Rows) > 0 || v != (Value{}) {
				out.Rows = append(out.Rows, Row[Value]{Day: sp.from, Value: v})
			}
			for start < len(own.Rows) && own.Rows[start].Day <= sp.from {
				start++
			}
		}
		for _, r := range own.Rows[start:] {
			if i+1 < len(spans) && r.Day >= spans[i+1].from {
				break
			}
			out.Rows = append(out.Rows, r)
		}
	}
	return out
}

// below is the rest of path after folder ("" or "/…"), ok false if path is
// not folder or inside it.
func below(path, folder string) (rest string, ok bool) {
	if path == folder {
		return "", true
	}
	if strings.HasPrefix(path, folder+"/") {
		return path[len(folder):], true
	}
	return "", false
}
