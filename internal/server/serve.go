package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// ListenAndServe serves the app on cfg.Port until ctx is cancelled, then shuts
// down gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	hs := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(s.cfg.Port)),
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- hs.ListenAndServe() }()
	s.log.Info("listening", "addr", hs.Addr, "mount_root", s.cfg.MountRoot,
		"status_dir", s.cfg.StatusDir, "data_dir", s.cfg.DataDir, "scan_interval", s.cfg.ScanInterval.String())

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	s.log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := hs.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// probeTimeout is how long Probe waits for the answer, within the 5 seconds
// the Dockerfile's HEALTHCHECK gives it.
const probeTimeout = 3 * time.Second

// Probe checks GET /api/health on the local server listening on port. It is
// what the Docker HEALTHCHECK runs (`hoardboard healthcheck`), so the reason an
// unhealthy server gives goes into its error, and from there into the
// container's health log.
func Probe(ctx context.Context, port int) error {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/api/health", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var report healthReport
		if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&report); err != nil || report.Error == "" {
			return fmt.Errorf("health: %s", resp.Status)
		}
		return fmt.Errorf("health: %s: %s", resp.Status, report.Error)
	}
	return nil
}
