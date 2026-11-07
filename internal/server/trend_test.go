package server_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// trend is the stacked category series of GET /api/overview (Trend in
// web/src/api/types.ts).
type trend struct {
	Days       []string `json:"days"`
	Categories []struct {
		CategoryID int64   `json:"categoryId"`
		Bytes      []int64 `json:"bytes"`
	} `json:"categories"`
	Uncategorized []int64 `json:"uncategorized"`
	Unaccounted   []int64 `json:"unaccounted"`
}

type overviewTrend struct {
	Totals space `json:"totals"`
	Donut  donut `json:"donut"`
	Trend  trend `json:"trend"`
}

func getTrend(t *testing.T, app *apitest.App) overviewTrend {
	t.Helper()
	var got overviewTrend
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)
	n := len(got.Trend.Days)
	if got.Trend.Days == nil || got.Trend.Categories == nil || len(got.Trend.Uncategorized) != n || len(got.Trend.Unaccounted) != n {
		t.Fatalf("trend = %+v, want days and a value per day in every series", got.Trend)
	}
	for _, c := range got.Trend.Categories {
		if len(c.Bytes) != n {
			t.Fatalf("trend category %d has %d values, want %d", c.CategoryID, len(c.Bytes), n)
		}
	}
	return got
}

func TestTheTrendSplitsEachDayByRankAndAReorderSplitsThePastAgain(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	if got := getTrend(t, app).Trend; len(got.Days) != 0 {
		t.Fatalf("before a Scan: trend days = %v, want none", got.Days)
	}

	// film.mkv seeds in seeding and is linked into movies.
	app.Mount.File("disk1", "data/seeding/film.mkv", 4*unit)
	app.Mount.Link("disk1", "data/seeding/film.mkv", "data/media/movies/film.mkv")
	app.Mount.File("disk1", "data/seeding/seed.iso", 2*unit)
	app.Mount.File("disk2", "data/media/movies/own.mkv", 1*unit)
	app.Mount.File("cache", "data/downloads/other.bin", 1*unit)
	app.Scan(t) // 2026-03-02

	app.Clock.Advance(day)
	app.Mount.File("disk2", "data/media/movies/new.mkv", 2*unit)
	app.Mount.Link("disk2", "data/media/movies/new.mkv", "data/seeding/new.mkv")
	app.Scan(t) // 2026-03-03

	movies := create(t, app, "Movies", "#e5484d")
	seeding := create(t, app, "Seeding", "#8e4ec6")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/seeding", seeding.ID)

	check := func(when string, ids []int64, want map[int64][]int64) {
		t.Helper()
		got := getTrend(t, app)
		if days := []string{"2026-03-02", "2026-03-03"}; !slices.Equal(got.Trend.Days, days) {
			t.Fatalf("%s: trend days = %v, want %v", when, got.Trend.Days, days)
		}
		var order []int64
		for _, c := range got.Trend.Categories {
			order = append(order, c.CategoryID)
			if !slices.Equal(c.Bytes, want[c.CategoryID]) {
				t.Errorf("%s: category %d = %v, want %v", when, c.CategoryID, c.Bytes, want[c.CategoryID])
			}
		}
		if !slices.Equal(order, ids) {
			t.Errorf("%s: trend categories %v, want Rank order %v", when, order, ids)
		}
		if u := []int64{1 * unit, 1 * unit}; !slices.Equal(got.Trend.Uncategorized, u) {
			t.Errorf("%s: uncategorized = %v, want %v", when, got.Trend.Uncategorized, u)
		}
		// Today's stack is the donut.
		last := len(got.Trend.Days) - 1
		today := donut{Uncategorized: got.Trend.Uncategorized[last], Unaccounted: got.Trend.Unaccounted[last]}
		for _, c := range got.Trend.Categories {
			today.Categories = append(today.Categories, slice{c.CategoryID, c.Bytes[last]})
		}
		if !slices.Equal(today.Categories, got.Donut.Categories) || today.Uncategorized != got.Donut.Uncategorized ||
			today.Unaccounted != got.Donut.Unaccounted {
			t.Errorf("%s: today's stack = %+v, want the donut %+v", when, today, got.Donut)
		}
	}
	// Each day's parts add up to what that day's Scan saw, 8 then 10 units:
	// film.mkv and new.mkv once, in the higher Rank.
	check("Movies over Seeding", []int64{movies.ID, seeding.ID}, map[int64][]int64{
		movies.ID:  {5 * unit, 7 * unit},
		seeding.ID: {2 * unit, 2 * unit},
	})
	reorder(t, app, seeding.ID, movies.ID)
	check("Seeding over Movies", []int64{seeding.ID, movies.ID}, map[int64][]int64{
		seeding.ID: {6 * unit, 8 * unit},
		movies.ID:  {1 * unit, 1 * unit},
	})
}

func TestEachDaysUnaccountedIsThatDaysUsedSpaceLessWhatItsScanSaw(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.SetSpace("disk2", 1000*unit, 50*unit)
	app.Mount.SetSpace("cache", 1000*unit, 10*unit)

	app.Mount.SetSpace("disk1", 1000*unit, 100*unit)
	app.Mount.File("disk1", "data/a.bin", 3*unit)
	app.Scan(t) // 2026-03-02: Used 160 units, 3 seen

	app.Clock.Advance(day)
	app.Mount.SetSpace("disk1", 1000*unit, 120*unit)
	app.Mount.File("disk1", "data/b.bin", 2*unit)
	app.Scan(t) // 2026-03-03: Used 180 units, 5 seen

	// 2026-03-04, no Scan: today's Used space is live.
	app.Clock.Advance(day)
	app.Mount.SetSpace("disk1", 1000*unit, 130*unit)

	got := getTrend(t, app)
	want := []int64{157 * unit, 175 * unit, 185 * unit}
	if !slices.Equal(got.Trend.Unaccounted, want) {
		t.Errorf("unaccounted = %v, want %v", got.Trend.Unaccounted, want)
	}
	if u := []int64{3 * unit, 5 * unit, 5 * unit}; !slices.Equal(got.Trend.Uncategorized, u) {
		t.Errorf("uncategorized = %v, want %v", got.Trend.Uncategorized, u)
	}
}
