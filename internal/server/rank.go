package server

import (
	"net/http"

	"github.com/Marcowich0/hoardboard/internal/category"
)

func init() {
	addRoutes(func(s *Server) {
		s.mux.HandleFunc("PUT /api/categories/order", s.handleReorderCategories)
	})
}

// reorderJSON is the body of PUT /api/categories/order.
type reorderJSON struct {
	// IDs is every category id once, highest Rank first.
	IDs []category.ID `json:"ids"`
}

// handleReorderCategories sets the categories' Rank, which decides who owns
// Shared bytes in the Overview's donut, and answers 204.
func (s *Server) handleReorderCategories(w http.ResponseWriter, r *http.Request) {
	var in reorderJSON
	if !readJSONBody(w, r, &in) {
		return
	}
	if err := s.cats.Reorder(r.Context(), in.IDs); err != nil {
		s.categoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
