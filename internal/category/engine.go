// Package category is the category engine and its store: the Categories the
// owner defines, the Assignments that put Folders into them, and what each
// category takes in a Scan's tree or on a past day.
//
// The engine never walks the Disks: a Folder's SingleBytes covers its files
// with one name, and the hardlinked files cover the rest, each with the
// Folder of every name (Bytes). A finished tree.Tree has both (Measure), and
// so has History for every past day (docs/adr/0005-category-history.md).
package category

import (
	"cmp"
	"math/bits"
	"slices"
	"strconv"
	"strings"

	"github.com/Marcowich0/hoardboard/internal/tree"
)

// ID names a Category: the categories table's id. Uncategorized is 0.
type ID int64

// Uncategorized is the automatic category of every byte no Assignment
// covers. It is never stored.
const Uncategorized ID = 0

// Assignments maps a Folder's path to the category it is assigned to. A
// Folder has at most one Assignment; everything below it is in that
// category too, unless a more specific Assignment says otherwise.
type Assignments map[string]ID

// Of is the effective category of the Folder (or file) at path: that of the
// most specific Assignment at or above it, which assigned is the path of.
// ok is false when no Assignment covers path: it is Uncategorized.
func (a Assignments) Of(path string) (id ID, assigned string, ok bool) {
	for p := path; p != ""; p = parent(p) {
		if id, ok := a[p]; ok {
			return id, p, true
		}
	}
	return Uncategorized, "", false
}

func parent(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

// Sizes is what each category takes in one tree, under one set of
// Assignments. Its zero value is an empty tree's.
//
// Inside, the tree's bytes are grouped by the set of categories they are
// in: a file with one name is in one category; a hardlinked file is in the
// category of each of its names. Every byte is in exactly one group, so
// the groups add up to the tree's Size, and any view of the categories
// (full Sizes, Shared bytes, a split of Used space by Rank) is a sum over
// them.
type Sizes struct {
	groups map[string]*group
}

// group is the bytes in exactly the categories ids (sorted, distinct).
type group struct {
	ids   []ID
	bytes int64
}

// Measure computes what each category takes in t (nil is an empty tree)
// under the Assignments a.
func Measure(t *tree.Tree, a Assignments) *Sizes {
	if t == nil {
		return MeasureBytes(Bytes{}, a)
	}
	paths := map[*tree.Folder]string{}
	path := func(f *tree.Folder) string {
		p, ok := paths[f]
		if !ok {
			p = f.Path()
			paths[f] = p
		}
		return p
	}
	links := make([]Link, len(t.Hardlinks))
	for i, l := range t.Hardlinks {
		links[i] = Link{Bytes: l.Bytes, Folders: make([]string, len(l.Folders))}
		for j, f := range l.Folders {
			links[i].Folders[j] = path(f)
		}
	}
	return MeasureBytes(Bytes{
		Single: func(p string) int64 {
			if f, ok := t.Lookup(p); ok {
				return f.SingleBytes
			}
			return 0
		},
		Links: links,
	}, a)
}

// Bytes is what the engine needs of a tree, on any day: the SingleBytes of
// a Folder (the root and the assigned Folders are all it asks for), and the
// hardlinked files with the Folder of each name. A Scan's tree has them
// (Measure), and so has History for a past day.
type Bytes struct {
	// Single is the SingleBytes of the Folder at path: its files with one
	// name, at any depth. 0 for a Folder that does not exist. Nil is an
	// empty tree.
	Single func(path string) int64
	// Links are the hardlinked files. Files whose names are in the same
	// Folders may come as one Link with their bytes added up.
	Links []Link
}

// Link is a hardlinked file (or several with the same Folders): its bytes,
// and the path of the Folder of each of its names.
type Link struct {
	Bytes   int64
	Folders []string
}

// MeasureBytes computes what each category takes in b under the
// Assignments a. A file with one name is in the category of its Folder's
// most specific Assignment; a hardlinked file is in the category of each of
// its names.
func MeasureBytes(b Bytes, a Assignments) *Sizes {
	s := &Sizes{groups: map[string]*group{}}
	single := b.Single
	if single == nil {
		single = func(string) int64 { return 0 }
	}
	// A Folder's SingleBytes adds up over its subfolders, so what an
	// Assignment's category gets of the files with one name is its Folder's
	// SingleBytes minus that of the Assignments nearest below it; the root
	// ("", Uncategorized) likewise.
	own := map[string]int64{"": single("")}
	for path := range a {
		if path != "" {
			own[path] += single(path)
		}
	}
	for path := range a {
		if path == "" {
			continue
		}
		_, above, _ := a.Of(parent(path))
		own[above] -= single(path)
	}
	for path, bytes := range own {
		s.add([]ID{a.byPath(path)}, bytes)
	}

	var ids []ID
	for _, l := range b.Links {
		ids = ids[:0]
		for _, f := range l.Folders {
			id, _, _ := a.Of(f)
			ids = append(ids, id)
		}
		slices.Sort(ids)
		s.add(slices.Compact(ids), l.Bytes)
	}
	return s
}

// byPath is the category of the Assignment at path, Uncategorized for the
// root.
func (a Assignments) byPath(path string) ID {
	if path == "" {
		return Uncategorized
	}
	return a[path]
}

func (s *Sizes) add(ids []ID, bytes int64) {
	if bytes == 0 {
		return
	}
	var key strings.Builder
	for _, id := range ids {
		key.WriteString(strconv.FormatInt(int64(id), 10))
		key.WriteByte(',')
	}
	g, ok := s.groups[key.String()]
	if !ok {
		g = &group{ids: slices.Clone(ids)}
		s.groups[key.String()] = g
	}
	g.bytes += bytes
}

// Size is the category's full Size: every byte with a name in one of its
// Folders, each physical file once, Shared bytes included.
func (s *Sizes) Size(id ID) int64 {
	var n int64
	for _, g := range s.groups {
		if slices.Contains(g.ids, id) {
			n += g.bytes
		}
	}
	return n
}

// Uncategorized is the bytes no Assignment covers: those with every name
// outside the assigned Folders. A hardlinked file with one name in a
// category is that category's, not Uncategorized.
func (s *Sizes) Uncategorized() int64 {
	var n int64
	for _, g := range s.groups {
		if len(g.ids) == 1 && g.ids[0] == Uncategorized {
			n += g.bytes
		}
	}
	return n
}

// Shared is the bytes a category shares with another one (or with
// Uncategorized): hardlinked files with names in both.
type Shared struct {
	With  ID
	Bytes int64
}

// SharedWith is what the category id shares with each other category,
// largest first. With Uncategorized means names in Folders no Assignment
// covers.
func (s *Sizes) SharedWith(id ID) []Shared {
	by := map[ID]int64{}
	for _, g := range s.groups {
		if len(g.ids) < 2 || !slices.Contains(g.ids, id) {
			continue
		}
		for _, other := range g.ids {
			if other != id {
				by[other] += g.bytes
			}
		}
	}
	out := make([]Shared, 0, len(by))
	for other, n := range by {
		out = append(out, Shared{With: other, Bytes: n})
	}
	slices.SortFunc(out, func(a, b Shared) int {
		return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), cmp.Compare(a.With, b.With))
	})
	return out
}

// Split is Used space in parts that never overlap: one per category, in
// Rank order, then Uncategorized and Unaccounted. The parts always add up to
// the Used space split.
type Split struct {
	Categories    []Part
	Uncategorized int64
	// Unaccounted is the Used space no Scan accounts for: Used space minus
	// every byte in the tree, floored at zero.
	Unaccounted int64
}

// Part is the bytes a category owns in a Split.
type Part struct {
	ID    ID
	Bytes int64
}

// Total is every byte in the tree, each physical file once.
func (s *Sizes) Total() int64 {
	var n int64
	for _, g := range s.groups {
		n += g.bytes
	}
	return n
}

// Split splits used, the filesystems' Used space, for the donut. ranked is
// every category id, highest Rank first. Bytes in several categories go to
// the highest-Ranked of them; a category outranks Uncategorized.
//
// When the tree holds more than used (files deleted since the Scan, or
// compressed data that the Scan counts at full size), the categories and
// Uncategorized shrink in proportion to add up to used, with Unaccounted 0
// (docs/adr/0007-scanned-bytes-shrink-to-used-space.md).
func (s *Sizes) Split(ranked []ID, used int64) Split {
	pos := make(map[ID]int, len(ranked))
	for i, id := range ranked {
		pos[id] = i
	}
	// parts is each category's bytes in Rank order, then Uncategorized.
	parts := make([]int64, len(ranked)+1)
	var accounted int64
	for _, g := range s.groups {
		accounted += g.bytes
		owner := len(ranked)
		for _, id := range g.ids {
			if i, ok := pos[id]; ok && i < owner {
				owner = i
			}
		}
		parts[owner] += g.bytes
	}
	out := Split{Categories: make([]Part, len(ranked))}
	if accounted > used {
		shrink(parts, accounted, max(0, used))
	} else {
		out.Unaccounted = used - accounted
	}
	for i, id := range ranked {
		out.Categories[i] = Part{ID: id, Bytes: parts[i]}
	}
	out.Uncategorized = parts[len(ranked)]
	return out
}

// shrink scales parts, which add up to total, to add up to to (to < total):
// each part times to/total, rounded down, and the bytes rounding leaves over
// go to the largest part (the first of them on a tie).
func shrink(parts []int64, total, to int64) {
	largest := 0
	for i, p := range parts {
		if p > parts[largest] {
			largest = i
		}
	}
	left := to
	for i, p := range parts {
		// p·to can overflow int64; its high word is below total, as p and
		// to are at most total.
		hi, lo := bits.Mul64(uint64(p), uint64(to))
		q, _ := bits.Div64(hi, lo, uint64(total))
		parts[i] = int64(q)
		left -= parts[i]
	}
	parts[largest] += left
}
