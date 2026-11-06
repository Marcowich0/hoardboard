package server_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// largest is GET /api/largest's answer (Largest in web/src/api/types.ts).
type largest struct {
	Path  string  `json:"path"`
	Files []entry `json:"files"`
}

// file is the part of a FileEntry these tests compare.
type file struct {
	Name  string
	Path  string
	Size  int64
	Disk  string
	Links int
}

func files(got largest) []file {
	out := make([]file, 0, len(got.Files))
	for _, f := range got.Files {
		if f.Kind != "file" {
			continue
		}
		out = append(out, file{f.Name, f.Path, f.Size, f.Disk, f.Links})
	}
	return out
}

func equalFiles(a, b []file) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLargestListsTheFilesOfTheWholeSubtreeLargestFirst(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a/a.mkv", 3*unit)
	app.Mount.File("disk2", "data/media/movies/b/b.mkv", 5*unit)
	app.Mount.File("cache", "data/media/tv/show/s01/e1.mkv", 4*unit)
	app.Mount.File("disk1", "data/media/poster.jpg", apitest.BlockSize)
	app.Mount.File("disk2", "data/torrents/big.iso", 9*unit) // outside data/media

	app.Scan(t)
	var got largest
	app.GetJSON(t, "/api/largest?path=data/media", http.StatusOK, &got)

	want := []file{
		{"b.mkv", "data/media/movies/b/b.mkv", 5 * unit, "disk2", 1},
		{"e1.mkv", "data/media/tv/show/s01/e1.mkv", 4 * unit, "cache", 1},
		{"a.mkv", "data/media/movies/a/a.mkv", 3 * unit, "disk1", 1},
		{"poster.jpg", "data/media/poster.jpg", apitest.BlockSize, "disk1", 1},
	}
	if got.Path != "data/media" || !equalFiles(files(got), want) {
		t.Fatalf("largest = %q %+v, want data/media %+v", got.Path, files(got), want)
	}
	for _, f := range got.Files {
		if f.Kind != "file" {
			t.Errorf("entry %+v: kind %q, want file", f, f.Kind)
		}
	}
}

func TestLargestListsAHardlinkedFileOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/torrents/x.mkv", 3*unit)
	app.Mount.Link("disk1", "data/torrents/x.mkv", "data/media/movies/x.mkv")
	app.Mount.Link("disk1", "data/torrents/x.mkv", "data/media/x-again.mkv")
	app.Mount.File("disk2", "data/media/y.mkv", 2*unit)

	app.Scan(t)
	var got largest
	app.GetJSON(t, "/api/largest?path=data", http.StatusOK, &got)

	// One entry per physical file, named by its first path in name order.
	want := []file{
		{"x.mkv", "data/media/movies/x.mkv", 3 * unit, "disk1", 3},
		{"y.mkv", "data/media/y.mkv", 2 * unit, "disk2", 1},
	}
	if !equalFiles(files(got), want) {
		t.Fatalf("largest files = %+v, want %+v", files(got), want)
	}

	app.GetJSON(t, "/api/largest?path=data/torrents", http.StatusOK, &got)
	want = []file{{"x.mkv", "data/torrents/x.mkv", 3 * unit, "disk1", 3}}
	if !equalFiles(files(got), want) {
		t.Fatalf("largest files in data/torrents = %+v, want %+v", files(got), want)
	}
}

func TestLargestListsOnlyTheTwentyLargest(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	for i := 1; i <= 22; i++ {
		app.Mount.File("disk1", fmt.Sprintf("data/f%02d.bin", i), i*apitest.BlockSize)
	}

	app.Scan(t)
	var got largest
	app.GetJSON(t, "/api/largest?path=data", http.StatusOK, &got)

	fs := files(got)
	if len(fs) != 20 {
		t.Fatalf("got %d files, want the 20 largest: %+v", len(fs), fs)
	}
	first := file{"f22.bin", "data/f22.bin", 22 * apitest.BlockSize, "disk1", 1}
	last := file{"f03.bin", "data/f03.bin", 3 * apitest.BlockSize, "disk1", 1}
	if fs[0] != first || fs[19] != last {
		t.Errorf("first, last = %+v, %+v; want %+v, %+v", fs[0], fs[19], first, last)
	}
}

func TestBeforeTheFirstScanThereAreNoLargestFiles(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/a.mkv", unit)

	var got largest
	app.GetJSON(t, "/api/largest?path=", http.StatusOK, &got)
	if got.Path != "" || got.Files == nil || len(got.Files) != 0 {
		t.Fatalf("largest = %+v, want the root with an empty list", got)
	}
	app.GetJSON(t, "/api/largest?path=data", http.StatusNotFound, nil)
}

func TestLargestRejectsBadRequests(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", unit)
	app.Scan(t)

	for path, status := range map[string]int{
		"/api/largest?path=data/nope":        http.StatusNotFound,
		"/api/largest?path=data/media/a.mkv": http.StatusNotFound, // a file, not a folder
		"/api/largest?path=/data":            http.StatusBadRequest,
		"/api/largest?path=data/":            http.StatusBadRequest,
		"/api/largest?path=data/../data":     http.StatusBadRequest,
	} {
		var got struct {
			Error string `json:"error"`
		}
		app.GetJSON(t, path, status, &got)
		if got.Error == "" {
			t.Errorf("GET %s: no error message", path)
		}
	}
}

func TestAfterARestartTheLargestFilesComeFromTheSavedTree(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/torrents/x.mkv", 2*unit)
	app.Mount.Link("disk1", "data/torrents/x.mkv", "data/media/x.mkv")
	app.Mount.File("disk2", "data/media/y.mkv", unit)
	app.Scan(t)

	app.Restart(t)
	var got largest
	app.GetJSON(t, "/api/largest?path=", http.StatusOK, &got)

	want := []file{
		{"x.mkv", "data/media/x.mkv", 2 * unit, "disk1", 2},
		{"y.mkv", "data/media/y.mkv", unit, "disk2", 1},
	}
	if !equalFiles(files(got), want) {
		t.Fatalf("largest after a restart = %+v, want %+v", files(got), want)
	}
}
