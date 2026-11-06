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
	// film.mkv seeds in torrents and is linked into movies; each also has a
	// file of its own. show.mkv is in tv and also left in downloads, which
	// no Assignment covers; other.bin is only there.
	app.Mount.File("disk1", "data/torrents/film.mkv", 4*unit)
	app.Mount.Link("disk1", "data/torrents/film.mkv", "data/media/movies/film.mkv")
	app.Mount.File("disk1", "data/torrents/seed.iso", 2*unit)
	app.Mount.File("disk2", "data/media/movies/own.mkv", 1*unit)
	app.Mount.File("disk2", "data/downloads/show.mkv", 3*unit)
	app.Mount.Link("disk2", "data/downloads/show.mkv", "data/media/tv/show.mkv")
	app.Mount.File("cache", "data/downloads/other.bin", 1*unit)
	app.Scan(t)

	movies := create(t, app, "Movies", "#e5484d")
	torrents := create(t, app, "Torrents", "#8e4ec6")
	tv := create(t, app, "TV", "#3e63dd")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/torrents", torrents.ID)
	assign(t, app, "data/media/tv", tv.ID)

	got := getDonut(t, app).Donut
	want := []slice{{movies.ID, 5 * unit}, {torrents.ID, 2 * unit}, {tv.ID, 3 * unit}}
	if !slices.Equal(got.Categories, want) || got.Uncategorized != 1*unit {
		t.Fatalf("Movies over Torrents: donut = %+v, uncategorized %d; want %+v, uncategorized %d",
			got.Categories, got.Uncategorized, want, 1*unit)
	}

	reorder(t, app, torrents.ID, movies.ID, tv.ID)
	got = getDonut(t, app).Donut
	want = []slice{{torrents.ID, 6 * unit}, {movies.ID, 1 * unit}, {tv.ID, 3 * unit}}
	if !slices.Equal(got.Categories, want) || got.Uncategorized != 1*unit {
		t.Fatalf("Torrents over Movies: donut = %+v, uncategorized %d; want %+v, uncategorized %d",
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
	app.Mount.Link("disk1", "data/media/movies/a.mkv", "data/torrents/a.mkv")
	app.Mount.File("disk2", "data/media/movies/b.mkv", 2*unit)
	app.Mount.File("cache", "appdata/plex/db", 1*unit)
	app.Scan(t)
	movies := create(t, app, "Movies", "#e5484d")
	torrents := create(t, app, "Torrents", "#8e4ec6")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/torrents", torrents.ID)

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
