package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Marcowich0/hoardboard/internal/scan"
)

func init() {
	addRoutes(func(s *Server) {
		s.mux.HandleFunc("GET /api/scan", s.handleScanStatus)
		s.mux.HandleFunc("POST /api/scan", s.handleScanNow)
	})
}

// scanStatusJSON is the answer of GET /api/scan (ScanStatus in
// web/src/api/types.ts), what the header shows.
type scanStatusJSON struct {
	Running *scanRunningJSON `json:"running"`
	Last    *scanLastJSON    `json:"last"`
	// Skipped and Next come with the scheduler; always null for now.
	Skipped *scanSkippedJSON `json:"skipped"`
	Next    *string          `json:"next"`
}

type scanRunningJSON struct {
	ID        int64  `json:"id"`
	StartedAt string `json:"startedAt"`
}

type scanLastJSON struct {
	ID         int64  `json:"id"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	DurationMs int64  `json:"durationMs"`
}

type scanSkippedJSON struct {
	At     string `json:"at"`
	Reason string `json:"reason"`
}

// scanRequestJSON is the answer of POST /api/scan (ScanRequestResult).
type scanRequestJSON struct {
	Status scanStatusJSON `json:"status"`
	// Started is true when a Scan runs (or ran) for the request, including
	// one it joined.
	Started bool `json:"started"`
}

func (s *Server) handleScanStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, statusJSON(s.scans.Status()))
}

// handleScanNow starts a Scan, or joins the one that runs, and answers at
// once with the status. With ?wait=true it answers once that Scan has ended:
// 200 if it finished, 500 with the reason if it failed.
func (s *Server) handleScanNow(w http.ResponseWriter, r *http.Request) {
	wait := false
	if v := r.URL.Query().Get("wait"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "wait: want true or false")
			return
		}
		wait = b
	}
	run, err := s.scans.Start()
	if err != nil {
		s.log.Error("scan", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if wait {
		select {
		case <-run.Done():
		case <-r.Context().Done():
			return
		}
		if err := run.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "scan failed: "+err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, scanRequestJSON{Status: statusJSON(s.scans.Status()), Started: true})
}

func statusJSON(st scan.Status) scanStatusJSON {
	var out scanStatusJSON
	if r := st.Running; r != nil {
		out.Running = &scanRunningJSON{ID: r.ID, StartedAt: timeJSON(r.Started)}
	}
	if d := st.Last; d != nil {
		out.Last = &scanLastJSON{
			ID:         d.ID,
			StartedAt:  timeJSON(d.Started),
			FinishedAt: timeJSON(d.Finished),
			DurationMs: d.Finished.Sub(d.Started).Milliseconds(),
		}
	}
	return out
}

// timeJSON is t in RFC 3339, UTC, to the millisecond.
func timeJSON(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z07:00") }
