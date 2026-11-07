package server

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/Marcowich0/hoardboard/internal/category"
	"github.com/Marcowich0/hoardboard/internal/history"
)

// categoryHistoryJSON is the answer of GET /api/history?category= (History
// in web/src/api/types.ts, with the category's id).
type categoryHistoryJSON struct {
	CategoryID category.ID         `json:"categoryId"`
	Points     []categoryPointJSON `json:"points"`
}

type categoryPointJSON struct {
	Day  string `json:"day"`
	Size int64  `json:"size"`
}

// handleCategoryHistory answers a category's History: its full Size on each
// day from the first Scan to today, under the current Assignments, each
// physical file once (docs/adr/0005-category-history.md). A group's covers
// the categories in it too.
func (s *Server) handleCategoryHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Has("path") {
		writeError(w, http.StatusBadRequest, "ask for a path or a category, not both")
		return
	}
	n, err := strconv.ParseInt(q.Get("category"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "category: want a category id")
		return
	}
	id := category.ID(n)
	list, err := s.cats.List(r.Context())
	if err != nil {
		s.categoryError(w, err)
		return
	}
	if !slices.ContainsFunc(list, func(c category.Category) bool { return c.ID == id }) {
		writeError(w, http.StatusNotFound, "no category "+q.Get("category"))
		return
	}
	assigned, err := s.cats.Assignments(r.Context())
	if err != nil {
		s.categoryError(w, err)
		return
	}
	p, err := category.MeasurePast(r.Context(), s.history, assigned, s.today())
	if err != nil {
		s.log.Error("category history", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	members := category.Members(list, id)
	out := categoryHistoryJSON{CategoryID: id, Points: make([]categoryPointJSON, len(p.Sizes))}
	for i, sz := range p.Sizes {
		out.Points[i] = categoryPointJSON{Day: (p.From + history.Day(i)).String(), Size: sz.Size(members...)}
	}
	writeJSON(w, http.StatusOK, out)
}
