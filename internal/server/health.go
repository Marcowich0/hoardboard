package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

func init() {
	addRoutes(func(s *Server) { s.mux.HandleFunc("GET /api/health", s.handleHealth) })
}

// healthTimeout is how long GET /api/health waits for what it checks, less
// than probeTimeout, so that Probe gets the reason.
const healthTimeout = 2 * time.Second

// healthReport is the answer of GET /api/health. Error says what failed, and
// Revision is the commit the server was built from.
type healthReport struct {
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	Revision string `json:"revision,omitempty"`
}

// handleHealth answers 200 if the server can do its work, and 503 with the
// reason if not.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.healthy(r.Context()); err != nil {
		s.log.Warn("unhealthy", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, healthReport{Status: "unhealthy", Error: err.Error(), Revision: s.cfg.Revision})
		return
	}
	writeJSON(w, http.StatusOK, healthReport{Status: "ok", Revision: s.cfg.Revision})
}

// healthy checks that the database answers and that the mount root and the
// status folder are there: without them no Scan can run, and a missing bind
// mount is the likeliest deployment mistake.
func (s *Server) healthy(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	for _, dir := range []struct{ what, path string }{
		{"mount root", s.cfg.MountRoot},
		{"status folder", s.cfg.StatusDir},
	} {
		info, err := os.Stat(dir.path)
		if err != nil {
			return fmt.Errorf("%s: %w", dir.what, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s: %s is not a folder", dir.what, dir.path)
		}
	}
	return nil
}
