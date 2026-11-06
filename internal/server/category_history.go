package server

import (
	"context"
	"net/http"
	"slices"
	"strconv"

	"github.com/Marcowich0/hoardboard/internal/category"
	"github.com/Marcowich0/hoardboard/internal/history"
)

// categoryHistoryJSON is the answer of GET /api/history?category= (History
// in web/src/api/types.ts, with the category's id).
type categoryHistoryJSON struct {
	CategoryID category.ID        `json:"categoryId"`
	Points     []categoryPointJSON `json:"points"`
}

type categoryPointJSON struct {
	Day  string `json:"day"`
	Size int64  `json:"size"`
}

// handleCategoryHistory answers a category's History: its full Size on each
// day from the first Scan to today, under the current Assignments, each
// physical file once (docs/adr/0004-category-history.md).
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
	p, err := s.measurePast(r.Context())
	if err != nil {
		s.log.Error("category history", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := categoryHistoryJSON{CategoryID: id, Points: make([]categoryPointJSON, len(p.sizes))}
	for i, sz := range p.sizes {
		out.Points[i] = categoryPointJSON{Day: (p.from + history.Day(i)).String(), Size: sz.Size(id)}
	}
	writeJSON(w, http.StatusOK, out)
}

// past is what each category took on every day of History, under the
// current Assignments.
type past struct {
	// from is the first day, the first Scan's; sizes[i] is day from+i, the
	// last one today.
	from  history.Day
	sizes []*category.Sizes
	// ranked is every category id, highest Rank first.
	ranked []category.ID
}

// measurePast rebuilds each day's bytes from History (every Folder's
// SingleBytes, and the hardlinked files with the Folders of their names),
// and measures them under the current Assignments, the way the engine
// measures the latest Scan. Before the first Scan it holds no day.
func (s *Server) measurePast(ctx context.Context) (past, error) {
	list, err := s.cats.List(ctx)
	if err != nil {
		return past{}, err
	}
	assigned, err := s.cats.Assignments(ctx)
	if err != nil {
		return past{}, err
	}
	var p past
	for _, c := range list {
		p.ranked = append(p.ranked, c.ID)
	}
	start, ok, err := s.history.Start(ctx)
	if err != nil || !ok {
		return p, err
	}
	p.from = start
	paths := []string{""}
	for path := range assigned {
		paths = append(paths, path)
	}
	folders, err := s.history.Read(ctx, paths, start)
	if err != nil {
		return past{}, err
	}
	links, err := s.history.ReadLinks(ctx, start)
	if err != nil {
		return past{}, err
	}
	var day history.Day
	b := category.Bytes{Single: func(path string) int64 { return folders[path].At(day).SingleBytes }}
	for day = start; day <= max(start, s.today()); day++ {
		b.Links = b.Links[:0]
		for _, l := range links {
			if n := l.Series.At(day); n != 0 {
				b.Links = append(b.Links, category.Link{Bytes: n, Folders: l.Folders})
			}
		}
		p.sizes = append(p.sizes, category.MeasureBytes(b, assigned))
	}
	return p, nil
}
