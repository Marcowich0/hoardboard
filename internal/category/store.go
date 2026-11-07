package category

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Marcowich0/hoardboard/internal/db"
)

// migrations are the category module's tables. Append only (see db.Migrate).
//
//   - categories: rank is the position in the owner's order, 1 the highest;
//     ranks stay 1…n.
//   - assignments: one per Folder (path is unique); deleting a category
//     deletes its Assignments.
var migrations = []string{
	`CREATE TABLE categories (
		id     INTEGER PRIMARY KEY,
		name   TEXT NOT NULL,
		colour TEXT NOT NULL,
		rank   INTEGER NOT NULL
	);
	CREATE TABLE assignments (
		path        TEXT PRIMARY KEY,
		category_id INTEGER NOT NULL REFERENCES categories (id) ON DELETE CASCADE
	);
	CREATE INDEX assignments_category ON assignments (category_id);`,
}

// Category is one of the owner's categories.
type Category struct {
	ID     ID
	Name   string
	Colour string // "#rrggbb"
	Rank   int    // 1 is the highest
}

// Input is a new category's name and colour, or the changes to one (nil
// keeps the field).
type Input struct {
	Name, Colour *string
}

// The kinds of error the Store answers, for errors.Is; the error's message
// says what is wrong in words for the owner.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid")
	ErrConflict = errors.New("conflict")
)

type problem struct {
	kind error
	msg  string
}

func (p *problem) Error() string { return p.msg }
func (p *problem) Unwrap() error { return p.kind }

func fail(kind error, format string, args ...any) error {
	return &problem{kind: kind, msg: fmt.Sprintf(format, args...)}
}

// Store keeps the Categories and Assignments in the database.
type Store struct {
	db *sql.DB
}

// Open brings the category tables up to date.
func Open(ctx context.Context, d *sql.DB) (*Store, error) {
	if err := db.Migrate(ctx, d, "category", migrations); err != nil {
		return nil, err
	}
	return &Store{db: d}, nil
}

// List is every Category, highest Rank first.
func (s *Store) List(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, colour, rank FROM categories ORDER BY rank, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Colour, &c.Rank); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Assignments is every Assignment.
func (s *Store) Assignments(ctx context.Context) (Assignments, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, category_id FROM assignments`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := Assignments{}
	for rows.Next() {
		var path string
		var id ID
		if err := rows.Scan(&path, &id); err != nil {
			return nil, err
		}
		out[path] = id
	}
	return out, rows.Err()
}

// Create adds a category below every other one in Rank. Name and colour
// are both needed.
func (s *Store) Create(ctx context.Context, in Input) (ID, error) {
	if in.Name == nil || in.Colour == nil {
		return 0, fail(ErrInvalid, "A category needs a name and a colour")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	name, colour, err := check(ctx, tx, in, 0)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO categories (name, colour, rank)
		VALUES (?, ?, (SELECT COALESCE(MAX(rank), 0) + 1 FROM categories))`, name, colour)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return ID(id), tx.Commit()
}

// Update renames and/or recolours the category id.
func (s *Store) Update(ctx context.Context, id ID, in Input) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := exists(ctx, tx, id); err != nil {
		return err
	}
	name, colour, err := check(ctx, tx, in, id)
	if err != nil {
		return err
	}
	if in.Name != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE categories SET name = ? WHERE id = ?`, name, id); err != nil {
			return err
		}
	}
	if in.Colour != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE categories SET colour = ? WHERE id = ?`, colour, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Delete removes the category id and its Assignments; the categories
// below it move up one Rank.
func (s *Store) Delete(ctx context.Context, id ID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var rank int
	err = tx.QueryRowContext(ctx, `DELETE FROM categories WHERE id = ? RETURNING rank`, id).Scan(&rank)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(ErrNotFound, "No category %d", id)
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE categories SET rank = rank - 1 WHERE rank > ?`, rank); err != nil {
		return err
	}
	return tx.Commit()
}

// Reorder sets every category's Rank: ids is every category id once,
// highest Rank first.
func (s *Store) Reorder(ctx context.Context, ids []ID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories`).Scan(&n); err != nil {
		return err
	}
	if len(ids) != n {
		return fail(ErrInvalid, "The order must name every category once: there are %d, it names %d", n, len(ids))
	}
	seen := make(map[ID]bool, len(ids))
	for i, id := range ids {
		if seen[id] {
			return fail(ErrInvalid, "The order names category %d twice", id)
		}
		seen[id] = true
		res, err := tx.ExecContext(ctx, `UPDATE categories SET rank = ? WHERE id = ?`, i+1, id)
		if err != nil {
			return err
		}
		if changed, err := res.RowsAffected(); err != nil {
			return err
		} else if changed == 0 {
			return fail(ErrInvalid, "The order names category %d, which doesn't exist", id)
		}
	}
	return tx.Commit()
}

// Assign puts the Folder at path into the category id, replacing its own
// Assignment if it has one. Subfolders assigned elsewhere keep theirs.
func (s *Store) Assign(ctx context.Context, path string, id ID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := exists(ctx, tx, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO assignments (path, category_id) VALUES (?, ?)
		ON CONFLICT (path) DO UPDATE SET category_id = excluded.category_id`, path, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Unassign removes the Assignment of the Folder at path.
func (s *Store) Unassign(ctx context.Context, path string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM assignments WHERE path = ?`, path)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fail(ErrNotFound, "%s has no Assignment", path)
	}
	return nil
}

// Repoint moves the Assignment of the Folder at from to the Folder at to,
// which must have none of its own. also runs inside the same transaction,
// so what it records (History's rename, see history.Store.Rename) stands or
// falls with the move.
func (s *Store) Repoint(ctx context.Context, from, to string, also func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var taken int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM assignments WHERE path = ?`, to).Scan(&taken); err != nil {
		return err
	}
	if taken > 0 {
		return fail(ErrConflict, "%s already has an Assignment", to)
	}
	res, err := tx.ExecContext(ctx, `UPDATE assignments SET path = ? WHERE path = ?`, to, from)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fail(ErrNotFound, "%s has no Assignment", from)
	}
	if also != nil {
		if err := also(tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func exists(ctx context.Context, tx *sql.Tx, id ID) error {
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM categories WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(ErrNotFound, "No category %d", id)
	}
	return err
}

var colourPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// check validates in for the category id (0 for a new one) and returns its
// name, trimmed, and colour, lower case.
func check(ctx context.Context, tx *sql.Tx, in Input, id ID) (name, colour string, err error) {
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" {
			return "", "", fail(ErrInvalid, "A category needs a name")
		}
		if strings.EqualFold(name, "Uncategorized") {
			return "", "", fail(ErrInvalid, "Uncategorized is built in")
		}
		taken, err := nameTaken(ctx, tx, name, id)
		if err != nil {
			return "", "", err
		}
		if taken {
			return "", "", fail(ErrConflict, "There is already a category called %s", name)
		}
	}
	if in.Colour != nil {
		if !colourPattern.MatchString(*in.Colour) {
			return "", "", fail(ErrInvalid, `A colour is "#rrggbb"`)
		}
		colour = strings.ToLower(*in.Colour)
	}
	return name, colour, nil
}

// nameTaken is whether a category other than id is called name, ignoring
// case (Unicode-aware, which SQLite's lower() is not).
func nameTaken(ctx context.Context, tx *sql.Tx, name string, id ID) (bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM categories WHERE id != ?`, id)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var other string
		if err := rows.Scan(&other); err != nil {
			return false, err
		}
		if strings.EqualFold(other, name) {
			return true, nil
		}
	}
	return false, rows.Err()
}
