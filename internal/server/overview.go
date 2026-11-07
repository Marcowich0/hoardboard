package server

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/Marcowich0/hoardboard/internal/category"
	"github.com/Marcowich0/hoardboard/internal/inventory"
	"github.com/Marcowich0/hoardboard/internal/tree"
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
	// Split is the Disk's Used space by category, like the donut, but
	// from the bytes the latest Scan found on this Disk alone.
	Split donutJSON `json:"split"`
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
	rk, err := s.readRanking(r.Context())
	if err == nil {
		resp.Donut = s.donut(rk, resp.Totals.Used)
		s.splitDisks(rk, resp.Disks)
		resp.Forecast, resp.ForecastDays, err = s.forecast(r.Context(), s.today(), resp.Totals.Size)
	}
	if err == nil {
		resp.Trend, err = s.trend(r.Context(), rk, resp.Totals.Used)
	}
	if err != nil {
		s.log.Error("overview", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ranking is what splits Used space by category: every category id,
// highest Rank first, and the Assignments.
type ranking struct {
	ranked   []category.ID
	assigned category.Assignments
}

func (s *Server) readRanking(ctx context.Context) (ranking, error) {
	list, err := s.cats.List(ctx)
	if err != nil {
		return ranking{}, err
	}
	assigned, err := s.cats.Assignments(ctx)
	if err != nil {
		return ranking{}, err
	}
	r := ranking{ranked: make([]category.ID, len(list)), assigned: assigned}
	for i, c := range list {
		r.ranked[i] = c.ID
	}
	return r, nil
}

// donut splits used, the live Used space, by category in the latest Scan's
// tree: Shared bytes go to the highest-Ranked category they are in, and
// what the Scan didn't see is Unaccounted.
func (s *Server) donut(r ranking, used int64) donutJSON {
	return splitJSON(category.Measure(s.scans.Tree(), r.assigned).Split(r.ranked, used))
}

// splitDisks splits each Disk's live Used space by category in the latest
// Scan's tree, from the bytes found on that Disk: hardlinks never cross
// Disks, so each file is on exactly one. A Disk the Scan didn't walk is all
// Unaccounted.
func (s *Server) splitDisks(r ranking, disks []diskJSON) {
	t := s.scans.Tree()
	for i := range disks {
		d := &disks[i]
		sizes := category.Measure(nil, r.assigned)
		if t != nil {
			if id := slices.Index(t.Disks, d.Name); id >= 0 {
				sizes = category.MeasureDisk(t, r.assigned, tree.DiskID(id))
			}
		}
		d.Split = splitJSON(sizes.Split(r.ranked, d.Used))
	}
}

func splitJSON(split category.Split) donutJSON {
	out := donutJSON{
		Categories:    make([]donutPartJSON, len(split.Categories)),
		Uncategorized: split.Uncategorized,
		Unaccounted:   split.Unaccounted,
	}
	for i, p := range split.Categories {
		out.Categories[i] = donutPartJSON{CategoryID: p.ID, Bytes: p.Bytes}
	}
	return out
}
