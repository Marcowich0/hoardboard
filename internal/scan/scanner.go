// Package scan runs Scans: it walks every Disk the inventory lists, reading
// metadata only at idle I/O priority, merges what it finds into one
// tree.Tree of Share paths, records the Scan in the scans table and saves the
// tree as the current snapshot.
//
// At most one Scan runs at a time: Start while one runs joins it. The latest
// tree stays in memory for the API (Tree), and is loaded from the snapshot
// at startup, so the Explorer works before the first Scan after a restart.
package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/Marcowich0/hoardboard/internal/clock"
	"github.com/Marcowich0/hoardboard/internal/inventory"
	"github.com/Marcowich0/hoardboard/internal/tree"
)

// AfterScan is work that belongs to a finished Scan, such as writing the
// day's History. It runs inside the transaction that records the Scan as
// finished and saves its snapshot, so either all of it is saved or none.
type AfterScan func(ctx context.Context, tx *sql.Tx, done Done, t *tree.Tree) error

// Config is what a Scanner needs.
type Config struct {
	DB        *sql.DB
	Inventory *inventory.Inventory
	Clock     clock.Clock
	Log       *slog.Logger
	// MountInfo is the mountinfo file listing mount points, which a Scan
	// never crosses. "" means DefaultMountInfo.
	MountInfo string
	// AfterScan runs, in order, for every Scan that finishes.
	AfterScan []AfterScan
}

// Done is a Scan that finished.
type Done struct {
	ID                int64
	Started, Finished time.Time
}

// Run is a Scan that started.
type Run struct {
	ID      int64
	Started time.Time
	done    chan struct{}
	err     error
}

// Done is closed when the Scan has ended, finished or not.
func (r *Run) Done() <-chan struct{} { return r.done }

// Err is why the Scan failed, nil if it finished. Call it after Done.
func (r *Run) Err() error { return r.err }

// Status is what the Scanner is doing and did.
type Status struct {
	// Running is the Scan that runs now, nil if none.
	Running *Run
	// Last is the latest Scan that finished, nil if none has.
	Last *Done
}

// Scanner runs Scans and holds the latest tree.
type Scanner struct {
	cfg    Config
	ctx    context.Context // cancelled by Close
	cancel context.CancelFunc

	mu      sync.Mutex
	running *Run
	last    *Done
	tree    *tree.Tree
}

// New prepares the scan tables and loads the saved snapshot, if any.
func New(ctx context.Context, cfg Config) (*Scanner, error) {
	if cfg.MountInfo == "" {
		cfg.MountInfo = DefaultMountInfo
	}
	if err := openStore(ctx, cfg.DB, cfg.Clock.Now()); err != nil {
		return nil, fmt.Errorf("scan store: %w", err)
	}
	s := &Scanner{cfg: cfg}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	if done, ok, err := lastFinished(ctx, cfg.DB); err != nil {
		return nil, fmt.Errorf("scan store: %w", err)
	} else if ok {
		s.last = &done
	}
	t, err := loadSnapshot(ctx, cfg.DB)
	if err != nil {
		// Not fatal: the next Scan makes a new one.
		cfg.Log.Warn("saved tree not loaded", "err", err)
	}
	s.tree = t
	return s, nil
}

// Close stops a running Scan and waits for it to end.
func (s *Scanner) Close() {
	s.cancel()
	s.mu.Lock()
	r := s.running
	s.mu.Unlock()
	if r != nil {
		<-r.done
	}
}

// Tree is the latest Scan's tree, nil before the first.
func (s *Scanner) Tree() *tree.Tree {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tree
}

// Status says whether a Scan runs and which finished last.
func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Status{Running: s.running, Last: s.last}
}

// Start starts a Scan, or returns the running one: concurrent requests share
// one Scan.
func (s *Scanner) Start() (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running != nil {
		return s.running, nil
	}
	if err := s.ctx.Err(); err != nil {
		return nil, errors.New("scanner is closed")
	}
	started := s.cfg.Clock.Now()
	id, err := insertRunning(s.ctx, s.cfg.DB, started)
	if err != nil {
		return nil, err
	}
	r := &Run{ID: id, Started: started, done: make(chan struct{})}
	s.running = r
	go s.run(r)
	return r, nil
}

func (s *Scanner) run(r *Run) {
	log := s.cfg.Log.With("scan", r.ID)
	log.Info("scan started")
	t, err := s.walk(log)
	finished := s.cfg.Clock.Now()
	done := Done{ID: r.ID, Started: r.Started, Finished: finished}
	if err == nil {
		err = saveFinished(s.ctx, s.cfg.DB, done, t, s.cfg.AfterScan)
	}
	if err != nil {
		// The context may be cancelled (stopping): record the failure anyway.
		if ferr := markFailed(context.WithoutCancel(s.ctx), s.cfg.DB, r.ID, finished); ferr != nil {
			log.Error("scan not recorded as failed", "err", ferr)
		}
		log.Error("scan failed", "err", err)
	} else {
		log.Info("scan finished", "duration", finished.Sub(r.Started).String(),
			"size", t.Root.Size, "items", t.Root.Items)
	}

	s.mu.Lock()
	r.err = err
	s.running = nil
	if err == nil {
		s.tree = t
		s.last = &done
	}
	s.mu.Unlock()
	close(r.done)
}

// walk walks every Disk Unraid lists, one after another, on one OS thread at
// idle I/O priority, into one merged tree.
func (s *Scanner) walk(log *slog.Logger) (*tree.Tree, error) {
	arr, err := s.cfg.Inventory.Read()
	if err != nil {
		return nil, err
	}
	mounts, err := readMountPoints(s.cfg.MountInfo)
	if err != nil {
		return nil, err
	}
	b := tree.NewBuilder()
	w := &walker{ctx: s.ctx, b: b, mounts: mounts}
	result := make(chan error, 1)
	go func() {
		// Deliberately never unlocked: the thread ends with this goroutine,
		// so its idle priority never reaches other work.
		runtime.LockOSThread()
		if err := beIdle(); err != nil {
			log.Warn("scan runs at normal priority", "err", err)
		}
		for _, d := range arr.Disks {
			// Mount points are listed by their real path.
			root, err := filepath.EvalSymlinks(d.Path)
			if err == nil {
				err = w.disk(d.Name, root)
			}
			if err := s.ctx.Err(); err != nil {
				result <- err
				return
			}
			if err != nil {
				// Unraid lists it, but its folder isn't there (a bind mount
				// missing from the container, say): scan the others.
				log.Warn("disk left out of the scan", "disk", d.Name, "err", err)
			}
		}
		result <- nil
	}()
	if err := <-result; err != nil {
		return nil, err
	}
	if w.unreadable > 0 {
		log.Warn("folders not readable; their contents are not counted", "folders", w.unreadable)
	}
	return b.Finish(), nil
}
