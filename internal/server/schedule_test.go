package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// skippedScan is ScanStatus.skipped in web/src/api/types.ts.
type skippedScan struct {
	At     string `json:"at"`
	Reason string `json:"reason"`
}

func TestScanNowIsSkippedDuringAParityCheck(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Status.StartParityCheck()

	var got scanRequest
	app.PostJSON(t, "/api/scan?wait=true", nil, http.StatusOK, &got)

	want := skippedScan{At: "2026-03-02T10:00:00.000Z", Reason: "Parity check in progress"}
	if got.Started || got.Status.Running != nil || got.Status.Last != nil ||
		got.Status.Skipped == nil || *got.Status.Skipped != want {
		t.Fatalf("POST /api/scan = %+v, want skipped %+v", got, want)
	}
}

func TestScanNowIsSkippedDuringArrayWork(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		work   func(*apitest.Status)
		reason string
	}{
		{"paused parity check", (*apitest.Status).PauseParityCheck, "Parity check paused"},
		{"rebuild", func(s *apitest.Status) { s.StartRebuild("disk2") }, "Disk rebuild in progress"},
		{"parity sync", func(s *apitest.Status) {
			s.SetVar("mdResyncAction", "recon P")
			s.SetVar("mdResync", "11718885324")
			s.SetVar("mdResyncPos", "42")
		}, "Parity sync in progress"},
		{"mover", (*apitest.Status).StartMover, "Mover is running"},
		{"stopped array", (*apitest.Status).StopArray, "Array is not started"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			app := apitest.Start(t)
			c.work(app.Status)

			var got scanRequest
			app.PostJSON(t, "/api/scan", nil, http.StatusOK, &got)
			var status scanStatus
			app.GetJSON(t, "/api/scan", http.StatusOK, &status)

			want := skippedScan{At: "2026-03-02T10:00:00.000Z", Reason: c.reason}
			if got.Started || status.Running != nil || status.Skipped == nil || *status.Skipped != want {
				t.Fatalf("POST /api/scan = %+v, then status %+v; want skipped %+v", got, status, want)
			}
		})
	}
}

func TestWhenArrayWorkEndsTheNextScanRuns(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/a.mkv", unit)
	app.Status.StartMover()
	app.PostJSON(t, "/api/scan", nil, http.StatusOK, nil)

	app.Clock.Advance(time.Minute)
	app.Status.Idle()
	app.Scan(t)
	var got scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &got)
	var root struct {
		Folder struct {
			Size int64 `json:"size"`
		} `json:"folder"`
	}
	app.GetJSON(t, "/api/tree?path=data", http.StatusOK, &root)

	if got.Last == nil || got.Last.StartedAt != "2026-03-02T10:01:00.000Z" || got.Skipped != nil {
		t.Fatalf("status = %+v, want last at 10:01 and the earlier skip no longer shown", got)
	}
	if root.Folder.Size != unit {
		t.Fatalf("data size = %d, want %d", root.Folder.Size, unit)
	}
}

func TestASkipAfterTheLastScanIsKnownAfterARestart(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Scan(t)
	app.Clock.Advance(time.Hour)
	app.Status.StartParityCheck()
	app.PostJSON(t, "/api/scan", nil, http.StatusOK, nil)

	app.Restart(t)
	var got scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &got)

	want := skippedScan{At: "2026-03-02T11:00:00.000Z", Reason: "Parity check in progress"}
	if got.Last == nil || got.Last.StartedAt != "2026-03-02T10:00:00.000Z" || got.Skipped == nil || *got.Skipped != want {
		t.Fatalf("status after restart = %+v, want last at 10:00 and skipped %+v", got, want)
	}
}

// startScheduled starts a server that schedules Scans every 30 minutes, with
// Scans held at their start until release; the first Scan starts with it.
func startScheduled(t *testing.T, opts ...apitest.Option) (app *apitest.App, release func()) {
	t.Helper()
	mount := apitest.NewMount(t)
	release = mount.HoldScans()
	opts = append([]apitest.Option{apitest.WithMount(mount), apitest.WithSchedule(),
		apitest.WithEnv("HOARDBOARD_SCAN_INTERVAL", "30m")}, opts...)
	return apitest.Start(t, opts...), release
}

func TestScansRunAtTheConfiguredInterval(t *testing.T) {
	t.Parallel()
	app, release := startScheduled(t)

	var first scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &first)
	release()
	app.Scan(t)
	release = app.Mount.HoldScans()
	app.Clock.Advance(29 * time.Minute)
	var early scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &early)
	app.Clock.Advance(time.Minute)
	var second scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &second)
	release()
	app.Scan(t)
	var after scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &after)

	if first.Running == nil || first.Running.StartedAt != "2026-03-02T10:00:00.000Z" ||
		first.Next == nil || *first.Next != "2026-03-02T10:30:00.000Z" {
		t.Fatalf("at start: %+v, want a Scan running since 10:00 and the next due 10:30", first)
	}
	if early.Running != nil || early.Next == nil || *early.Next != "2026-03-02T10:30:00.000Z" {
		t.Fatalf("at 10:29: %+v, want nothing running, the next due 10:30", early)
	}
	if second.Running == nil || second.Running.StartedAt != "2026-03-02T10:30:00.000Z" ||
		second.Next == nil || *second.Next != "2026-03-02T11:00:00.000Z" {
		t.Fatalf("at 10:30: %+v, want a Scan running since 10:30 and the next due 11:00", second)
	}
	if after.Last == nil || after.Last.StartedAt != "2026-03-02T10:30:00.000Z" {
		t.Fatalf("last = %+v, want the Scan of 10:30", after.Last)
	}
}

func TestWithoutAScheduleNothingIsDue(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Clock.Advance(2 * time.Hour)

	var got scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &got)
	if got.Running != nil || got.Last != nil || got.Next != nil {
		t.Fatalf("status = %+v, want no Scan and none due", got)
	}
}

func TestScheduledScansAreSkippedDuringArrayWork(t *testing.T) {
	t.Parallel()
	status := apitest.NewStatus(t)
	status.StartParityCheck()
	app, release := startScheduled(t, apitest.WithStatus(status))
	release()

	var skipped scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &skipped)
	app.Status.Idle()
	app.Clock.Advance(30 * time.Minute)
	app.Scan(t)
	var after scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &after)

	want := skippedScan{At: "2026-03-02T10:00:00.000Z", Reason: "Parity check in progress"}
	if skipped.Running != nil || skipped.Skipped == nil || *skipped.Skipped != want ||
		skipped.Next == nil || *skipped.Next != "2026-03-02T10:30:00.000Z" {
		t.Fatalf("at start: %+v, want skipped %+v and the next due 10:30", skipped, want)
	}
	if after.Last == nil || after.Last.StartedAt != "2026-03-02T10:30:00.000Z" || after.Skipped != nil {
		t.Fatalf("after the check: %+v, want the Scan of 10:30 and no skip shown", after)
	}
}

func TestAScheduledAndARequestedScanShareOneScan(t *testing.T) {
	t.Parallel()
	app, release := startScheduled(t)

	// The scheduled Scan runs; a request joins it.
	var scheduled scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &scheduled)
	var joined scanRequest
	app.PostJSON(t, "/api/scan", nil, http.StatusOK, &joined)
	release()
	app.Scan(t)

	// A requested Scan runs; the schedule joins it.
	release = app.Mount.HoldScans()
	app.Clock.Advance(29 * time.Minute)
	var requested scanRequest
	app.PostJSON(t, "/api/scan", nil, http.StatusOK, &requested)
	app.Clock.Advance(time.Minute)
	var tick scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &tick)
	release()

	if scheduled.Running == nil || !joined.Started || joined.Status.Running == nil ||
		*joined.Status.Running != *scheduled.Running {
		t.Fatalf("request = %+v, want it to join the scheduled Scan %+v", joined, scheduled.Running)
	}
	if requested.Status.Running == nil || tick.Running == nil || *tick.Running != *requested.Status.Running ||
		tick.Running.StartedAt != "2026-03-02T10:29:00.000Z" {
		t.Fatalf("at 10:30 running = %+v, want the requested Scan %+v", tick.Running, requested.Status.Running)
	}
}

func TestAfterARestartTheNextScanIsDueOneIntervalAfterTheLast(t *testing.T) {
	t.Parallel()
	app, release := startScheduled(t)
	release()
	app.Scan(t)

	app.Clock.Advance(20 * time.Minute)
	app.Restart(t)
	var got scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &got)

	if got.Running != nil || got.Next == nil || *got.Next != "2026-03-02T10:30:00.000Z" {
		t.Fatalf("after restart: %+v, want nothing running and the next due 10:30", got)
	}
}
