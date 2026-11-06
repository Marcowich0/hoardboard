package server_test

import (
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// sizeOn is one point of GET /api/history?category= (History in
// web/src/api/types.ts).
type sizeOn struct {
	Day  string `json:"day"`
	Size int64  `json:"size"`
}

func categoryHistory(t *testing.T, app *apitest.App, id int64) []sizeOn {
	t.Helper()
	var got struct {
		CategoryID int64    `json:"categoryId"`
		Points     []sizeOn `json:"points"`
	}
	app.GetJSON(t, "/api/history?category="+strconv.FormatInt(id, 10), http.StatusOK, &got)
	if got.CategoryID != id {
		t.Errorf("history categoryId = %d, want %d", got.CategoryID, id)
	}
	if got.Points == nil {
		t.Fatalf("history points = null, want a list")
	}
	return got.Points
}

func TestACategorysHistoryCountsEachFileOncePerDay(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02
	// a.mkv is in movies and seeds in torrents, outside the category.
	app.Mount.File("disk1", "data/media/movies/a.mkv", 2*unit)
	app.Mount.Link("disk1", "data/media/movies/a.mkv", "data/torrents/a.mkv")
	app.Mount.File("disk2", "data/media/tv/s.mkv", 1*unit)
	app.Scan(t)

	// b.mkv has a name in both of the category's Folders: once.
	app.Clock.Advance(day) // 2026-03-03
	app.Mount.File("disk1", "data/media/tv/b.mkv", 4*unit)
	app.Mount.Link("disk1", "data/media/tv/b.mkv", "data/media/movies/b.mkv")
	app.Scan(t)

	// a.mkv leaves the category; its torrent stays.
	app.Clock.Advance(day) // 2026-03-04
	app.Mount.Remove("disk1", "data/media/movies/a.mkv")
	app.Scan(t)

	app.Clock.Advance(day) // 2026-03-05, no Scan: carried forward

	// The category is made after the Scans: its past comes from History.
	media := create(t, app, "Media", "#e5484d")
	assign(t, app, "data/media/movies", media.ID)
	assign(t, app, "data/media/tv", media.ID)

	want := []sizeOn{
		{"2026-03-02", 3 * unit},
		{"2026-03-03", 7 * unit},
		{"2026-03-04", 5 * unit},
		{"2026-03-05", 5 * unit},
	}
	if got := categoryHistory(t, app, media.ID); !slices.Equal(got, want) {
		t.Errorf("Media history = %+v, want %+v", got, want)
	}
}

func TestAnUnknownCategoryHasNoHistory(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.GetJSON(t, "/api/history?category=42", http.StatusNotFound, nil)
	app.GetJSON(t, "/api/history?category=movies", http.StatusBadRequest, nil)
	app.GetJSON(t, "/api/history?category=1&path=data", http.StatusBadRequest, nil)
}

func TestReassigningAFolderRecalculatesTheCategorysPast(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 2*unit)
	app.Mount.File("disk2", "data/media/tv/s.mkv", 1*unit)
	app.Scan(t) // 2026-03-02
	movies := create(t, app, "Movies", "#e5484d")
	tv := create(t, app, "TV", "#3e63dd")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/media/tv", tv.ID)
	app.Clock.Advance(day)
	app.Mount.File("disk2", "data/media/tv/t.mkv", 4*unit)
	app.Scan(t) // 2026-03-03

	want := []sizeOn{{"2026-03-02", 2 * unit}, {"2026-03-03", 2 * unit}}
	if got := categoryHistory(t, app, movies.ID); !slices.Equal(got, want) {
		t.Fatalf("Movies before = %+v, want %+v", got, want)
	}

	// tv moves into Movies: Movies had it all along, TV never had anything.
	assign(t, app, "data/media/tv", movies.ID)
	want = []sizeOn{{"2026-03-02", 3 * unit}, {"2026-03-03", 7 * unit}}
	if got := categoryHistory(t, app, movies.ID); !slices.Equal(got, want) {
		t.Errorf("Movies with tv = %+v, want %+v", got, want)
	}
	if got := categoryHistory(t, app, tv.ID); !slices.Equal(got, []sizeOn{{"2026-03-02", 0}, {"2026-03-03", 0}}) {
		t.Errorf("TV without tv = %+v, want zeros", got)
	}

	// A broader Assignment takes the rest of media; Movies' own stays.
	media := create(t, app, "Media", "#30a46c")
	assign(t, app, "data/media", media.ID)
	send(t, app, http.MethodDelete, "/api/assignments?path=data/media/tv", nil, http.StatusNoContent, nil)
	want = []sizeOn{{"2026-03-02", 1 * unit}, {"2026-03-03", 5 * unit}}
	if got := categoryHistory(t, app, media.ID); !slices.Equal(got, want) {
		t.Errorf("Media = %+v, want %+v", got, want)
	}
}
