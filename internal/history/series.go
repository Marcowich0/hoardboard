// Package history is the daily History of every Folder: its Size and Shared
// bytes, one value per day, kept forever (docs/adr/0003-folder-history.md).
//
// A day's row is stored only when its value differs from the previous stored
// day's, so the stored rows of a Folder are sparse. Series is the one place
// that turns them back into a value per day, carrying the last stored value
// forward: every reader (a Folder's History, the change over a period, a
// category's History) goes through Series.At or Series.Daily.
package history

import (
	"slices"
	"time"
)

// Day is a calendar day, counted in days since 1970-01-01. Which day a moment
// falls on is decided in the moment's own time zone (DayOf): for the server,
// its local time zone (TZ).
type Day int32

// DayOf is the day t falls on in t's time zone.
func DayOf(t time.Time) Day {
	y, m, d := t.Date()
	return Day(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

// String is the day as "YYYY-MM-DD".
func (d Day) String() string {
	return time.Unix(int64(d)*86400, 0).UTC().Format(time.DateOnly)
}

// Row is a stored value: from Day on, until the next Row, the value was V.
type Row[V any] struct {
	Day   Day
	Value V
}

// Series is the stored rows of one thing's History (a Folder's, a Disk's),
// oldest first, at most one per day.
type Series[V any] struct {
	Rows []Row[V]
}

// At is the value on day d: the last stored value on or before d, carried
// forward. Before the first Row it is V's zero value: the thing did not
// exist yet (or History had not started; see Store.Start).
func (s Series[V]) At(d Day) V {
	i, found := slices.BinarySearchFunc(s.Rows, d, func(r Row[V], d Day) int { return int(r.Day - d) })
	if found {
		return s.Rows[i].Value
	}
	if i == 0 {
		var zero V
		return zero
	}
	return s.Rows[i-1].Value
}

// First is the day of the first Row, ok false if there is none.
func (s Series[V]) First() (d Day, ok bool) {
	if len(s.Rows) == 0 {
		return 0, false
	}
	return s.Rows[0].Day, true
}

// Last is the day of the last Row, ok false if there is none.
func (s Series[V]) Last() (d Day, ok bool) {
	if len(s.Rows) == 0 {
		return 0, false
	}
	return s.Rows[len(s.Rows)-1].Day, true
}

// Daily is one Row per day from from to to, both included, each with At's
// value: the continuous series the stored rows stand for. It is empty if to
// is before from.
func (s Series[V]) Daily(from, to Day) []Row[V] {
	if to < from {
		return nil
	}
	out := make([]Row[V], 0, int(to-from)+1)
	i := 0
	var v V
	for d := from; d <= to; d++ {
		for i < len(s.Rows) && s.Rows[i].Day <= d {
			v = s.Rows[i].Value
			i++
		}
		out = append(out, Row[V]{Day: d, Value: v})
	}
	return out
}
