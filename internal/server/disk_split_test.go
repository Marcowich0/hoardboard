package server_test

import (
	"net/http"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// diskSplits is each Disk's split of GET /api/overview, by name.
func diskSplits(t *testing.T, app *apitest.App) map[string]struct {
	Used  int64
	Split donut
} {
	t.Helper()
	var got struct {
		Disks []struct {
			Name  string `json:"name"`
			Used  int64  `json:"used"`
			Split donut  `json:"split"`
		} `json:"disks"`
	}
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)
	out := map[string]struct {
		Used  int64
		Split donut
	}{}
	for _, d := range got.Disks {
		if d.Split.Categories == nil {
			t.Fatalf("%s split categories = null, want a list", d.Name)
		}
		out[d.Name] = struct {
			Used  int64
			Split donut
		}{d.Used, d.Split}
	}
	return out
}

func TestEachDiskSplitsItsUsedSpaceByCategory(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	// Movies are on disk1 and disk2; film.mkv seeds in torrents on disk1
	// (a hardlink stays on its Disk); tv is on disk2; the cache holds
	// something no Assignment covers.
	app.Mount.SetSpace("disk1", 1000*unit, 100*unit)
	app.Mount.SetSpace("disk2", 1000*unit, 100*unit)
	app.Mount.SetSpace("cache", 1000*unit, 100*unit)
	app.Mount.File("disk1", "data/media/movies/film.mkv", 4*unit)
	app.Mount.Link("disk1", "data/media/movies/film.mkv", "data/torrents/film.mkv")
	app.Mount.File("disk1", "data/torrents/seed.iso", 2*unit)
	app.Mount.File("disk2", "data/media/movies/other.mkv", 3*unit)
	app.Mount.File("disk2", "data/media/tv/show.mkv", 5*unit)
	app.Mount.File("cache", "appdata/plex/db", 1*unit)
	app.Scan(t)

	movies := create(t, app, "Movies", "#2a78d6")
	tv := create(t, app, "TV", "#eb6834")
	torrents := create(t, app, "Torrents", "#1baf7a")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/media/tv", tv.ID)
	assign(t, app, "data/torrents", torrents.ID)

	got := diskSplits(t, app)
	want := map[string]donut{
		// film.mkv is shared by Movies and Torrents: Movies ranks higher.
		"disk1": {Categories: []slice{{movies.ID, 4 * unit}, {tv.ID, 0}, {torrents.ID, 2 * unit}}, Uncategorized: 0, Unaccounted: 94 * unit},
		"disk2": {Categories: []slice{{movies.ID, 3 * unit}, {tv.ID, 5 * unit}, {torrents.ID, 0}}, Uncategorized: 0, Unaccounted: 92 * unit},
		"cache": {Categories: []slice{{movies.ID, 0}, {tv.ID, 0}, {torrents.ID, 0}}, Uncategorized: 1 * unit, Unaccounted: 99 * unit},
	}
	for name, w := range want {
		d, ok := got[name]
		if !ok {
			t.Errorf("no disk %s in the overview", name)
			continue
		}
		if !equalDonut(d.Split, w) {
			t.Errorf("%s split = %+v, want %+v", name, d.Split, w)
		}
		if d.Split.sum() != d.Used {
			t.Errorf("%s split adds up to %d, want its Used space %d", name, d.Split.sum(), d.Used)
		}
	}
}

func TestBeforeTheFirstScanADisksUsedSpaceIsUnaccounted(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.SetSpace("disk1", 1000*unit, 100*unit)
	for name, d := range diskSplits(t, app) {
		if d.Split.Unaccounted != d.Used || d.Split.Uncategorized != 0 {
			t.Errorf("%s split = %+v, want all of its %d Used space Unaccounted", name, d.Split, d.Used)
		}
	}
}

func equalDonut(a, b donut) bool {
	if a.Uncategorized != b.Uncategorized || a.Unaccounted != b.Unaccounted || len(a.Categories) != len(b.Categories) {
		return false
	}
	for i := range a.Categories {
		if a.Categories[i] != b.Categories[i] {
			return false
		}
	}
	return true
}
