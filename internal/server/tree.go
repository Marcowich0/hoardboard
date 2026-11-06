package server

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/Marcowich0/hoardboard/internal/category"
	"github.com/Marcowich0/hoardboard/internal/history"
	"github.com/Marcowich0/hoardboard/internal/tree"
)

func init() {
	addRoutes(func(s *Server) { s.mux.HandleFunc("GET /api/tree", s.handleTree) })
}

// treeJSON is the answer of GET /api/tree (Tree in web/src/api/types.ts).
type treeJSON struct {
	Path     string     `json:"path"`
	Period   string     `json:"period"`
	Folder   folderJSON `json:"folder"`
	Children []any      `json:"children"` // folderEntryJSON, then fileEntryJSON
}

// folderJSON is FolderInfo.
type folderJSON struct {
	Path        string          `json:"path"`
	Name        string          `json:"name"`
	Size        int64           `json:"size"`
	Change      *int64          `json:"change"`
	Items       int64           `json:"items"`
	SharedBytes int64           `json:"sharedBytes"`
	Disks       []diskBytesJSON `json:"disks"`
	// Category is null when the Folder is Uncategorized.
	Category *folderCategoryJSON `json:"category"`
}

// folderCategoryJSON is a Folder's effective category and the path of the
// Assignment that decides it: its own or an ancestor's.
type folderCategoryJSON struct {
	CategoryID   category.ID `json:"categoryId"`
	AssignedPath string      `json:"assignedPath"`
}

type diskBytesJSON struct {
	Disk  string `json:"disk"`
	Bytes int64  `json:"bytes"`
}

type folderEntryJSON struct {
	Kind        string          `json:"kind"` // "folder"
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	Size        int64           `json:"size"`
	Change      *int64          `json:"change"`
	Items       int64           `json:"items"`
	SharedBytes int64           `json:"sharedBytes"`
	Disks       []diskBytesJSON `json:"disks"`
	// CategoryID is its effective category, null for Uncategorized.
	CategoryID *category.ID `json:"categoryId"`
}

type fileEntryJSON struct {
	Kind  string `json:"kind"` // "file"
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Disk  string `json:"disk"`
	Links uint32 `json:"links"`
	// CategoryID is the effective category of the Folder holding this
	// name, null for Uncategorized.
	CategoryID *category.ID `json:"categoryId"`
}

// periodDays is how many days back each period of GET /api/tree's change
// reaches.
var periodDays = map[string]int{"7d": 7, "30d": 30, "1y": 365}

const defaultPeriod = "30d"

// handleTree answers a Folder of the latest Scan's tree: its figures, its
// subfolders and its files. Before the first Scan the root is empty.
func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path := q.Get("path")
	if !validPath(path) {
		writeError(w, http.StatusBadRequest, `path: want a Share path such as "data/media", without leading or trailing "/"`)
		return
	}
	period := q.Get("period")
	if period == "" {
		period = defaultPeriod
	}
	days, ok := periodDays[period]
	if !ok {
		writeError(w, http.StatusBadRequest, "period: want 7d, 30d or 1y")
		return
	}

	t := s.scans.Tree()
	if t == nil {
		t = tree.NewBuilder().Finish()
	}
	f, ok := t.Lookup(path)
	if !ok {
		writeError(w, http.StatusNotFound, "no folder "+path)
		return
	}
	change, err := s.changes(r.Context(), f, path, days)
	if err != nil {
		s.log.Error("tree", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	assigned, err := s.cats.Assignments(r.Context())
	if err != nil {
		s.categoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, treeJSON{
		Path:     path,
		Period:   period,
		Folder:   folderInfoJSON(t, f, path, change, assigned),
		Children: childrenJSON(t, f, path, change, assigned),
	})
}

// changeFunc is a Folder's change over the period, by path; nil when History
// doesn't reach back that far.
type changeFunc func(path string, size int64) *int64

// changes reads, from History, how f and its subfolders changed over the
// last days days: their Size from the latest Scan minus their Size then (zero
// for a Folder that didn't exist then).
func (s *Server) changes(ctx context.Context, f *tree.Folder, path string, days int) (changeFunc, error) {
	start, ok, err := s.history.Start(ctx)
	if err != nil {
		return nil, err
	}
	then := s.today() - history.Day(days)
	if !ok || then < start {
		return func(string, int64) *int64 { return nil }, nil
	}
	paths := make([]string, 0, len(f.Folders)+1)
	paths = append(paths, path)
	for _, c := range f.Folders {
		paths = append(paths, joinPath(path, c.Name))
	}
	series, err := s.history.Read(ctx, paths, then)
	if err != nil {
		return nil, err
	}
	return func(path string, size int64) *int64 {
		d := size - series[path].At(then).Size
		return &d
	}, nil
}

// validPath accepts "" and slash-separated names, none empty, "." or "..".
func validPath(p string) bool {
	if p == "" {
		return true
	}
	for name := range strings.SplitSeq(p, "/") {
		if name == "" || name == "." || name == ".." || strings.ContainsRune(name, 0) {
			return false
		}
	}
	return true
}

func folderInfoJSON(t *tree.Tree, f *tree.Folder, path string, change changeFunc, assigned category.Assignments) folderJSON {
	var c *folderCategoryJSON
	if id, at, ok := assigned.Of(path); ok {
		c = &folderCategoryJSON{CategoryID: id, AssignedPath: at}
	}
	return folderJSON{
		Path:        path,
		Name:        f.Name,
		Size:        f.Size,
		Change:      change(path, f.Size),
		Items:       f.Items,
		SharedBytes: f.SharedBytes,
		Disks:       disksJSON(t, f),
		Category:    c,
	}
}

// childrenJSON lists f's subfolders, then its files, each largest first.
func childrenJSON(t *tree.Tree, f *tree.Folder, path string, change changeFunc, assigned category.Assignments) []any {
	join := func(name string) string { return joinPath(path, name) }
	folders := slices.Clone(f.Folders)
	slices.SortStableFunc(folders, func(a, b *tree.Folder) int { return cmp.Compare(b.Size, a.Size) })
	files := slices.Clone(f.Files)
	slices.SortStableFunc(files, func(a, b tree.File) int { return cmp.Compare(b.Bytes, a.Bytes) })

	out := make([]any, 0, len(folders)+len(files))
	for _, c := range folders {
		out = append(out, folderEntryJSON{
			Kind:        "folder",
			Name:        c.Name,
			Path:        join(c.Name),
			Size:        c.Size,
			Change:      change(join(c.Name), c.Size),
			Items:       c.Items,
			SharedBytes: c.SharedBytes,
			Disks:       disksJSON(t, c),
			CategoryID:  categoryOf(assigned, join(c.Name)),
		})
	}
	for _, file := range files {
		out = append(out, fileEntryJSON{
			Kind:  "file",
			Name:  file.Name,
			Path:  join(file.Name),
			Size:  file.Bytes,
			Disk:  t.Disks[file.Disk],
			Links: file.Links,
			// A file is in its Folder's category.
			CategoryID: categoryOf(assigned, path),
		})
	}
	return out
}

// categoryOf is the effective category of the Folder at path, nil when it
// is Uncategorized.
func categoryOf(assigned category.Assignments, path string) *category.ID {
	if id, _, ok := assigned.Of(path); ok {
		return &id
	}
	return nil
}

// disksJSON is where f's bytes are, largest first; Disks holding none of
// its bytes are left out.
func disksJSON(t *tree.Tree, f *tree.Folder) []diskBytesJSON {
	out := []diskBytesJSON{}
	for id, n := range f.DiskBytes {
		if n > 0 {
			out = append(out, diskBytesJSON{Disk: t.Disks[id], Bytes: n})
		}
	}
	slices.SortStableFunc(out, func(a, b diskBytesJSON) int { return cmp.Compare(b.Bytes, a.Bytes) })
	return out
}
