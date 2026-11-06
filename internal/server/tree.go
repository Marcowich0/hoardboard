package server

import (
	"cmp"
	"net/http"
	"slices"
	"strings"

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

// folderJSON is FolderInfo. Change and Category come with History and the
// category engine; until then they are null.
type folderJSON struct {
	Path        string          `json:"path"`
	Name        string          `json:"name"`
	Size        int64           `json:"size"`
	Change      *int64          `json:"change"`
	Items       int64           `json:"items"`
	SharedBytes int64           `json:"sharedBytes"`
	Disks       []diskBytesJSON `json:"disks"`
	Category    *struct{}       `json:"category"`
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
	CategoryID  *int64          `json:"categoryId"`
}

type fileEntryJSON struct {
	Kind       string `json:"kind"` // "file"
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Disk       string `json:"disk"`
	Links      uint32 `json:"links"`
	CategoryID *int64 `json:"categoryId"`
}

var periods = []string{"7d", "30d", "1y"}

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
	if !slices.Contains(periods, period) {
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
	writeJSON(w, http.StatusOK, treeJSON{
		Path:     path,
		Period:   period,
		Folder:   folderInfoJSON(t, f, path),
		Children: childrenJSON(t, f, path),
	})
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

func folderInfoJSON(t *tree.Tree, f *tree.Folder, path string) folderJSON {
	return folderJSON{
		Path:        path,
		Name:        f.Name,
		Size:        f.Size,
		Items:       f.Items,
		SharedBytes: f.SharedBytes,
		Disks:       disksJSON(t, f),
	}
}

// childrenJSON lists f's subfolders, then its files, each largest first.
func childrenJSON(t *tree.Tree, f *tree.Folder, path string) []any {
	join := func(name string) string {
		if path == "" {
			return name
		}
		return path + "/" + name
	}
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
			Items:       c.Items,
			SharedBytes: c.SharedBytes,
			Disks:       disksJSON(t, c),
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
		})
	}
	return out
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
