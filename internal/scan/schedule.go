package scan

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Marcowich0/hoardboard/internal/clock"
)

// schedule is the Scanner's timer for scheduled Scans.
type schedule struct {
	every time.Duration
	next  time.Time
	timer clock.Timer
}

// Schedule starts a Scan every interval, on the Scanner's clock, until
// Close. The first is due one interval after the latest Scan started
// (finished, skipped or not), and at once if that time has passed or there
// was never a Scan, so a restart doesn't wait an hour to bring stale data up
// to date. A scheduled Scan goes through Start like any other: it joins a
// running Scan and is skipped during array work. Call Schedule at most once.
func (s *Scanner) Schedule(ctx context.Context, every time.Duration) error {
	var latest sql.NullInt64
	if err := s.cfg.DB.QueryRowContext(ctx, `SELECT MAX(started) FROM scans`).Scan(&latest); err != nil {
		return fmt.Errorf("scan schedule: %w", err)
	}
	now := s.cfg.Clock.Now()
	due := now
	if latest.Valid {
		due = time.UnixMilli(latest.Int64).Add(every)
	}
	if !due.After(now) {
		s.mu.Lock()
		s.sched = &schedule{every: every}
		s.mu.Unlock()
		s.tick()
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sched = &schedule{every: every, next: due, timer: s.cfg.Clock.AfterFunc(due.Sub(now), s.tick)}
	return nil
}

// tick sets the timer for the next scheduled Scan and starts this one.
func (s *Scanner) tick() {
	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	sc := s.sched
	sc.next = s.cfg.Clock.Now().Add(sc.every)
	sc.timer = s.cfg.Clock.AfterFunc(sc.every, s.tick)
	s.mu.Unlock()
	if _, _, err := s.Start(); err != nil {
		s.cfg.Log.Error("scheduled scan", "err", err)
	}
}
