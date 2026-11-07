// Package clock is the source of "now" for everything in hoardboard that
// depends on the time: when a Scan ran, which day a History point belongs to,
// and when the next scheduled Scan is due. Production uses System; tests
// inject a Fake and move it by hand.
package clock

import (
	"slices"
	"sync"
	"time"
)

// Clock tells the current time and runs functions later.
type Clock interface {
	Now() time.Time
	// AfterFunc calls f in its own goroutine once d has passed, unless the
	// returned Timer is stopped first.
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is a pending AfterFunc call.
type Timer interface {
	// Stop cancels the call; it reports false if the call already ran or
	// was stopped.
	Stop() bool
}

// System is the real clock.
func System() Clock { return systemClock{} }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// Fake is a clock that only moves when told to. It is safe for concurrent use.
//
// Its AfterFunc calls run when Advance or Set moves the clock to or past
// their time, in order of that time, each with the clock reading the time it
// was due, and before Advance or Set returns: so a test that moves the clock
// sees at once what those calls did.
type Fake struct {
	mu      sync.Mutex
	now     time.Time
	pending []*fakeTimer
}

type fakeTimer struct {
	f   *Fake
	at  time.Time
	run func()
}

// AfterFunc schedules f for when the clock reaches now+d. With d <= 0 it
// calls f in its own goroutine at once, as time.AfterFunc does.
func (f *Fake) AfterFunc(d time.Duration, fn func()) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &fakeTimer{f: f, at: f.now.Add(d), run: fn}
	if d <= 0 {
		go fn()
		return t
	}
	f.pending = append(f.pending, t)
	return t
}

// Stop cancels the call if it has not run yet.
func (t *fakeTimer) Stop() bool {
	t.f.mu.Lock()
	defer t.f.mu.Unlock()
	i := slices.Index(t.f.pending, t)
	if i < 0 {
		return false
	}
	t.f.pending = slices.Delete(t.f.pending, i, i+1)
	return true
}

// moveTo runs every pending call due by target, earliest first, with the
// clock at its time, then leaves the clock at target. f.mu is held on entry
// and on return, but not while a call runs.
func (f *Fake) moveTo(target time.Time) {
	for {
		var due *fakeTimer
		for _, t := range f.pending {
			if !t.at.After(target) && (due == nil || t.at.Before(due.at)) {
				due = t
			}
		}
		if due == nil {
			break
		}
		f.pending = slices.DeleteFunc(f.pending, func(t *fakeTimer) bool { return t == due })
		if due.at.After(f.now) {
			f.now = due.at
		}
		f.mu.Unlock()
		due.run()
		f.mu.Lock()
	}
	f.now = target
}

// NewFake returns a Fake that reads start until it is moved.
func NewFake(start time.Time) *Fake { return &Fake{now: start} }

// Now returns the fake's current time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Set moves the fake to t, which may be earlier than now.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.moveTo(t)
}

// Advance moves the fake forward by d and returns the new time.
func (f *Fake) Advance(d time.Duration) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.moveTo(f.now.Add(d))
	return f.now
}
