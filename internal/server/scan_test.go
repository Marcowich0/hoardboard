package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// scanStatus is GET /api/scan's answer (ScanStatus in web/src/api/types.ts).
type scanStatus struct {
	Running *struct {
		ID        int64  `json:"id"`
		StartedAt string `json:"startedAt"`
	} `json:"running"`
	Last *struct {
		ID         int64  `json:"id"`
		StartedAt  string `json:"startedAt"`
		FinishedAt string `json:"finishedAt"`
		DurationMs int64  `json:"durationMs"`
	} `json:"last"`
	Skipped *struct{} `json:"skipped"`
	Next    *string   `json:"next"`
}

type scanRequest struct {
	Status  scanStatus `json:"status"`
	Started bool       `json:"started"`
}

func TestBeforeTheFirstScanThereIsNoLastScan(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	var got scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &got)
	if got.Running != nil || got.Last != nil || got.Skipped != nil {
		t.Fatalf("status = %+v, want nothing running, nothing finished", got)
	}
}

func TestTheStatusShowsWhenTheLastScanFinishedAndHowLongItTook(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	release := app.Mount.HoldScans()

	var started scanRequest
	app.PostJSON(t, "/api/scan", nil, http.StatusOK, &started)
	var running scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &running)
	app.Clock.Advance(3 * time.Second)
	release()
	app.Scan(t)
	var got scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &got)

	if !started.Started || started.Status.Running == nil || started.Status.Running.StartedAt != "2026-03-02T10:00:00.000Z" {
		t.Errorf("POST /api/scan = %+v, want a Scan running since 10:00", started)
	}
	if running.Running == nil || running.Last != nil {
		t.Errorf("status while it runs = %+v, want running and no last", running)
	}
	if got.Running != nil || got.Last == nil || got.Last.StartedAt != "2026-03-02T10:00:00.000Z" ||
		got.Last.FinishedAt != "2026-03-02T10:00:03.000Z" || got.Last.DurationMs != 3000 {
		t.Fatalf("status = %+v, want last 10:00:00 to 10:00:03, 3000 ms", got)
	}
}

func TestAScanRequestedWhileOneRunsJoinsIt(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/a.mkv", unit)
	release := app.Mount.HoldScans()

	var first, second scanRequest
	app.PostJSON(t, "/api/scan", nil, http.StatusOK, &first)
	app.Clock.Advance(time.Second)
	app.PostJSON(t, "/api/scan", nil, http.StatusOK, &second)
	release()
	app.Scan(t) // joins the same Scan once more, and waits for it
	var after scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &after)

	if first.Status.Running == nil || second.Status.Running == nil || !second.Started ||
		*first.Status.Running != *second.Status.Running {
		t.Fatalf("second request = %+v, want it to join %+v", second.Status.Running, first.Status.Running)
	}
	if after.Last == nil || after.Last.ID != first.Status.Running.ID {
		t.Fatalf("last = %+v, want the one Scan %d", after.Last, first.Status.Running.ID)
	}

	// Once it has finished, a request starts a new one.
	app.Scan(t)
	var next scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &next)
	if next.Last == nil || next.Last.ID == after.Last.ID {
		t.Fatalf("last = %+v, want a new Scan after %d", next.Last, after.Last.ID)
	}
}

func TestTheLastScanIsKnownAfterARestart(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Scan(t)
	var before scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &before)

	app.Clock.Advance(time.Hour)
	app.Restart(t)
	var after scanStatus
	app.GetJSON(t, "/api/scan", http.StatusOK, &after)

	if after.Last == nil || before.Last == nil || *after.Last != *before.Last {
		t.Fatalf("last after restart = %+v, want %+v", after.Last, before.Last)
	}
}

func TestScanRejectsABadWait(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.PostJSON(t, "/api/scan?wait=soon", nil, http.StatusBadRequest, nil)
}
