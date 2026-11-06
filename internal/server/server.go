// Package server is hoardboard's HTTP server: the JSON API under /api/ and
// the embedded single-page app everywhere else.
package server

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/Marcowich0/hoardboard/internal/clock"
	"github.com/Marcowich0/hoardboard/internal/config"
	"github.com/Marcowich0/hoardboard/internal/db"
)

// Server holds what the handlers share.
type Server struct {
	cfg   config.Config
	log   *slog.Logger
	clock clock.Clock
	db    *sql.DB
	mux   *http.ServeMux
}

// New opens the database in cfg.DataDir and sets up the routes. Close it
// when done.
func New(cfg config.Config, log *slog.Logger) (*Server, error) {
	d, err := db.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	c := cfg.Clock
	if c == nil {
		c = clock.System()
	}
	s := &Server{cfg: cfg, log: log, clock: c, db: d, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

// Handler is the server's HTTP handler: the routes, behind cross-origin
// protection for the POST endpoints (the app has no login) and the security
// headers.
func (s *Server) Handler() http.Handler {
	return securityHeaders(http.NewCrossOriginProtection().Handler(s.mux))
}

// Close closes the database.
func (s *Server) Close() error { return s.db.Close() }
