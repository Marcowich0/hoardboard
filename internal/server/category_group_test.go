package server_test

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// grouped is the categories in Rank order, each as "Name#rank" with
// "<Group" when it is in a group.
func grouped(got categories) []string {
	names := map[int64]string{}
	for _, c := range got.Categories {
		names[c.ID] = c.Name
	}
	var out []string
	for _, c := range got.Categories {
		s := fmt.Sprintf("%s#%d", c.Name, c.Rank)
		if c.GroupID != nil {
			s += "<" + names[*c.GroupID]
		}
		out = append(out, s)
	}
	return out
}

// moveInto is PATCH /api/categories/{id} with groupId (nil: the top level).
func moveInto(t *testing.T, app *apitest.App, id int64, group *int64, wantStatus int) {
	t.Helper()
	var out map[string]any
	app.SendJSON(t, http.MethodPatch, fmt.Sprintf("/api/categories/%d", id), map[string]any{"groupId": group}, wantStatus, &out)
}

func TestCategoriesGoIntoAGroupAndFollowItInRank(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	media := create(t, app, "Media", "#2a78d6")
	backups := create(t, app, "Backups", "#eb6834")
	movies := create(t, app, "Movies", "#1baf7a")
	tv := create(t, app, "TV", "#eda100")

	moveInto(t, app, movies.ID, &media.ID, http.StatusOK)
	moveInto(t, app, tv.ID, &media.ID, http.StatusOK)
	want := []string{"Media#1", "Movies#2<Media", "TV#3<Media", "Backups#4"}
	if got := grouped(list(t, app)); !slices.Equal(got, want) {
		t.Fatalf("after grouping = %v, want %v", got, want)
	}

	// A new category can start in a group: last in it.
	var music category
	app.PostJSON(t, "/api/categories", map[string]any{"name": "Music", "colour": "#e87ba4", "groupId": media.ID}, http.StatusCreated, &music)
	if music.GroupID == nil || *music.GroupID != media.ID {
		t.Errorf("created Music groupId = %v, want %d", music.GroupID, media.ID)
	}

	// A reorder keeps a group together, under it.
	reorder(t, app, music.ID, backups.ID, tv.ID, media.ID, movies.ID)
	want = []string{"Backups#1", "Media#2", "Music#3<Media", "TV#4<Media", "Movies#5<Media"}
	if got := grouped(list(t, app)); !slices.Equal(got, want) {
		t.Fatalf("after a reorder = %v, want %v", got, want)
	}
	app.Restart(t)
	if got := grouped(list(t, app)); !slices.Equal(got, want) {
		t.Fatalf("after a restart = %v, want %v", got, want)
	}

	// Out of the group: to the end of the top level.
	moveInto(t, app, tv.ID, nil, http.StatusOK)
	want = []string{"Backups#1", "Media#2", "Music#3<Media", "Movies#4<Media", "TV#5"}
	if got := grouped(list(t, app)); !slices.Equal(got, want) {
		t.Fatalf("after leaving the group = %v, want %v", got, want)
	}

	// Deleting the group moves its categories to the top level, where it was.
	send(t, app, http.MethodDelete, fmt.Sprintf("/api/categories/%d", media.ID), nil, http.StatusNoContent, nil)
	want = []string{"Backups#1", "Music#2", "Movies#3", "TV#4"}
	if got := grouped(list(t, app)); !slices.Equal(got, want) {
		t.Fatalf("after deleting the group = %v, want %v", got, want)
	}
}

func TestGroupsAreOneLevelDeep(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	media := create(t, app, "Media", "#2a78d6")
	movies := create(t, app, "Movies", "#1baf7a")
	hd := create(t, app, "HD", "#eda100")
	moveInto(t, app, movies.ID, &media.ID, http.StatusOK)

	moveInto(t, app, hd.ID, &movies.ID, http.StatusBadRequest)   // Movies is in a group
	moveInto(t, app, media.ID, &hd.ID, http.StatusBadRequest)    // Media has a category in it
	moveInto(t, app, media.ID, &media.ID, http.StatusBadRequest) // itself
	moveInto(t, app, hd.ID, ptr(999), http.StatusBadRequest)     // no such category
	moveInto(t, app, hd.ID, ptr(-1), http.StatusBadRequest)      // not an id
	want := []string{"Media#1", "Movies#2<Media", "HD#3"}
	if got := grouped(list(t, app)); !slices.Equal(got, want) {
		t.Fatalf("after refused moves = %v, want %v", got, want)
	}
}

func TestAGroupsSizeAndHistoryCoverTheCategoriesInIt(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02
	// film.mkv is in movies and seeds in seeding; ep.mkv is in tv and
	// linked into movies too (both in Media: once); extras is Media's own.
	app.Mount.File("disk1", "data/media/movies/film.mkv", 4*unit)
	app.Mount.Link("disk1", "data/media/movies/film.mkv", "data/seeding/film.mkv")
	app.Mount.File("disk2", "data/media/tv/ep.mkv", 2*unit)
	app.Mount.Link("disk2", "data/media/tv/ep.mkv", "data/media/movies/ep.mkv")
	app.Mount.File("disk2", "data/media/extras/x.bin", 1*unit)
	app.Scan(t)
	app.Clock.Advance(day) // 2026-03-03
	app.Mount.File("disk1", "data/media/tv/ep2.mkv", 3*unit)
	app.Scan(t)

	media := create(t, app, "Media", "#2a78d6")
	movies := create(t, app, "Movies", "#1baf7a")
	tv := create(t, app, "TV", "#eda100")
	seeding := create(t, app, "Seeding", "#6250d6")
	assign(t, app, "data/media", media.ID)
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/media/tv", tv.ID)
	assign(t, app, "data/seeding", seeding.ID)
	moveInto(t, app, movies.ID, &media.ID, http.StatusOK)
	moveInto(t, app, tv.ID, &media.ID, http.StatusOK)

	got := list(t, app)
	if g := named(t, got, "Media"); g.Size != 10*unit {
		t.Errorf("Media size = %d, want %d (its own, Movies' and TV's, each file once)", g.Size, 10*unit)
	} else if want := []sharedWith{{&seeding.ID, 4 * unit}}; !equalShared(g.SharedWith, want) {
		t.Errorf("Media shares %s, want %s (nothing with its own categories)", showShared(g.SharedWith), showShared(want))
	}
	if m := named(t, got, "Movies"); m.Size != 6*unit {
		t.Errorf("Movies size = %d, want %d (a category in a group is measured alone)", m.Size, 6*unit)
	}

	wantHistory := []sizeOn{{"2026-03-02", 7 * unit}, {"2026-03-03", 10 * unit}}
	if h := categoryHistory(t, app, media.ID); !slices.Equal(h, wantHistory) {
		t.Errorf("Media history = %+v, want %+v", h, wantHistory)
	}
}
