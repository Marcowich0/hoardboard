package server_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

func TestTorrentsHardlinksCountOnlyOutsideTorrents(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/torrents/movies/release/film.mkv", 4*unit)
	app.Mount.Link("disk1", "data/torrents/movies/release/film.mkv", "data/library/Film/film.mkv")
	app.Mount.File("disk1", "data/torrents/seed.iso", 2*unit)
	app.Mount.SetSpace("disk1", 1000*unit, 10*unit)
	app.Mount.SetSpace("disk2", 1000*unit, 0)
	app.Mount.SetSpace("cache", 1000*unit, 0)
	app.Scan(t)
	// Torrents outranks Movies: that must never give it the library's bytes.
	torrents := create(t, app, "Torrents", "#8e4ec6")
	movies := create(t, app, "Movies", "#e5484d")
	assign(t, app, "data/torrents", torrents.ID)
	assign(t, app, "data/library", movies.ID)

	check := func() {
		t.Helper()
		var got tree
		app.GetJSON(t, "/api/tree?path=data/torrents", http.StatusOK, &got)
		if got.Folder.Size != 2*unit || got.Folder.CountedElsewhereBytes != 4*unit || got.Folder.SharedBytes != 0 {
			t.Errorf("torrents = %+v, want Size 2, counted elsewhere 4 units", got.Folder)
		}
		if want := []diskBytes{{"disk1", 2 * unit}}; !equalDisks(got.Folder.Disks, want) {
			t.Errorf("disks = %+v, want %+v", got.Folder.Disks, want)
		}
		if f := child(t, got, "movies"); f.Size != 0 || f.CountedElsewhereBytes != 4*unit {
			t.Errorf("movies seeding folder = %+v", f)
		}
		app.GetJSON(t, "/api/tree?path=data/torrents/movies/release", http.StatusOK, &got)
		f := child(t, got, "film.mkv")
		if f.Size != 4*unit || f.Links != 2 || f.CountedIn == nil || f.CountedIn.Path != "data/library/Film" || f.CountedIn.CategoryID == nil || *f.CountedIn.CategoryID != movies.ID {
			t.Fatalf("visible seeding file = %+v, want original Size and Movies destination", f)
		}
		cats := list(t, app)
		if a, b := named(t, cats, "Torrents"), named(t, cats, "Movies"); a.Size != 2*unit || b.Size != 4*unit || len(a.SharedWith) != 0 || len(b.SharedWith) != 0 {
			t.Errorf("categories: Torrents %+v, Movies %+v", a, b)
		}
		want := []slice{{torrents.ID, 2 * unit}, {movies.ID, 4 * unit}}
		if d := getDonut(t, app).Donut; !slices.Equal(d.Categories, want) || d.Unaccounted != 4*unit || d.Uncategorized != 0 {
			t.Errorf("donut = %+v", d)
		}
		if d := diskSplits(t, app)["disk1"]; !slices.Equal(d.Split.Categories, want) || d.Split.Unaccounted != 4*unit {
			t.Errorf("disk1 = %+v", d)
		}
		size, tiles := getTreemap(t, app, "data/torrents/movies/release")
		if size != 0 || len(tiles) != 1 || tiles[0].Size != 0 || tiles[0].CountedElsewhereBytes != 4*unit || tiles[0].CountedIn == nil || tiles[0].CountedIn.Path != "data/library/Film" {
			t.Errorf("seeding treemap = size %d, %+v", size, tiles)
		}
		_, tiles = getTreemap(t, app, "data")
		for _, tile := range tiles {
			if tile.Name == "torrents" && (tile.Size != 2*unit || tile.CountedElsewhereBytes != 0) {
				t.Errorf("parent treemap = %+v", tile)
			}
		}
		var largest struct{ Files []entry }
		app.GetJSON(t, "/api/largest?path=data/torrents", http.StatusOK, &largest)
		if len(largest.Files) != 2 || largest.Files[0].CountedIn == nil {
			t.Errorf("largest seeding files = %+v", largest.Files)
		}
	}
	check()
	app.Restart(t) // Recompute attribution from an existing snapshot, without a Scan.
	check()

	app.Clock.Advance(day)
	app.Mount.Remove("disk1", "data/library/Film/film.mkv")
	app.Scan(t) // With no non-torrents name, the file counts toward torrents again.
	var got tree
	app.GetJSON(t, "/api/tree?path=data/torrents/movies/release", http.StatusOK, &got)
	if f := child(t, got, "film.mkv"); f.CountedIn != nil || got.Folder.Size != 4*unit || got.Folder.CountedElsewhereBytes != 0 {
		t.Errorf("after library removal = %+v, file %+v", got.Folder, f)
	}
	if got := folderHistory(t, app, "data/torrents"); !slices.Equal(got, []point{{"2026-03-02", 2 * unit, 0}, {"2026-03-03", 6 * unit, 0}}) {
		t.Errorf("folder History = %+v", got)
	}
	if got := categoryHistory(t, app, torrents.ID); !slices.Equal(got, []sizeOn{{"2026-03-02", 2 * unit}, {"2026-03-03", 6 * unit}}) {
		t.Errorf("Torrents History = %+v", got)
	}
	if got := categoryHistory(t, app, movies.ID); !slices.Equal(got, []sizeOn{{"2026-03-02", 4 * unit}, {"2026-03-03", 0}}) {
		t.Errorf("Movies History = %+v", got)
	}
	trend := getTrend(t, app).Trend
	for _, c := range trend.Categories {
		want := []int64{2 * unit, 6 * unit}
		if c.CategoryID == movies.ID {
			want = []int64{4 * unit, 0}
		}
		if !slices.Equal(c.Bytes, want) {
			t.Errorf("trend category %d = %v, want %v", c.CategoryID, c.Bytes, want)
		}
	}
}

func TestTorrentsIsAnExactFolderNameAtAnyDepth(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path     string
		excluded bool
	}{
		{"torrents", true}, {"data/torrents/nested", true}, {"data/downloads/torrents/nested", true},
		{"data/my-torrents", false}, {"data/torrents-old", false}, {"data/Torrents", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			app := apitest.Start(t)
			app.Mount.File("disk1", tc.path+"/film.mkv", 4*unit)
			app.Mount.Link("disk1", tc.path+"/film.mkv", "library/film.mkv")
			app.Scan(t)
			var got tree
			app.GetJSON(t, "/api/tree?path="+tc.path, http.StatusOK, &got)
			want := int64(4 * unit)
			if tc.excluded {
				want = 0
			}
			if got.Folder.Size != want || (child(t, got, "film.mkv").CountedIn != nil) != tc.excluded {
				t.Errorf("folder = %+v, want Size %d, excluded %v", got, want, tc.excluded)
			}
		})
	}
}

func TestSeveralTorrentsNamesCountOnlyTowardNonTorrentsNames(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/torrents/a/film.mkv", 4*unit)
	app.Mount.Link("disk1", "data/torrents/a/film.mkv", "data/torrents/b/film.mkv")
	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data/torrents", http.StatusOK, &got)
	if got.Folder.Size != 4*unit || got.Folder.CountedElsewhereBytes != 0 {
		t.Errorf("torrent-only hardlinks = %+v", got.Folder)
	}
	app.Mount.Link("disk1", "data/torrents/a/film.mkv", "data/zlibrary/film.mkv")
	app.Mount.Link("disk1", "data/torrents/a/film.mkv", "data/zzlibrary/film.mkv")
	app.Scan(t)
	app.GetJSON(t, "/api/tree?path=data/torrents", http.StatusOK, &got)
	if got.Folder.Size != 0 || got.Folder.CountedElsewhereBytes != 4*unit {
		t.Errorf("several seeding names = %+v", got.Folder)
	}
	for _, p := range []string{"data/zlibrary", "data/zzlibrary", "data"} {
		app.GetJSON(t, "/api/tree?path="+p, http.StatusOK, &got)
		if got.Folder.Size != 4*unit || got.Folder.CountedElsewhereBytes != 0 {
			t.Errorf("%s = %+v", p, got.Folder)
		}
	}
	// Prefer the library name, even though torrents sorts before zlibrary.
	var largest struct{ Files []entry }
	app.GetJSON(t, "/api/largest?path=data", http.StatusOK, &largest)
	if len(largest.Files) != 1 || largest.Files[0].Path != "data/zlibrary/film.mkv" || largest.Files[0].CountedIn != nil {
		t.Errorf("largest = %+v", largest.Files)
	}
	// An assigned torrent never claims an unassigned library's bytes.
	torrents := create(t, app, "Torrents", "#8e4ec6")
	assign(t, app, "data/torrents", torrents.ID)
	if cats := list(t, app); named(t, cats, "Torrents").Size != 0 || cats.Uncategorized.Size != 4*unit {
		t.Errorf("categories = %+v", cats)
	}
}
