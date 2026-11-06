package server

import (
	"net/http"

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
	v := treeView{t: t, assigned: assigned}
	files := []fileEntryJSON{}
	for _, n := range f.Largest(largestCount) {
		files = append(files, v.file(n.File, n.Dir))
	}
	writeJSON(w, http.StatusOK, largestJSON{Path: path, Files: files})
}

// largestCount is how many files GET /api/largest answers.
const largestCount = 20
