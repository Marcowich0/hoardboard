package server

import (
	"context"
	"net/http"
	"time"

	"github.com/Marcowich0/hoardboard/internal/category"
	"github.com/Marcowich0/hoardboard/internal/inventory"
)

func init() {
	addRoutes(func(s *Server) { s.mux.HandleFunc("GET /api/overview", s.handleOverview) })
}

// overviewResponse is the answer of GET /api/overview (Overview in
// web/src/api/types.ts): what comes live from the disks, the donut, the
// trend and the forecast.
type overviewResponse struct {
	// Today is the server's day, "YYYY-MM-DD".
	Today string `json:"today"`
	// Totals are the data disks' and the pools' figures added up; parity is
	// never in them.
	Totals spaceJSON  `json:"totals"`
	Disks  []diskJSON `json:"disks"`
	// Parity is information only: size and status, no Used or Free space.
	Parity []parityJSON `json:"parity"`
	// Donut is Totals.Used split by category (UsedSplit).
	Donut donutJSON `json:"donut"`
	// Trend is the donut's split on every day of History.
	Trend trendJSON `json:"trend"`
	// Forecast is the growth rate and the day the array is full, fitted to
	// the last 90 days of total Used space; null with fewer than 7 daily
	// points. Its day is null when Used space isn't growing (see
	// forecast.go).
	Forecast *forecastJSON `json:"forecast"`
	// ForecastDays is the daily points of total Used space the fit had.
	ForecastDays int `json:"forecastDays"`
}

// donutJSON is UsedSplit: the parts add up to Totals.Used.
type donutJSON struct {
	// Categories is every category in Rank order.
	Categories    []donutPartJSON `json:"categories"`
	Uncategorized int64           `json:"uncategorized"`
	Unaccounted   int64           `json:"unaccounted"`
}

type donutPartJSON struct {
	CategoryID category.ID `json:"categoryId"`
	Bytes      int64       `json:"bytes"`
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
	if resp.Donut, err = s.donut(r.Context(), resp.Totals.Used); err == nil {
		resp.Forecast, resp.ForecastDays, err = s.forecast(r.Context(), s.today(), resp.Totals.Size)
	}
	if err == nil {
		resp.Trend, err = s.trend(r.Context(), resp.Totals.Used)
	}
	if err != nil {
		s.log.Error("overview", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// donut splits used, the live Used space, by category in the latest Scan's
// tree: Shared bytes go to the highest-Ranked category they are in, and
// what the Scan didn't see is Unaccounted.
func (s *Server) donut(ctx context.Context, used int64) (donutJSON, error) {
	list, err := s.cats.List(ctx)
	if err != nil {
		return donutJSON{}, err
	}
	assigned, err := s.cats.Assignments(ctx)
	if err != nil {
		return donutJSON{}, err
	}
	ranked := make([]category.ID, len(list))
	for i, c := range list {
		ranked[i] = c.ID
	}
	split := category.Measure(s.scans.Tree(), assigned).Split(ranked, used)
	out := donutJSON{
		Categories:    make([]donutPartJSON, len(split.Categories)),
		Uncategorized: split.Uncategorized,
		Unaccounted:   split.Unaccounted,
	}
	for i, p := range split.Categories {
		out.Categories[i] = donutPartJSON{CategoryID: p.ID, Bytes: p.Bytes}
	}
	return out, nil
}
