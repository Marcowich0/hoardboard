package server

import (
	"net/http"

	"github.com/Marcowich0/hoardboard/internal/history"
)

func init() {
	addRoutes(func(s *Server) { s.mux.HandleFunc("GET /api/history", s.handleHistory) })
}

// folderHistoryJSON is the answer of GET /api/history?path= (FolderHistory in
// web/src/api/types.ts).
type folderHistoryJSON struct {
	Path   string             `json:"path"`
	Points []historyPointJSON `json:"points"`
}

type historyPointJSON struct {
	Day         string `json:"day"`
	Size        int64  `json:"size"`
	SharedBytes int64  `json:"sharedBytes"`
}

// today is the server's day: the clock's now in the server's time zone (TZ),
// the same day a Scan finishing now records its History under.
func (s *Server) today() history.Day { return history.DayOf(s.clock.Now()) }

// handleHistory answers a Folder's History: one point per day, from the
// first Scan that saw it to today, the last stored value carried forward. A
// deleted Folder's History is still there (zero since the Scan that no
// longer found it).
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("category") {
		s.handleCategoryHistory(w, r)
		return
	}
	path := r.URL.Query().Get("path")
	if !validPath(path) {
		writeError(w, http.StatusBadRequest, `path: want a Share path such as "data/media", without leading or trailing "/"`)
		return
	}
	got, err := s.history.Read(r.Context(), []string{path}, 0)
	if err != nil {
		s.log.Error("history", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	series, known := got[path]
	if !known && path != "" {
		// A Folder of the latest tree has History; before the first Scan
		// only the root is known (empty).
		found := false
		if t := s.scans.Tree(); t != nil {
			_, found = t.Lookup(path)
		}
		if !found {
			writeError(w, http.StatusNotFound, "no folder "+path)
			return
		}
	}
	out := folderHistoryJSON{Path: path, Points: []historyPointJSON{}}
	if first, ok := series.First(); ok {
		last, _ := series.Last()
		for _, p := range series.Daily(first, max(last, s.today())) {
			out.Points = append(out.Points, historyPointJSON{
				Day: p.Day.String(), Size: p.Value.Size, SharedBytes: p.Value.SharedBytes,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}
