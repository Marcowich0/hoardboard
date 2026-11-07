package server_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// overviewDonut is the part of GET /api/overview that splits Used space.
type overviewDonut struct {
	Totals space `json:"totals"`
	Donut  donut `json:"donut"`
}

// donut is UsedSplit in web/src/api/types.ts.
type donut struct {
	Categories    []slice `json:"categories"`
	Uncategorized int64   `json:"uncategorized"`
	Unaccounted   int64   `json:"unaccounted"`
}

type slice struct {
	CategoryID int64 `json:"categoryId"`
	Bytes      int64 `json:"bytes"`
}

func (d donut) sum() int64 {
	n := d.Uncategorized + d.Unaccounted
	for _, c := range d.Categories {
		n += c.Bytes
	}
	return n
}

func getDonut(t *testing.T, app *apitest.App) overviewDonut {
	t.Helper()
	var got overviewDonut
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)
	if got.Donut.Categories == nil {
		t.Fatalf("donut categories = null, want a list")
	}
	return got
}

func TestADonutBytesHardlinkedIntoTwoCategoriesGoToTheHigherRank(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	// film.mkv seeds in seeding and is linked into movies; each also has a
	// file of its own. show.mkv is in tv and also left in downloads, which
	// no Assignment covers; other.bin is only there.
	app.Mount.File("disk1", "data/seeding/film.mkv", 4*unit)
	app.Mount.Link("disk1", "data/seeding/film.mkv", "data/media/movies/film.mkv")
	app.Mount.File("disk1", "data/seeding/seed.iso", 2*unit)
	app.Mount.File("disk2", "data/media/movies/own.mkv", 1*unit)
	app.Mount.File("disk2", "data/downloads/show.mkv", 3*unit)
	app.Mount.Link("disk2", "data/downloads/show.mkv", "data/media/tv/show.mkv")
	app.Mount.File("cache", "data/downloads/other.bin", 1*unit)
	app.Scan(t)

	movies := create(t, app, "Movies", "#e5484d")
	seeding := create(t, app, "Seeding", "#8e4ec6")
	tv := create(t, app, "TV", "#3e63dd")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/seeding", seeding.ID)
	assign(t, app, "data/media/tv", tv.ID)

	got := getDonut(t, app).Donut
	want := []slice{{movies.ID, 5 * unit}, {seeding.ID, 2 * unit}, {tv.ID, 3 * unit}}
	if !slices.Equal(got.Categories, want) || got.Uncategorized != 1*unit {
		t.Fatalf("Movies over Seeding: donut = %+v, uncategorized %d; want %+v, uncategorized %d",
			got.Categories, got.Uncategorized, want, 1*unit)
	}

	reorder(t, app, seeding.ID, movies.ID, tv.ID)
	got = getDonut(t, app).Donut
	want = []slice{{seeding.ID, 6 * unit}, {movies.ID, 1 * unit}, {tv.ID, 3 * unit}}
	if !slices.Equal(got.Categories, want) || got.Uncategorized != 1*unit {
		t.Fatalf("Seeding over Movies: donut = %+v, uncategorized %d; want %+v, uncategorized %d",
			got.Categories, got.Uncategorized, want, 1*unit)
	}
}

func TestTheDonutAddsUpToUsedSpace(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	// Before any Scan, nothing is accounted for: all of Used space is
	// Unaccounted.
	before := getDonut(t, app)
	if before.Donut.Unaccounted != before.Totals.Used || before.Donut.sum() != before.Totals.Used {
		t.Fatalf("before a Scan: donut = %+v, used %d; want it all Unaccounted", before.Donut, before.Totals.Used)
	}

	app.Mount.File("disk1", "data/media/movies/a.mkv", 4*unit)
	app.Mount.Link("disk1", "data/media/movies/a.mkv", "data/seeding/a.mkv")
	app.Mount.File("disk2", "data/media/movies/b.mkv", 2*unit)
	app.Mount.File("cache", "appdata/plex/db", 1*unit)
	app.Scan(t)
	movies := create(t, app, "Movies", "#e5484d")
	seeding := create(t, app, "Seeding", "#8e4ec6")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/seeding", seeding.ID)

	got := getDonut(t, app)
	if got.Donut.sum() != got.Totals.Used {
		t.Errorf("donut %+v sums to %d, want Used space %d", got.Donut, got.Donut.sum(), got.Totals.Used)
	}
	if got.Donut.Unaccounted < 0 {
		t.Errorf("unaccounted = %d, want never negative", got.Donut.Unaccounted)
	}
	// What the Scan accounted for: 7 units, a.mkv once.
	if accounted := got.Donut.sum() - got.Donut.Unaccounted; accounted != 7*unit {
		t.Errorf("categories + uncategorized = %d, want %d", accounted, 7*unit)
	}
}

func TestWhenTheScanSawMoreThanUsedSpaceThePartsShrinkToFitIt(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 5*unit)
	app.Mount.File("disk2", "data/media/tv/b.mkv", 3*unit)
	app.Mount.File("cache", "data/downloads/c.bin", 1*unit)
	// The filesystems report 4 units used (65536 bytes) of the 9 the Scan
	// sees (btrfs compression, say).
	low := func() {
		app.Mount.SetSpace("disk1", 1000*unit, 2*unit)
		app.Mount.SetSpace("disk2", 1000*unit, 1*unit)
		app.Mount.SetSpace("cache", 1000*unit, 1*unit)
	}
	low()
	app.Scan(t) // 2026-03-02

	app.Clock.Advance(day)
	app.Mount.SetSpace("disk1", 1000*unit, 8*unit) // 12 units in all
	app.Scan(t)                                    // 2026-03-03
	low()                                          // and live Used space falls back to 4 units

	movies := create(t, app, "Movies", "#e5484d")
	tv := create(t, app, "TV", "#3e63dd")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/media/tv", tv.ID)

	// 5:3:1 of 65536 bytes is 36408.9, 21845.3 and 7281.8: each part rounded
	// down, and the 2 bytes left over go to the largest, Movies.
	scaled := donut{Categories: []slice{{movies.ID, 36410}, {tv.ID, 21845}}, Uncategorized: 7281}
	got := getTrend(t, app)
	if got.Totals.Used != 4*unit {
		t.Fatalf("used = %d, want %d", got.Totals.Used, 4*unit)
	}
	if !slices.Equal(got.Donut.Categories, scaled.Categories) || got.Donut.Uncategorized != scaled.Uncategorized ||
		got.Donut.Unaccounted != 0 || got.Donut.sum() != 4*unit {
		t.Errorf("donut = %+v, want %+v summing to %d", got.Donut, scaled, 4*unit)
	}

	// 2026-03-02 is scaled the same way; 2026-03-03 (today, live) too.
	want := map[int64][]int64{movies.ID: {36410, 36410}, tv.ID: {21845, 21845}}
	for _, c := range got.Trend.Categories {
		if !slices.Equal(c.Bytes, want[c.CategoryID]) {
			t.Errorf("trend category %d = %v, want %v", c.CategoryID, c.Bytes, want[c.CategoryID])
		}
	}
	if u := []int64{7281, 7281}; !slices.Equal(got.Trend.Uncategorized, u) {
		t.Errorf("trend uncategorized = %v, want %v", got.Trend.Uncategorized, u)
	}
	if u := []int64{0, 0}; !slices.Equal(got.Trend.Unaccounted, u) {
		t.Errorf("trend unaccounted = %v, want %v", got.Trend.Unaccounted, u)
	}
}

func TestEachDayOfTheTrendShrinksToFitItsOwnUsedSpace(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 5*unit)
	app.Mount.SetSpace("disk1", 1000*unit, 1*unit)
	app.Mount.SetSpace("disk2", 1000*unit, 1*unit)
	app.Mount.SetSpace("cache", 1000*unit, 1*unit)
	app.Scan(t) // 2026-03-02: 3 units used, 5 seen

	app.Clock.Advance(day)
	app.Mount.SetSpace("disk1", 1000*unit, 10*unit)
	app.Scan(t) // 2026-03-03: 12 units used, 5 seen
	movies := create(t, app, "Movies", "#e5484d")
	assign(t, app, "data/media/movies", movies.ID)

	got := getTrend(t, app)
	if len(got.Trend.Categories) != 1 || !slices.Equal(got.Trend.Categories[0].Bytes, []int64{3 * unit, 5 * unit}) ||
		!slices.Equal(got.Trend.Unaccounted, []int64{0, 7 * unit}) {
		t.Errorf("trend = %+v, want Movies 3 then 5 units, Unaccounted 0 then 7", got.Trend)
	}
}
