package server

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/Marcowich0/hoardboard/internal/category"
	"github.com/Marcowich0/hoardboard/internal/tree"
)

func init() {
	addRoutes(func(s *Server) { s.mux.HandleFunc("GET /api/largest", s.handleLargest) })
}

// largestJSON is the answer of GET /api/largest (Largest in web/src/api/types.ts).
type largestJSON struct {
	Path  string          `json:"path"`
	Files []fileEntryJSON `json:"files"`
}

// handleLargest answers the largest files anywhere below a Folder of the
// latest Scan's tree, largest first. Files come from the latest Scan only.
func (s *Server) handleLargest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !validPath(path) {
		writeError(w, http.StatusBadRequest, `path: want a Share path such as "data/media", without leading or trailing "/"`)
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
	assigned, err := s.cats.Assignments(r.Context())
	if err != nil {
		s.categoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, largestJSON{Path: path, Files: largestFiles(t, f, path, assigned)})
}

// largestCount is how many files GET /api/largest answers.
const largestCount = 20

// largestFiles lists the largestCount largest physical files below f,
// largest first, then by path. A hardlinked file is listed once, by its
// first path in name order, in the category of that name's Folder.
func largestFiles(t *tree.Tree, f *tree.Folder, path string, assigned category.Assignments) []fileEntryJSON {
	type identity struct {
		disk     tree.DiskID
		dev, ino uint64
	}
	idOf := func(file *tree.File) identity { return identity{file.Disk, file.Dev, file.Ino} }
	walk := func(visit func(dir string, file *tree.File)) {
		var rec func(f *tree.Folder, dir string)
		rec = func(f *tree.Folder, dir string) {
			for i := range f.Files {
				visit(dir, &f.Files[i])
			}
			for _, c := range f.Folders {
				rec(c, joinPath(dir, c.Name))
			}
		}
		rec(f, path)
	}

	// The name each file with several names is listed by.
	firstName := map[identity]string{}
	walk(func(dir string, file *tree.File) {
		if file.Links > 1 {
			p := joinPath(dir, file.Name)
			if q, ok := firstName[idOf(file)]; !ok || p < q {
				firstName[idOf(file)] = p
			}
		}
	})

	// top holds the largest so far, in answer order; a file smaller than its
	// last needs no path.
	top := make([]fileEntryJSON, 0, largestCount+1)
	before := func(a, b fileEntryJSON) int {
		return cmp.Or(cmp.Compare(b.Size, a.Size), cmp.Compare(a.Path, b.Path))
	}
	walk(func(dir string, file *tree.File) {
		if len(top) == largestCount && file.Bytes < top[len(top)-1].Size {
			return
		}
		p := joinPath(dir, file.Name)
		if file.Links > 1 && firstName[idOf(file)] != p {
			return
		}
		e := fileEntryJSON{
			Kind:  "file",
			Name:  file.Name,
			Path:  p,
			Size:  file.Bytes,
			Disk:  t.Disks[file.Disk],
			Links: file.Links,
		}
		i, _ := slices.BinarySearchFunc(top, e, before)
		if i < largestCount {
			e.CategoryID = categoryOf(assigned, dir)
			top = slices.Insert(top, i, e)[:min(len(top)+1, largestCount)]
		}
	})
	return top
}

// joinPath is the Share path of name inside the Folder at path.
func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "/" + name
}
