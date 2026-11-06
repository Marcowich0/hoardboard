package server_test

import (
	"net/http"
	"os"
	"slices"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// unit is the file size these tests build with: a few blocks.
const unit = 4 * apitest.BlockSize

// tree is GET /api/tree's answer (Tree in web/src/api/types.ts).
type tree struct {
	Path     string     `json:"path"`
	Period   string     `json:"period"`
	Folder   folderInfo `json:"folder"`
	Children []entry    `json:"children"`
}

type folderInfo struct {
	Path        string      `json:"path"`
	Name        string      `json:"name"`
	Size        int64       `json:"size"`
	Change      *int64      `json:"change"`
	Items       int64       `json:"items"`
	SharedBytes int64       `json:"sharedBytes"`
	Disks       []diskBytes `json:"disks"`
}

type diskBytes struct {
	Disk  string `json:"disk"`
	Bytes int64  `json:"bytes"`
}

// entry is a FolderEntry or a FileEntry.
type entry struct {
	Kind        string      `json:"kind"`
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	Size        int64       `json:"size"`
	Items       int64       `json:"items"`
	SharedBytes int64       `json:"sharedBytes"`
	Disks       []diskBytes `json:"disks"`
	Disk        string      `json:"disk"`
	Links       int         `json:"links"`
}

func TestAFolderSpreadOverDisksIsOneFolder(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 4*unit)
	app.Mount.File("disk2", "data/media/movies/b.mkv", 2*unit)
	app.Mount.File("cache", "data/media/movies/c.mkv", 1*unit)
	app.Mount.File("disk2", "data/media/movies/d.mkv", 3*unit)

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data/media", http.StatusOK, &got)

	if len(got.Children) != 1 {
		t.Fatalf("children = %+v, want the one folder movies", got.Children)
	}
	movies := got.Children[0]
	if movies.Kind != "folder" || movies.Name != "movies" || movies.Path != "data/media/movies" {
		t.Fatalf("child = %+v, want folder movies at data/media/movies", movies)
	}
	if movies.Size != 10*unit {
		t.Errorf("movies size = %d, want %d", movies.Size, 10*unit)
	}
	want := []diskBytes{{"disk2", 5 * unit}, {"disk1", 4 * unit}, {"cache", 1 * unit}}
	if !equalDisks(movies.Disks, want) {
		t.Errorf("movies disks = %+v, want %+v (largest first)", movies.Disks, want)
	}
	if got.Folder.Size != 10*unit || !equalDisks(got.Folder.Disks, want) {
		t.Errorf("data/media = %+v, want 10 MiB on %+v", got.Folder, want)
	}
}

func equalDisks(a, b []diskBytes) bool {
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

func TestAFolderListsItsSubfoldersThenItsFiles(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/small.nfo", apitest.BlockSize)
	app.Mount.File("disk2", "data/media/big.mkv", 2*unit)
	app.Mount.File("disk1", "data/media/tv/show/e1.mkv", unit)

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data/media", http.StatusOK, &got)

	want := []entry{
		{Kind: "folder", Name: "tv", Path: "data/media/tv", Size: unit, Items: 2},
		{Kind: "file", Name: "big.mkv", Path: "data/media/big.mkv", Size: 2 * unit, Disk: "disk2", Links: 1},
		{Kind: "file", Name: "small.nfo", Path: "data/media/small.nfo", Size: apitest.BlockSize, Disk: "disk1", Links: 1},
	}
	if len(got.Children) != len(want) {
		t.Fatalf("children = %+v, want %+v", got.Children, want)
	}
	for i, w := range want {
		c := got.Children[i]
		c.Disks = nil
		if c.Kind != w.Kind || c.Name != w.Name || c.Path != w.Path || c.Size != w.Size ||
			c.Items != w.Items || c.Disk != w.Disk || c.Links != w.Links {
			t.Errorf("child %d = %+v, want %+v", i, c, w)
		}
	}
	if got.Path != "data/media" || got.Period != "30d" || got.Folder.Name != "media" || got.Folder.Path != "data/media" {
		t.Errorf("tree = %q %q, folder %q at %q; want data/media, 30d, media", got.Path, got.Period, got.Folder.Name, got.Folder.Path)
	}
}

func TestItemsCountAFolderOnSeveralDisksOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", apitest.BlockSize)
	app.Mount.File("disk2", "data/media/movies/b.mkv", apitest.BlockSize)
	app.Mount.File("cache", "data/media/movies/extras/c.mkv", apitest.BlockSize)

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data", http.StatusOK, &got)

	// media, movies, extras, a.mkv, b.mkv, c.mkv
	if got.Folder.Items != 6 {
		t.Fatalf("data items = %d, want 6", got.Folder.Items)
	}
}

func TestTheRootListsEveryShare(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", 3*unit)
	app.Mount.File("cache", "appdata/plex/db", unit)
	app.Mount.File("disk2", "Photos/2025/p.jpg", 2*unit)

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree", http.StatusOK, &got)

	if got.Path != "" || got.Folder.Name != "" || got.Folder.Size != 6*unit {
		t.Errorf("root = %q, folder %+v; want path \"\", 6 MiB", got.Path, got.Folder)
	}
	var names []string
	for _, c := range got.Children {
		names = append(names, c.Kind+" "+c.Path)
	}
	if want := []string{"folder data", "folder Photos", "folder appdata"}; !slices.Equal(names, want) {
		t.Fatalf("root children = %v, want %v", names, want)
	}
}

func TestBeforeTheFirstScanTheTreeIsEmpty(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/a.mkv", unit)

	var got tree
	app.GetJSON(t, "/api/tree", http.StatusOK, &got)
	if got.Folder.Size != 0 || len(got.Children) != 0 || got.Folder.Disks == nil {
		t.Fatalf("tree = %+v, want an empty root with an empty disk list", got)
	}
	app.GetJSON(t, "/api/tree?path=data", http.StatusNotFound, nil)
}

func TestTreeRejectsBadRequests(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", unit)
	app.Scan(t)

	for path, status := range map[string]int{
		"/api/tree?path=data/nope":        http.StatusNotFound,
		"/api/tree?path=data/media/a.mkv": http.StatusNotFound, // a file, not a folder
		"/api/tree?path=/data":            http.StatusBadRequest,
		"/api/tree?path=data/":            http.StatusBadRequest,
		"/api/tree?path=data/../data":     http.StatusBadRequest,
		"/api/tree?path=data&period=2w":   http.StatusBadRequest,
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

func TestThePeriodIsEchoed(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/a.mkv", unit)
	app.Scan(t)

	var got tree
	app.GetJSON(t, "/api/tree?path=data&period=1y", http.StatusOK, &got)
	if got.Period != "1y" {
		t.Fatalf("period = %q, want 1y", got.Period)
	}
}

func TestSymlinksAreListedButNotFollowed(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 4*unit)
	// Short targets, so that no filesystem gives the link a block of its own.
	app.Mount.Symlink("disk1", "movies", "data/media/again")    // a folder
	app.Mount.Symlink("disk1", "/", "data/media/root")          // out of the Disk
	app.Mount.Symlink("disk1", "..", "data/media/movies/up")    // a loop
	app.Mount.Symlink("disk1", "movies/a.mkv", "data/media/am") // a file

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data/media", http.StatusOK, &got)

	if got.Folder.Size != 4*unit {
		t.Errorf("data/media size = %d, want %d: only a.mkv", got.Folder.Size, 4*unit)
	}
	var names []string
	for _, c := range got.Children {
		names = append(names, c.Kind+" "+c.Name)
	}
	slices.Sort(names)
	if want := []string{"file again", "file am", "file root", "folder movies"}; !slices.Equal(names, want) {
		t.Errorf("children = %v, want %v: symlinks are files", names, want)
	}
	app.GetJSON(t, "/api/tree?path=data/media/again", http.StatusNotFound, nil)
}

func TestTheScanNeverOpensAFile(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/a.mkv", unit)
	// Opening a FIFO with no writer blocks: a Scan that opened it would never
	// end, and the test would time out.
	app.Mount.Fifo("disk1", "data/pipe")

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data", http.StatusOK, &got)
	if len(got.Children) != 2 || got.Folder.Size != unit {
		t.Fatalf("data = %+v, want a.mkv and the pipe", got)
	}
}

func TestAFolderTheScanCannotListIsThereEmpty(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("cache", "appdata/db/private/x", unit)
	app.Mount.RootOnly("cache", "appdata/db/private")
	app.Mount.File("cache", "appdata/db/public", 2*unit)
	if canList(app.Mount.Path("cache", "appdata/db/private")) {
		t.Skip("this process can list root-only folders: see TestARootOnlyFolderIsCountedWithTheCapability")
	}

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=appdata/db", http.StatusOK, &got)

	if got.Folder.Size != 2*unit || len(got.Children) != 2 || got.Children[0].Name != "private" ||
		got.Children[0].Size != 0 || got.Children[1].Name != "public" {
		t.Fatalf("appdata/db = %+v, want the folder private (empty), then the file public (2 MiB)", got)
	}
}

func TestARootOnlyFolderIsCountedWithTheCapability(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("cache", "appdata/db/private/x", unit)
	app.Mount.RootOnly("cache", "appdata/db/private")
	if !canList(app.Mount.Path("cache", "appdata/db/private")) {
		t.Skip("needs CAP_DAC_READ_SEARCH, as in the container; the smoke test covers it")
	}

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=appdata/db/private", http.StatusOK, &got)
	if got.Folder.Size != unit || len(got.Children) != 1 {
		t.Fatalf("private = %+v, want x (1 MiB)", got)
	}
}

func TestAnotherScanSeesTheChanges(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", unit)
	app.Mount.File("disk2", "data/media/b.mkv", unit)
	app.Scan(t)

	app.Mount.Remove("disk1", "data/media/a.mkv")
	app.Mount.File("cache", "data/media/c.mkv", 2*unit)
	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data/media", http.StatusOK, &got)

	want := []diskBytes{{"cache", 2 * unit}, {"disk2", unit}}
	if got.Folder.Size != 3*unit || !equalDisks(got.Folder.Disks, want) {
		t.Fatalf("data/media = %+v, want 3 MiB on %+v", got.Folder, want)
	}
}

func TestAfterARestartTheSavedTreeIsServed(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", unit)
	app.Mount.File("disk2", "data/media/b.mkv", 2*unit)
	app.Mount.Link("disk2", "data/media/b.mkv", "data/media/b2.mkv")
	app.Scan(t)

	// Changes after the Scan must not show until the next one.
	app.Mount.Remove("disk1", "data/media/a.mkv")
	app.Restart(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data/media", http.StatusOK, &got)

	if got.Folder.Size != 3*unit || len(got.Children) != 3 {
		t.Fatalf("data/media = %+v, want the saved a.mkv, b.mkv and b2.mkv, b.mkv counted once", got)
	}
	want := []diskBytes{{"disk2", 2 * unit}, {"disk1", unit}}
	if !equalDisks(got.Folder.Disks, want) {
		t.Errorf("disks = %+v, want %+v", got.Folder.Disks, want)
	}
	for _, c := range got.Children {
		if c.Name == "b2.mkv" && (c.Links != 2 || c.Disk != "disk2") {
			t.Errorf("b2.mkv = %+v, want 2 links on disk2", c)
		}
	}
}

func TestADiskWithoutItsFolderIsLeftOutOfTheScan(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Status.AddDataDisk("disk4") // but no <mount root>/disk4
	app.Mount.File("disk1", "data/a.mkv", unit)

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data", http.StatusOK, &got)
	if got.Folder.Size != unit {
		t.Fatalf("data = %+v, want 1 MiB", got.Folder)
	}
}

func TestOnlyTheDisksUnraidListsAreScanned(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/a.mkv", unit)
	app.Mount.File("disk3", "data/b.mkv", 2*unit) // an empty slot in disks.ini
	app.Mount.File("user", "data/a.mkv", unit)    // Unraid's merged view
	app.Scan(t)
	var before tree
	app.GetJSON(t, "/api/tree?path=data", http.StatusOK, &before)

	app.Status.AddDataDisk("disk3")
	app.Scan(t)
	var after tree
	app.GetJSON(t, "/api/tree?path=data", http.StatusOK, &after)

	if before.Folder.Size != unit {
		t.Errorf("before disk3 is listed: data = %d bytes, want %d", before.Folder.Size, unit)
	}
	want := []diskBytes{{"disk3", 2 * unit}, {"disk1", unit}}
	if after.Folder.Size != 3*unit || !equalDisks(after.Folder.Disks, want) {
		t.Errorf("after disk3 is listed: data = %+v, want 3 MiB on %+v", after.Folder, want)
	}
}

// canList is whether this process can list the folder at path.
func canList(path string) bool {
	_, err := os.ReadDir(path)
	return err == nil
}

func TestTheScanDoesNotCrossAMountPoint(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", unit)
	app.Mount.File("disk1", "data/old backup/b.img", 2*unit)
	app.Mount.MountPoint("disk1", "data/old backup")

	app.Scan(t)
	var got tree
	app.GetJSON(t, "/api/tree?path=data", http.StatusOK, &got)

	if got.Folder.Size != unit || len(got.Children) != 1 || got.Children[0].Name != "media" {
		t.Fatalf("data = %+v, want only media: \"data/old backup\" is another filesystem", got)
	}
}
