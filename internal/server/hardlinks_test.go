package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// child is the entry of got named name, or fails the test.
func child(t *testing.T, got tree, name string) entry {
	t.Helper()
	for _, e := range got.Children {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("no child %q in %+v", name, got.Children)
	return entry{}
}

func TestAFileHardlinkedTwiceInOneFolderCountsOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 4*unit)
	app.Mount.Link("disk1", "data/media/movies/a.mkv", "data/media/movies/a copy.mkv")
	app.Mount.File("disk1", "data/media/movies/b.mkv", 1*unit)

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data/media", http.StatusOK, &got)

	movies := child(t, got, "movies")
	if movies.Size != 5*unit {
		t.Errorf("movies size = %d, want %d (a.mkv once)", movies.Size, 5*unit)
	}
	if movies.Items != 3 {
		t.Errorf("movies items = %d, want 3 (every name)", movies.Items)
	}
	if movies.SharedBytes != 0 {
		t.Errorf("movies shared bytes = %d, want 0 (both names are inside)", movies.SharedBytes)
	}
	if want := []diskBytes{{"disk1", 5 * unit}}; !equalDisks(movies.Disks, want) {
		t.Errorf("movies disks = %+v, want %+v", movies.Disks, want)
	}
	if got.Folder.Size != 5*unit {
		t.Errorf("data/media size = %d, want %d", got.Folder.Size, 5*unit)
	}

	var in tree
	app.GetJSON(t, "/api/tree?path=data/media/movies", http.StatusOK, &in)
	if in.Folder.Size != 5*unit || in.Folder.SharedBytes != 0 {
		t.Errorf("movies folder = %+v, want size %d and no shared bytes", in.Folder, 5*unit)
	}
	for _, name := range []string{"a.mkv", "a copy.mkv"} {
		if f := child(t, in, name); f.Size != 4*unit || f.Links != 2 {
			t.Errorf("%s = %+v, want size %d with 2 links", name, f, 4*unit)
		}
	}
}

func TestAFileHardlinkedAcrossTwoFoldersCountsOnceAboveThem(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/seeding/movie.mkv", 4*unit)
	app.Mount.Link("disk1", "data/seeding/movie.mkv", "data/media/movies/Movie (2020)/movie.mkv")
	app.Mount.File("disk1", "data/seeding/only-seeding.mkv", 2*unit)
	app.Mount.File("disk1", "data/media/movies/only-here.mkv", 1*unit)

	app.Scan(t)
	var data tree
	app.GetJSON(t, "/api/tree?path=data", http.StatusOK, &data)

	if data.Folder.Size != 7*unit || data.Folder.SharedBytes != 0 {
		t.Errorf("data = size %d, shared %d; want size %d (movie.mkv once), shared 0 (both names inside)",
			data.Folder.Size, data.Folder.SharedBytes, 7*unit)
	}
	if seeding := child(t, data, "seeding"); seeding.Size != 6*unit || seeding.SharedBytes != 4*unit {
		t.Errorf("seeding = size %d, shared %d; want size %d, shared %d",
			seeding.Size, seeding.SharedBytes, 6*unit, 4*unit)
	}
	if media := child(t, data, "media"); media.Size != 5*unit || media.SharedBytes != 4*unit {
		t.Errorf("media = size %d, shared %d; want size %d, shared %d",
			media.Size, media.SharedBytes, 5*unit, 4*unit)
	}

	var movies tree
	app.GetJSON(t, "/api/tree?path=data/media/movies", http.StatusOK, &movies)
	if movies.Folder.Size != 5*unit || movies.Folder.SharedBytes != 4*unit {
		t.Errorf("data/media/movies = size %d, shared %d; want size %d, shared %d",
			movies.Folder.Size, movies.Folder.SharedBytes, 5*unit, 4*unit)
	}
	if one := child(t, movies, "Movie (2020)"); one.SharedBytes != 4*unit {
		t.Errorf("Movie (2020) shared = %d, want %d", one.SharedBytes, 4*unit)
	}

	var root tree
	app.GetJSON(t, "/api/tree", http.StatusOK, &root)
	if root.Folder.Size != 7*unit || root.Folder.SharedBytes != 0 {
		t.Errorf("root = size %d, shared %d; want size %d, shared 0", root.Folder.Size, root.Folder.SharedBytes, 7*unit)
	}
}

func TestTheSameInodeOnTwoDisksIsTwoFiles(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", 4*unit)
	// Every fake Disk is a folder on one real filesystem, so a hardlink from
	// disk1 into disk2 gives two Disks the same device and inode number, as
	// two real Disks can by chance.
	other := app.Mount.Path("disk2", "data/media/b.mkv")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(app.Mount.Path("disk1", "data/media/a.mkv"), other); err != nil {
		t.Fatal(err)
	}

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data/media", http.StatusOK, &got)

	if got.Folder.Size != 8*unit {
		t.Errorf("data/media size = %d, want %d (one file per Disk)", got.Folder.Size, 8*unit)
	}
	if want := []diskBytes{{"disk1", 4 * unit}, {"disk2", 4 * unit}}; !equalDisks(got.Folder.Disks, want) {
		t.Errorf("data/media disks = %+v, want %+v", got.Folder.Disks, want)
	}
}

func TestANameTheScanCannotSeeIsOutside(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", 4*unit)
	app.Mount.File("disk1", "data/media/b.mkv", 1*unit)
	app.Mount.Link("disk1", "data/media/a.mkv", "appdata/locked/a.mkv")
	app.Mount.RootOnly("disk1", "appdata/locked")
	if canList(app.Mount.Path("disk1", "appdata/locked")) {
		t.Skip("this test runs with CAP_DAC_READ_SEARCH, so the Scan sees every name")
	}

	app.Scan(t)
	var root tree
	app.GetJSON(t, "/api/tree", http.StatusOK, &root)

	if root.Folder.Size != 5*unit || root.Folder.SharedBytes != 4*unit {
		t.Errorf("root = size %d, shared %d; want size %d, shared %d (a.mkv's other name is unseen)",
			root.Folder.Size, root.Folder.SharedBytes, 5*unit, 4*unit)
	}
	if data := child(t, root, "data"); data.SharedBytes != 4*unit {
		t.Errorf("data shared = %d, want %d", data.SharedBytes, 4*unit)
	}
}
