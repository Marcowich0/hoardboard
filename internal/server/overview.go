package server

import (
	"net/http"
	"time"

	"github.com/Marcowich0/hoardboard/internal/inventory"
)

func init() {
	addRoutes(func(s *Server) { s.mux.HandleFunc("GET /api/overview", s.handleOverview) })
}

// overviewResponse is the answer of GET /api/overview (Overview in
// web/src/api/types.ts). For now it holds what comes live from the disks;
// the donut, trend and forecast come with the category engine.
type overviewResponse struct {
	// Today is the server's day, "YYYY-MM-DD".
	Today string `json:"today"`
	// Totals are the data disks' and the pools' figures added up; parity is
	// never in them.
	Totals spaceJSON  `json:"totals"`
	Disks  []diskJSON `json:"disks"`
	// Parity is information only: size and status, no Used or Free space.
	Parity []parityJSON `json:"parity"`
}

type spaceJSON struct {
	Size int64 `json:"size"`
	Used int64 `json:"used"`
	Free int64 `json:"free"`
}

type diskJSON struct {
	Name       string         `json:"name"`
	Kind       inventory.Kind `json:"kind"`
	Filesystem string         `json:"filesystem"`
	spaceJSON
}

type parityJSON struct {
	Name   string                 `json:"name"`
	Size   int64                  `json:"size"`
	Status inventory.ParityStatus `json:"status"`
	// LastCheck is the server's day the last check finished, null if unknown.
	LastCheck *string `json:"lastCheck"`
}

// handleOverview reads the Disks and their Used and Free space afresh on
// every request: they are live, not from a Scan.
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	arr, err := s.inv.Read()
	if err != nil {
		s.log.Error("overview", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := s.clock.Now()
	resp := overviewResponse{
		Today:  now.Format(time.DateOnly),
		Disks:  []diskJSON{},
		Parity: []parityJSON{},
	}
	for _, p := range arr.Parity {
		pj := parityJSON{Name: p.Name, Size: p.Size, Status: p.Status}
		if !p.LastCheck.IsZero() {
			day := p.LastCheck.In(now.Location()).Format(time.DateOnly)
			pj.LastCheck = &day
		}
		resp.Parity = append(resp.Parity, pj)
	}
	for _, d := range arr.Disks {
		sp, err := d.Space()
		if err != nil {
			// Unraid lists it, but its folder isn't there (a bind mount
			// missing from the container, say): leave it out rather than
			// lose the whole overview.
			s.log.Warn("overview: disk left out", "disk", d.Name, "err", err)
			continue
		}
		js := spaceJSON{Size: sp.Size, Used: sp.Used, Free: sp.Free}
		resp.Disks = append(resp.Disks, diskJSON{Name: d.Name, Kind: d.Kind, Filesystem: d.Filesystem, spaceJSON: js})
		resp.Totals.Size += js.Size
		resp.Totals.Used += js.Used
		resp.Totals.Free += js.Free
	}
	writeJSON(w, http.StatusOK, resp)
}
