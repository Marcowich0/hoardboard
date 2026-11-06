package category

import (
	"context"
	"maps"
	"slices"

	"github.com/Marcowich0/hoardboard/internal/history"
)

// History is what the engine reads of History to measure a past day;
// history.Store has it.
type History interface {
	// Start is the first day of History, ok false before the first Scan.
	Start(ctx context.Context) (d history.Day, ok bool, err error)
	// Read is the History of each of paths, following renames.
	Read(ctx context.Context, paths []string, from history.Day) (map[string]history.Series[history.Value], error)
	// ReadLinks is the History of the hardlinked files by their Folders.
	ReadLinks(ctx context.Context, from history.Day) ([]history.LinkSet, error)
	// Renames is every rename a re-point recorded.
	Renames(ctx context.Context) (history.Renames, error)
}

// Past is what each category took on every day of History.
type Past struct {
	// From is the first day, the first Scan's; Sizes[i] is day From+i, the
	// last one today.
	From  history.Day
	Sizes []*Sizes
}

// MeasurePast rebuilds each day's bytes from h (every Folder's SingleBytes,
// and the hardlinked files with the Folders of their names), and measures
// them under the Assignments a, the way Measure measures the latest Scan,
// from the first day of History to today. Before the first Scan it holds no
// day.
func MeasurePast(ctx context.Context, h History, a Assignments, today history.Day) (Past, error) {
	start, ok, err := h.Start(ctx)
	if err != nil || !ok {
		return Past{}, err
	}
	paths := []string{""}
	for path := range a {
		paths = append(paths, path)
	}
	folders, err := h.Read(ctx, paths, start)
	if err != nil {
		return Past{}, err
	}
	links, err := h.ReadLinks(ctx, start)
	if err != nil {
		return Past{}, err
	}
	renames, err := h.Renames(ctx)
	if err != nil {
		return Past{}, err
	}
	p := Past{From: start}
	var (
		day history.Day
		// assignedThen is a under the paths their Folders had on day;
		// todayPath maps each of those paths back to today's.
		assignedThen, todayPath = a, map[string]string{}
	)
	b := Bytes{Single: func(path string) int64 {
		if p, ok := todayPath[path]; ok {
			path = p
		}
		return folders[path].At(day).SingleBytes
	}}
	for day = start; day <= max(start, today); day++ {
		if len(renames) > 0 {
			assignedThen, todayPath = a.on(renames, day)
		}
		b.Links = b.Links[:0]
		for _, l := range links {
			if n := l.Series.At(day); n != 0 {
				b.Links = append(b.Links, Link{Bytes: n, Folders: l.Folders})
			}
		}
		p.Sizes = append(p.Sizes, MeasureBytes(b, assignedThen))
	}
	return p, nil
}

// on is the Assignments a under the paths their Folders had on day
// (history.Renames.At), so that hardlinked files, recorded by the paths
// their Folders had then, meet the right category. todayPath maps each of
// those paths back to today's, whose History (history.Store.Read) follows
// the renames. Should a renamed Folder's old path also be assigned itself,
// the renamed Folder's Assignment wins: the owner re-pointed that past to
// it.
func (a Assignments) on(renames history.Renames, day history.Day) (assignedThen Assignments, todayPath map[string]string) {
	assignedThen = make(Assignments, len(a))
	todayPath = make(map[string]string, len(a))
	paths := slices.Sorted(maps.Keys(a))
	for _, renamed := range []bool{false, true} {
		for _, p := range paths {
			was := renames.At(p, day)
			if (was != p) == renamed {
				assignedThen[was] = a[p]
				todayPath[was] = p
			}
		}
	}
	return assignedThen, todayPath
}
