package history

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Marcowich0/hoardboard/internal/inventory"
	"github.com/Marcowich0/hoardboard/internal/scan"
	"github.com/Marcowich0/hoardboard/internal/tree"
)

// The daily History of each Disk's Used space, which the growth rate and the
// forecast are fitted to (docs/adr/0004-daily-used-space-per-disk.md).
// Folder History can't stand in for it: Unaccounted is part of Used space.

// UsedSpace is a scan.AfterScan that records every Disk's Used space, as its
// filesystem reports it when the Scan finishes, as the Disk's value for the
// day the Scan finished. It follows the Folders' write rule (changeOn): the
// last Scan of a day wins, an unchanged day has no row, and a Disk Unraid no
// longer lists gets a row of zeros, once. A Disk Unraid lists but whose
// filesystem can't be asked keeps its rows as they are.
func (s *Store) UsedSpace(inv *inventory.Inventory) scan.AfterScan {
	return func(ctx context.Context, tx *sql.Tx, done scan.Done, _ *tree.Tree) error {
		arr, err := inv.Read()
		if err != nil {
			return fmt.Errorf("used space history: %w", err)
		}
		listed := map[string]bool{}
		now := map[string]int64{}
		for _, d := range arr.Disks {
			listed[d.Name] = true
			if sp, err := d.Space(); err == nil {
				now[d.Name] = sp.Used
			}
		}
		today := DayOf(done.Finished)
		known, err := latestUsed(ctx, tx, today)
		if err != nil {
			return fmt.Errorf("used space history: %w", err)
		}
		for disk := range now {
			if _, ok := known[disk]; !ok {
				known[disk] = storedUsed{}
			}
		}
		for disk, k := range known {
			used, found := now[disk]
			if listed[disk] && !found {
				continue // not measured this time
			}
			switch changeOn(k.before, k.today, used, found) {
			case writeRow:
				_, err = tx.ExecContext(ctx, `INSERT INTO disk_history (disk, day, used) VALUES (?, ?, ?)
					ON CONFLICT (disk, day) DO UPDATE SET used = excluded.used`, disk, today, used)
			case removeRow:
				_, err = tx.ExecContext(ctx, `DELETE FROM disk_history WHERE disk = ? AND day = ?`, disk, today)
			}
			if err != nil {
				return fmt.Errorf("used space history: %w", err)
			}
		}
		return nil
	}
}

// storedUsed is what History holds for one Disk around today.
type storedUsed struct {
	before *int64 // the last row before today, nil if none
	today  *int64 // today's row, nil if none
}

// latestUsed reads, for every Disk History knows, its last row before today
// and its row for today.
func latestUsed(ctx context.Context, tx *sql.Tx, today Day) (map[string]storedUsed, error) {
	rows, err := tx.QueryContext(ctx, `SELECT d.disk, b.used, t.used
		FROM (SELECT DISTINCT disk FROM disk_history) d
		LEFT JOIN disk_history b ON b.disk = d.disk
			AND b.day = (SELECT max(day) FROM disk_history WHERE disk = d.disk AND day < ?1)
		LEFT JOIN disk_history t ON t.disk = d.disk AND t.day = ?1`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]storedUsed{}
	for rows.Next() {
		var (
			disk          string
			before, today sql.NullInt64
			k             storedUsed
		)
		if err := rows.Scan(&disk, &before, &today); err != nil {
			return nil, err
		}
		if before.Valid {
			k.before = &before.Int64
		}
		if today.Valid {
			k.today = &today.Int64
		}
		out[disk] = k
	}
	return out, rows.Err()
}

// TotalUsed is the array's total Used space per day, the Disks' values
// carried forward and added up, for the last days days up to today (both
// included), but from no earlier than the first day any Scan recorded: the
// daily points the forecast is fitted to. It is empty before the first Scan.
func (s *Store) TotalUsed(ctx context.Context, today Day, days int) ([]Row[int64], error) {
	from := today - Day(days) + 1
	rows, err := s.db.QueryContext(ctx, `SELECT h.disk, h.day, h.used FROM disk_history h
		WHERE h.day >= coalesce((SELECT max(day) FROM disk_history WHERE disk = h.disk AND day <= ?1), ?1)
		ORDER BY h.disk, h.day`, from)
	if err != nil {
		return nil, fmt.Errorf("used space history: %w", err)
	}
	defer rows.Close()
	byDisk := map[string]Series[int64]{}
	first, seen := Day(0), false
	for rows.Next() {
		var (
			disk string
			r    Row[int64]
		)
		if err := rows.Scan(&disk, &r.Day, &r.Value); err != nil {
			return nil, fmt.Errorf("used space history: %w", err)
		}
		series := byDisk[disk]
		series.Rows = append(series.Rows, r)
		byDisk[disk] = series
		if !seen || r.Day < first {
			first, seen = r.Day, true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("used space history: %w", err)
	}
	if !seen {
		return nil, nil
	}
	total := Series[int64]{}.Daily(max(first, from), today)
	for _, series := range byDisk {
		for i, r := range series.Daily(max(first, from), today) {
			total[i].Value += r.Value
		}
	}
	return total, nil
}
