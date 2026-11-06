package server

import "net/http"

// routeGroups are the features' routes. A feature keeps its routes and
// handlers in its own file and adds its group there, from init:
//
//	func init() { addRoutes((*Server).overviewRoutes) }
//
// so a new feature needs no change here. The order does not matter:
// http.ServeMux picks the most specific pattern, and panics at startup if two
// conflict.
var routeGroups []func(*Server)

func addRoutes(group func(*Server)) { routeGroups = append(routeGroups, group) }

func (s *Server) routes() {
	for _, group := range routeGroups {
		group(s)
	}
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	s.mux.Handle("/", webApp())
}
