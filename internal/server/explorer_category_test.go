package server_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// The Explorer's view of categories: GET /api/tree and GET /api/largest
// with the effective category of each Folder and file.

// treeCategories is the category part of GET /api/tree's answer.
type treeCategories struct {
	Folder struct {
		Category *folderCategory `json:"category"`
	} `json:"folder"`
	Children []struct {
		Kind       string `json:"kind"`
		Name       string `json:"name"`
		CategoryID *int64 `json:"categoryId"`
	} `json:"children"`
}

type folderCategory struct {
	CategoryID   int64  `json:"categoryId"`
	AssignedPath string `json:"assignedPath"`
}

func treeOf(t *testing.T, app *apitest.App, path string) treeCategories {
	t.Helper()
	var got treeCategories
	app.GetJSON(t, "/api/tree?path="+path, http.StatusOK, &got)
	return got
}

func showID(id *int64) string {
	if id == nil {
		return "null"
	}
	return fmt.Sprint(*id)
}

func showCategory(c *folderCategory) string {
	if c == nil {
		return "null"
	}
	return fmt.Sprintf("%+v", *c)
}

func TestTheExplorerShowsAFoldersCategoryAndTheAssignmentThatDecidesIt(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/heat/heat.mkv", unit)
	app.Mount.File("disk1", "data/media/tv/show.mkv", unit)
	app.Scan(t)
	media := create(t, app, "Media", "#3e63dd")
	movies := create(t, app, "Movies", "#e5484d")

	cases := []struct {
		path string
		want *folderCategory
	}{
		{"data", nil},
		{"data/media", nil},
		{"data/media/movies/heat", nil},
	}
	check := func(stage string) {
		t.Helper()
		for _, c := range cases {
			got := treeOf(t, app, c.path).Folder.Category
			if showCategory(got) != showCategory(c.want) {
				t.Errorf("%s: %s's category = %s, want %s", stage, c.path, showCategory(got), showCategory(c.want))
			}
		}
	}
	check("before any Assignment")

	assign(t, app, "data/media", media.ID)
	cases = []struct {
		path string
		want *folderCategory
	}{
		{"data", nil},
		{"data/media", &folderCategory{media.ID, "data/media"}},
		{"data/media/movies/heat", &folderCategory{media.ID, "data/media"}},
	}
	check("media assigned")

	assign(t, app, "data/media/movies", movies.ID)
	cases = []struct {
		path string
		want *folderCategory
	}{
		{"data", nil},
		{"data/media", &folderCategory{media.ID, "data/media"}},
		{"data/media/movies", &folderCategory{movies.ID, "data/media/movies"}},
		{"data/media/movies/heat", &folderCategory{movies.ID, "data/media/movies"}},
		{"data/media/tv", &folderCategory{media.ID, "data/media"}},
	}
	check("movies assigned below media (the most specific wins)")

	send(t, app, http.MethodDelete, "/api/assignments?path=data/media/movies", nil, http.StatusNoContent, nil)
	cases = []struct {
		path string
		want *folderCategory
	}{
		{"data/media/movies", &folderCategory{media.ID, "data/media"}},
		{"data/media/movies/heat", &folderCategory{media.ID, "data/media"}},
	}
	check("movies unassigned again")
}

func TestTheExplorersChildrenShowTheirEffectiveCategory(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/heat.mkv", 3*unit)
	app.Mount.File("disk1", "data/media/tv/show.mkv", 2*unit)
	app.Mount.File("disk2", "data/media/music/song.flac", 1*unit)
	app.Mount.File("disk2", "data/media/readme.txt", 1*unit)
	app.Scan(t)
	media := create(t, app, "Media", "#3e63dd")
	movies := create(t, app, "Movies", "#e5484d")

	want := func(stage string, path string, ids map[string]*int64) {
		t.Helper()
		got := treeOf(t, app, path)
		if len(got.Children) != len(ids) {
			t.Fatalf("%s: %s's children = %+v, want %d", stage, path, got.Children, len(ids))
		}
		for _, c := range got.Children {
			id, ok := ids[c.Name]
			if !ok {
				t.Errorf("%s: unexpected child %s", stage, c.Name)
				continue
			}
			if showID(c.CategoryID) != showID(id) {
				t.Errorf("%s: %s/%s (%s) categoryId = %s, want %s", stage, path, c.Name, c.Kind, showID(c.CategoryID), showID(id))
			}
		}
	}
	want("before any Assignment", "data/media",
		map[string]*int64{"movies": nil, "tv": nil, "music": nil, "readme.txt": nil})

	assign(t, app, "data/media/movies", movies.ID)
	want("movies assigned", "data/media",
		map[string]*int64{"movies": &movies.ID, "tv": nil, "music": nil, "readme.txt": nil})
	want("movies assigned", "data/media/movies", map[string]*int64{"heat.mkv": &movies.ID})

	assign(t, app, "data/media", media.ID)
	want("media assigned above movies", "data/media",
		map[string]*int64{"movies": &movies.ID, "tv": &media.ID, "music": &media.ID, "readme.txt": &media.ID})
	want("media assigned above movies", "data", map[string]*int64{"media": &media.ID})
}

func TestTheLargestFilesShowTheirEffectiveCategory(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/heat/heat.mkv", 4*unit)
	app.Mount.File("disk2", "data/media/tv/show.mkv", 3*unit)
	app.Mount.File("disk1", "data/torrents/other.iso", 2*unit)
	app.Scan(t)
	media := create(t, app, "Media", "#3e63dd")
	movies := create(t, app, "Movies", "#e5484d")
	assign(t, app, "data/media", media.ID)
	assign(t, app, "data/media/movies", movies.ID)

	var got struct {
		Files []struct {
			Path       string `json:"path"`
			CategoryID *int64 `json:"categoryId"`
		} `json:"files"`
	}
	app.GetJSON(t, "/api/largest?path=data", http.StatusOK, &got)
	want := []struct {
		path string
		id   *int64
	}{
		{"data/media/movies/heat/heat.mkv", &movies.ID},
		{"data/media/tv/show.mkv", &media.ID},
		{"data/torrents/other.iso", nil},
	}
	if len(got.Files) != len(want) {
		t.Fatalf("files = %+v, want %d", got.Files, len(want))
	}
	for i, w := range want {
		g := got.Files[i]
		if g.Path != w.path || showID(g.CategoryID) != showID(w.id) {
			t.Errorf("files[%d] = %s in %s, want %s in %s", i, g.Path, showID(g.CategoryID), w.path, showID(w.id))
		}
	}
}
