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

	// film.mkv seeds in torrents and is linked into movies.
	app.Mount.File("disk1", "data/torrents/film.mkv", 4*unit)
	app.Mount.Link("disk1", "data/torrents/film.mkv", "data/media/movies/film.mkv")
	app.Mount.File("disk1", "data/torrents/seed.iso", 2*unit)
	app.Mount.File("disk2", "data/media/movies/own.mkv", 1*unit)
	app.Mount.File("cache", "data/downloads/other.bin", 1*unit)
	app.Scan(t) // 2026-03-02

	app.Clock.Advance(day)
	app.Mount.File("disk2", "data/media/movies/new.mkv", 2*unit)
	app.Mount.Link("disk2", "data/media/movies/new.mkv", "data/torrents/new.mkv")
	app.Scan(t) // 2026-03-03

	movies := create(t, app, "Movies", "#e5484d")
	torrents := create(t, app, "Torrents", "#8e4ec6")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/torrents", torrents.ID)

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
	check("Movies over Torrents", []int64{movies.ID, torrents.ID}, map[int64][]int64{
		movies.ID:   {5 * unit, 7 * unit},
		torrents.ID: {2 * unit, 2 * unit},
	})
	reorder(t, app, torrents.ID, movies.ID)
	check("Torrents over Movies", []int64{torrents.ID, movies.ID}, map[int64][]int64{
		torrents.ID: {6 * unit, 8 * unit},
		movies.ID:   {1 * unit, 1 * unit},
	})
}
