package server_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// history is GET /api/history?path='s answer (FolderHistory in
// web/src/api/types.ts).
type history struct {
	Path   string  `json:"path"`
	Points []point `json:"points"`
}

type point struct {
	Day         string `json:"day"`
	Size        int64  `json:"size"`
	SharedBytes int64  `json:"sharedBytes"`
}

const day = 24 * time.Hour

func folderHistory(t *testing.T, app *apitest.App, path string) []point {
	t.Helper()
	var got history
	app.GetJSON(t, "/api/history?path="+path, http.StatusOK, &got)
	if got.Path != path {
		t.Errorf("history path = %q, want %q", got.Path, path)
	}
	return got.Points
}

func TestScansOnSeveralDaysMakeOnePointPerFolderPerDay(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // Epoch is 2026-03-02, mid-morning
	app.Mount.File("disk1", "data/media/movies/a.mkv", 1*unit)
	app.Scan(t)

	app.Clock.Advance(day)
	app.Mount.File("disk2", "data/media/movies/b.mkv", 2*unit)
	app.Scan(t)

	app.Clock.Advance(day)
	app.Mount.File("disk1", "data/media/tv/c.mkv", 4*unit)
	app.Scan(t)

	want := []point{
		{"2026-03-02", 1 * unit, 0},
		{"2026-03-03", 3 * unit, 0},
		{"2026-03-04", 3 * unit, 0},
	}
	if got := folderHistory(t, app, "data/media/movies"); !slices.Equal(got, want) {
		t.Errorf("movies history = %+v, want %+v", got, want)
	}
	want = []point{
		{"2026-03-02", 1 * unit, 0},
		{"2026-03-03", 3 * unit, 0},
		{"2026-03-04", 7 * unit, 0},
	}
	if got := folderHistory(t, app, "data/media"); !slices.Equal(got, want) {
		t.Errorf("data/media history = %+v, want %+v", got, want)
	}
	if got := folderHistory(t, app, ""); !slices.Equal(got, want) {
		t.Errorf("root history = %+v, want %+v", got, want)
	}
	want = []point{{"2026-03-04", 4 * unit, 0}}
	if got := folderHistory(t, app, "data/media/tv"); !slices.Equal(got, want) {
		t.Errorf("tv history = %+v (it appeared on the third day), want %+v", got, want)
	}
}

// changes is GET /api/tree's change over the period: the Folder's own, then
// each child Folder's by name.
type changes struct {
	Folder struct {
		Change *int64 `json:"change"`
	} `json:"folder"`
	Children []struct {
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Change *int64 `json:"change"`
	} `json:"children"`
}

func treeChanges(t *testing.T, app *apitest.App, path, period string) map[string]*int64 {
	t.Helper()
	var got changes
	app.GetJSON(t, "/api/tree?path="+path+"&period="+period, http.StatusOK, &got)
	out := map[string]*int64{".": got.Folder.Change}
	for _, c := range got.Children {
		if c.Kind == "folder" {
			out[c.Name] = c.Change
		}
	}
	return out
}

func delta(n int64) *int64 { return &n }

func checkChanges(t *testing.T, app *apitest.App, path, period string, want map[string]*int64) {
	t.Helper()
	got := treeChanges(t, app, path, period)
	show := func(m map[string]*int64) map[string]any {
		out := map[string]any{}
		for k, v := range m {
			if v == nil {
				out[k] = nil
			} else {
				out[k] = *v
			}
		}
		return out
	}
	if len(got) != len(want) {
		t.Errorf("%s change over %s = %v, want %v", path, period, show(got), show(want))
		return
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok || (g == nil) != (w == nil) || (g != nil && *g != *w) {
			t.Errorf("%s change over %s = %v, want %v", path, period, show(got), show(want))
			return
		}
	}
}

func TestTheTreeShowsTheChangeOverThePeriod(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02: History starts
	app.Mount.File("disk1", "data/media/movies/a.mkv", 1*unit)
	app.Scan(t)

	app.Clock.Advance(7 * day) // 2026-03-09
	app.Mount.File("disk2", "data/media/movies/b.mkv", 2*unit)
	app.Mount.File("disk1", "data/media/tv/c.mkv", 4*unit)
	app.Scan(t)

	// tv did not exist a week ago: it grew by all of it.
	checkChanges(t, app, "data/media", "7d", map[string]*int64{
		".": delta(6 * unit), "movies": delta(2 * unit), "tv": delta(4 * unit),
	})
	// History doesn't reach back 30 days yet.
	checkChanges(t, app, "data/media", "30d", map[string]*int64{".": nil, "movies": nil, "tv": nil})

	app.Clock.Advance(23 * day) // 2026-04-01, 30 days after the first Scan
	app.Mount.Remove("disk1", "data/media/movies/a.mkv")
	app.Scan(t)

	// A week ago (2026-03-25) had no Scan: the value of 2026-03-09 carries.
	checkChanges(t, app, "data/media", "7d", map[string]*int64{
		".": delta(-1 * unit), "movies": delta(-1 * unit), "tv": delta(0),
	})
	checkChanges(t, app, "data/media", "30d", map[string]*int64{
		".": delta(5 * unit), "movies": delta(1 * unit), "tv": delta(4 * unit),
	})
	checkChanges(t, app, "data/media", "1y", map[string]*int64{".": nil, "movies": nil, "tv": nil})

	app.Clock.Set(apitest.Epoch.Add(365 * day)) // 2027-03-02, no Scan since 2026-04-01
	checkChanges(t, app, "data/media", "1y", map[string]*int64{
		".": delta(5 * unit), "movies": delta(1 * unit), "tv": delta(4 * unit),
	})
	checkChanges(t, app, "data/media", "30d", map[string]*int64{
		".": delta(0), "movies": delta(0), "tv": delta(0),
	})
}

func TestTheLastScanOfADayWins(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02 10:00
	app.Mount.File("disk1", "data/movies/a.mkv", 1*unit)
	app.Scan(t)

	app.Clock.Advance(day)
	app.Mount.File("disk1", "data/movies/b.mkv", 2*unit)
	app.Scan(t)
	app.Clock.Advance(time.Hour)
	app.Mount.File("disk1", "data/movies/c.mkv", 4*unit)
	app.Scan(t)

	want := []point{{"2026-03-02", 1 * unit, 0}, {"2026-03-03", 7 * unit, 0}}
	if got := folderHistory(t, app, "data/movies"); !slices.Equal(got, want) {
		t.Errorf("history = %+v, want %+v", got, want)
	}

	// Back to the day before's value by the day's last Scan.
	app.Clock.Advance(time.Hour)
	app.Mount.Remove("disk1", "data/movies/b.mkv")
	app.Mount.Remove("disk1", "data/movies/c.mkv")
	app.Scan(t)
	want = []point{{"2026-03-02", 1 * unit, 0}, {"2026-03-03", 1 * unit, 0}}
	if got := folderHistory(t, app, "data/movies"); !slices.Equal(got, want) {
		t.Errorf("history after undoing the day = %+v, want %+v", got, want)
	}
}

func TestDaysWithoutAChangeOrAScanCarryTheLastValue(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/movies/a.mkv", 1*unit)
	app.Scan(t)
	app.Clock.Advance(day)
	app.Scan(t) // unchanged
	app.Clock.Advance(2 * day)
	app.Mount.File("disk1", "data/movies/b.mkv", 2*unit)
	app.Scan(t)
	app.Clock.Advance(2 * day) // today, 2026-03-07, has no Scan yet

	want := []point{
		{"2026-03-02", 1 * unit, 0},
		{"2026-03-03", 1 * unit, 0},
		{"2026-03-04", 1 * unit, 0},
		{"2026-03-05", 3 * unit, 0},
		{"2026-03-06", 3 * unit, 0},
		{"2026-03-07", 3 * unit, 0},
	}
	if got := folderHistory(t, app, "data/movies"); !slices.Equal(got, want) {
		t.Errorf("history = %+v, want %+v", got, want)
	}

	app.Restart(t)
	if got := folderHistory(t, app, "data/movies"); !slices.Equal(got, want) {
		t.Errorf("history after a restart = %+v, want %+v", got, want)
	}
}

func TestADeletedFoldersHistoryIsKept(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/old/a.mkv", 2*unit)
	app.Mount.File("disk1", "data/keep/b.mkv", 1*unit)
	app.Scan(t)
	app.Clock.Advance(day)
	app.Mount.Remove("disk1", "data/old")
	app.Scan(t)
	app.Clock.Advance(day)
	app.Scan(t)

	want := []point{
		{"2026-03-02", 2 * unit, 0},
		{"2026-03-03", 0, 0},
		{"2026-03-04", 0, 0},
	}
	if got := folderHistory(t, app, "data/old"); !slices.Equal(got, want) {
		t.Errorf("deleted folder's history = %+v, want %+v", got, want)
	}

	// It comes back under the same path: its History goes on.
	app.Clock.Advance(day)
	app.Mount.File("disk2", "data/old/c.mkv", 1*unit)
	app.Scan(t)
	want = append(want, point{"2026-03-05", 1 * unit, 0})
	if got := folderHistory(t, app, "data/old"); !slices.Equal(got, want) {
		t.Errorf("returning folder's history = %+v, want %+v", got, want)
	}
}

func TestHistoryRecordsSharedBytes(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/seeding/a.mkv", 2*unit)
	app.Scan(t)
	app.Clock.Advance(day)
	app.Mount.Link("disk1", "data/seeding/a.mkv", "data/media/a.mkv")
	app.Scan(t)

	want := []point{{"2026-03-02", 2 * unit, 0}, {"2026-03-03", 2 * unit, 2 * unit}}
	if got := folderHistory(t, app, "data/seeding"); !slices.Equal(got, want) {
		t.Errorf("seeding history = %+v, want %+v", got, want)
	}
	want = []point{{"2026-03-02", 2 * unit, 0}, {"2026-03-03", 2 * unit, 0}}
	if got := folderHistory(t, app, "data"); !slices.Equal(got, want) {
		t.Errorf("data history = %+v, want %+v (both names are inside it)", got, want)
	}
}

func TestADayIsTheServersLocalDay(t *testing.T) {
	t.Parallel()
	copenhagen, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		t.Fatal(err)
	}
	app := apitest.Start(t)
	// 23:30 and 00:30 in Copenhagen (UTC+1 in March): one UTC day, two local days.
	app.Clock.Set(time.Date(2026, time.March, 2, 23, 30, 0, 0, copenhagen))
	app.Mount.File("disk1", "data/movies/a.mkv", 1*unit)
	app.Scan(t)
	app.Clock.Advance(time.Hour)
	app.Mount.File("disk1", "data/movies/b.mkv", 2*unit)
	app.Scan(t)

	want := []point{{"2026-03-02", 1 * unit, 0}, {"2026-03-03", 3 * unit, 0}}
	if got := folderHistory(t, app, "data/movies"); !slices.Equal(got, want) {
		t.Errorf("history = %+v, want %+v", got, want)
	}
}

func TestHistoryBeforeTheFirstScanIsEmpty(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	if got := folderHistory(t, app, ""); len(got) != 0 {
		t.Errorf("root history = %+v, want none", got)
	}
	checkChanges(t, app, "", "7d", map[string]*int64{".": nil})
}

func TestHistoryRejectsBadRequests(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/movies/a.mkv", 1*unit)
	app.Scan(t)
	for path, status := range map[string]int{
		"/api/history?path=data/nope":   http.StatusNotFound,
		"/api/history?path=/data":       http.StatusBadRequest,
		"/api/history?path=data/../x":   http.StatusBadRequest,
		"/api/history?path=data/":       http.StatusBadRequest,
		"/api/history?path=data/movies": http.StatusOK,
	} {
		app.GetJSON(t, path, status, nil)
	}
}
