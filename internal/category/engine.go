// Package category is the category engine and its store: the Categories the
// owner defines, the Assignments that put Folders into them, and what each
// category takes in a Scan's tree.
//
// The engine works on the finished tree.Tree and never walks the Disks
// again: a Folder's SingleBytes covers its files with one name, and the
// Tree's Hardlinks cover the rest, each with the Folder of every name.
package category

import (
	"cmp"
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
	s := &Sizes{groups: map[string]*group{}}
	if t == nil {
		return s
	}
	// The category of each Folder holding a hardlinked file's name.
	of := map[*tree.Folder]ID{}
	var walk func(f *tree.Folder, path string, id ID)
	walk = func(f *tree.Folder, path string, id ID) {
		if c, ok := a[path]; ok && path != "" {
			id = c
		}
		of[f] = id
		own := f.SingleBytes
		for _, c := range f.Folders {
			own -= c.SingleBytes
			p := c.Name
			if path != "" {
				p = path + "/" + c.Name
			}
			walk(c, p, id)
		}
		s.add([]ID{id}, own)
	}
	walk(t.Root, "", Uncategorized)

	var ids []ID
	for _, l := range t.Hardlinks {
		ids = ids[:0]
		for _, f := range l.Folders {
			ids = append(ids, of[f])
		}
		slices.Sort(ids)
		s.add(slices.Compact(ids), l.Bytes)
	}
	return s
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
// Rank order, then Uncategorized and Unaccounted.
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

// Split splits used, the filesystems' Used space, for the donut. ranked is
// every category id, highest Rank first. Bytes in several categories go to
// the highest-Ranked of them; a category outranks Uncategorized. Unless the
// tree holds more than used (files deleted since the Scan), the parts add
// up to used exactly.
func (s *Sizes) Split(ranked []ID, used int64) Split {
	pos := make(map[ID]int, len(ranked))
	for i, id := range ranked {
		pos[id] = i
	}
	owned := make([]int64, len(ranked))
	out := Split{Categories: make([]Part, len(ranked))}
	var accounted int64
	for _, g := range s.groups {
		accounted += g.bytes
		owner := -1
		for _, id := range g.ids {
			if i, ok := pos[id]; ok && (owner < 0 || i < owner) {
				owner = i
			}
		}
		if owner < 0 {
			out.Uncategorized += g.bytes
		} else {
			owned[owner] += g.bytes
		}
	}
	for i, id := range ranked {
		out.Categories[i] = Part{ID: id, Bytes: owned[i]}
	}
	out.Unaccounted = max(0, used-accounted)
	return out
}
