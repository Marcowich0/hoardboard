package history

// change is what a Scan does to one thing's row for the day it finished.
type change int

const (
	leaveRow  change = iota // the stored rows already say it
	writeRow                // write (or overwrite) the day's row with the value
	removeRow               // delete the day's row: the value carried forward says it
)

// changeOn is the write rule every daily History shares (a Folder's Size,
// a Disk's Used space; docs/adr/0003-folder-history.md): a day's row is
// stored only when its value differs from the last row before the day, and
// removed again when a later Scan that day brings the value back to it, so
// the last Scan of a day wins and an unchanged day has no row.
//
// before is the last row before the day and today the day's row, nil where
// there is none. v is the value the Scan found, with found false when the
// Scan no longer found the thing at all: then v must be the zero value, and
// a thing that had a value before the day gets a row of zeros, once.
func changeOn[V comparable](before, today *V, v V, found bool) change {
	keep := found || before != nil
	if keep && (before == nil || *before != v) {
		if today == nil || *today != v {
			return writeRow
		}
		return leaveRow
	}
	if today != nil {
		return removeRow
	}
	return leaveRow
}
