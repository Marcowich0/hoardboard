package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// categories is GET /api/categories' answer (Categories in web/src/api/types.ts).
type categories struct {
	Categories    []category `json:"categories"`
	Uncategorized struct {
		Size int64 `json:"size"`
	} `json:"uncategorized"`
}

type category struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Colour      string       `json:"colour"`
	Rank        int          `json:"rank"`
	Size        int64        `json:"size"`
	SharedWith  []sharedWith `json:"sharedWith"`
	Assignments []assignment `json:"assignments"`
}

type sharedWith struct {
	CategoryID *int64 `json:"categoryId"`
	Bytes      int64  `json:"bytes"`
}

type assignment struct {
	Path    string `json:"path"`
	Size    *int64 `json:"size"`
	Missing bool   `json:"missing"`
}

func TestAFreshInstallHasNoCategoriesAndEverythingIsUncategorized(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 4*unit)
	app.Mount.File("disk2", "appdata/plex/db", 2*unit)

	var before categories
	app.GetJSON(t, "/api/categories", http.StatusOK, &before)
	if len(before.Categories) != 0 || before.Uncategorized.Size != 0 {
		t.Errorf("before the first Scan = %+v, want no categories and nothing Uncategorized", before)
	}

	app.Scan(t)
	var got categories
	app.GetJSON(t, "/api/categories", http.StatusOK, &got)
	if got.Categories == nil || len(got.Categories) != 0 {
		t.Errorf("categories = %#v, want []", got.Categories)
	}
	if got.Uncategorized.Size != 6*unit {
		t.Errorf("uncategorized = %d, want %d (everything scanned)", got.Uncategorized.Size, 6*unit)
	}
}

// send sends method path with body as JSON (nil for none) and fails the
// test unless the status is wantStatus. Answers without a body (204) are
// checked here; the rest decode into out, as with apitest's PostJSON.
func send(t *testing.T, app *apitest.App, method, path string, body any, wantStatus int, out any) {
	t.Helper()
	if wantStatus != http.StatusNoContent {
		app.SendJSON(t, method, path, body, wantStatus, out)
		return
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, path, r)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp := app.Do(t, req)
	if resp.StatusCode != wantStatus {
		got, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: status %d, want %d; body: %s", method, path, resp.StatusCode, wantStatus, got)
	}
}

// create makes a category and returns it.
func create(t *testing.T, app *apitest.App, name, colour string) category {
	t.Helper()
	var c category
	app.PostJSON(t, "/api/categories", map[string]string{"name": name, "colour": colour}, http.StatusCreated, &c)
	return c
}

// list is GET /api/categories.
func list(t *testing.T, app *apitest.App) categories {
	t.Helper()
	var got categories
	app.GetJSON(t, "/api/categories", http.StatusOK, &got)
	return got
}

// named is the category called name in got, or fails the test.
func named(t *testing.T, got categories, name string) category {
	t.Helper()
	for _, c := range got.Categories {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no category %q in %+v", name, got.Categories)
	return category{}
}

func TestCategoriesCanBeCreatedRenamedRecolouredAndDeleted(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	movies := create(t, app, "  Movies ", "#E5484D")
	if movies.ID == 0 || movies.Name != "Movies" || movies.Colour != "#e5484d" || movies.Rank != 1 {
		t.Errorf("created = %+v, want Movies, #e5484d, rank 1", movies)
	}
	if movies.Size != 0 || movies.SharedWith == nil || len(movies.SharedWith) != 0 ||
		movies.Assignments == nil || len(movies.Assignments) != 0 {
		t.Errorf("created = %#v, want empty: size 0, sharedWith [] and assignments []", movies)
	}
	tv := create(t, app, "TV", "#3e63dd")
	if tv.Rank != 2 {
		t.Errorf("second category's rank = %d, want 2 (new ones go last)", tv.Rank)
	}

	var renamed category
	app.SendJSON(t, http.MethodPatch, fmt.Sprintf("/api/categories/%d", movies.ID),
		map[string]string{"name": "Films"}, http.StatusOK, &renamed)
	if renamed.Name != "Films" || renamed.Colour != "#e5484d" {
		t.Errorf("renamed = %+v, want Films keeping #e5484d", renamed)
	}
	var recoloured category
	app.SendJSON(t, http.MethodPatch, fmt.Sprintf("/api/categories/%d", movies.ID),
		map[string]string{"colour": "#30a46c"}, http.StatusOK, &recoloured)
	if recoloured.Name != "Films" || recoloured.Colour != "#30a46c" {
		t.Errorf("recoloured = %+v, want Films in #30a46c", recoloured)
	}

	send(t, app, http.MethodDelete, fmt.Sprintf("/api/categories/%d", movies.ID), nil, http.StatusNoContent, nil)
	got := list(t, app)
	if len(got.Categories) != 1 || got.Categories[0].Name != "TV" || got.Categories[0].Rank != 1 {
		t.Errorf("after deleting Films = %+v, want TV alone at rank 1", got.Categories)
	}
	send(t, app, http.MethodDelete, fmt.Sprintf("/api/categories/%d", movies.ID), nil, http.StatusNotFound, nil)
}

func TestACategoryNeedsAUniqueNameAndAColour(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	tv := create(t, app, "TV", "#3e63dd")

	for _, c := range []struct {
		body map[string]string
		want int
	}{
		{map[string]string{"name": "", "colour": "#3e63dd"}, http.StatusBadRequest},
		{map[string]string{"name": "   ", "colour": "#3e63dd"}, http.StatusBadRequest},
		{map[string]string{"name": "Music"}, http.StatusBadRequest},
		{map[string]string{"name": "Music", "colour": "red"}, http.StatusBadRequest},
		{map[string]string{"name": "uncategorized", "colour": "#3e63dd"}, http.StatusBadRequest},
		{map[string]string{"name": "tv", "colour": "#3e63dd"}, http.StatusConflict},
	} {
		var e struct{ Error string }
		app.PostJSON(t, "/api/categories", c.body, c.want, &e)
		if e.Error == "" {
			t.Errorf("POST %v: no error message", c.body)
		}
	}
	app.SendJSON(t, http.MethodPatch, fmt.Sprintf("/api/categories/%d", tv.ID),
		map[string]string{"name": "TV"}, http.StatusOK, nil) // its own name is fine
	app.SendJSON(t, http.MethodPatch, "/api/categories/999", map[string]string{"name": "X"}, http.StatusNotFound, nil)
	app.SendJSON(t, http.MethodPatch, "/api/categories/abc", map[string]string{"name": "X"}, http.StatusBadRequest, nil)

	if got := list(t, app); len(got.Categories) != 1 {
		t.Errorf("categories = %+v, want TV only", got.Categories)
	}
}

// assign puts the folder at path into the category id.
func assign(t *testing.T, app *apitest.App, path string, id int64) {
	t.Helper()
	send(t, app, http.MethodPost, "/api/assignments", map[string]any{"path": path, "categoryId": id}, http.StatusNoContent, nil)
}

func TestFoldersFromDifferentSharesFormOneCategory(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "Photos/2024/a.jpg", 2*unit)
	app.Mount.File("disk2", "Photos/2025/b.jpg", 1*unit)
	app.Mount.File("disk1", "data/phone/c.jpg", 3*unit)
	app.Mount.File("disk2", "data/media/d.mkv", 5*unit)
	app.Scan(t)

	photos := create(t, app, "Photos", "#30a46c")
	assign(t, app, "Photos", photos.ID)
	assign(t, app, "data/phone", photos.ID)

	got := list(t, app)
	p := named(t, got, "Photos")
	if p.Size != 6*unit {
		t.Errorf("Photos size = %d, want %d", p.Size, 6*unit)
	}
	want := []assignment{{"Photos", ptr(3 * unit), false}, {"data/phone", ptr(3 * unit), false}}
	if !equalAssignments(p.Assignments, want) {
		t.Errorf("Photos assignments = %s, want %s", show(p.Assignments), show(want))
	}
	if got.Uncategorized.Size != 5*unit {
		t.Errorf("uncategorized = %d, want %d (data/media)", got.Uncategorized.Size, 5*unit)
	}
}

func TestTheMostSpecificAssignmentWins(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 4*unit)
	app.Mount.File("disk2", "data/media/movies/kids/b.mkv", 2*unit)
	app.Mount.File("disk1", "data/media/tv/c.mkv", 3*unit)
	app.Mount.File("disk1", "data/media/d.nfo", 1*unit)
	app.Mount.File("disk1", "data/torrents/e.iso", 7*unit)
	app.Scan(t)

	media := create(t, app, "Media", "#3e63dd")
	movies := create(t, app, "Movies", "#e5484d")
	kids := create(t, app, "Kids", "#ffb224")
	assign(t, app, "data/media", media.ID)
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/media/movies/kids", kids.ID)

	got := list(t, app)
	for name, want := range map[string]int64{"Media": 4 * unit, "Movies": 4 * unit, "Kids": 2 * unit} {
		if c := named(t, got, name); c.Size != want {
			t.Errorf("%s size = %d, want %d", name, c.Size, want)
		}
	}
	if got.Uncategorized.Size != 7*unit {
		t.Errorf("uncategorized = %d, want %d (data/torrents)", got.Uncategorized.Size, 7*unit)
	}
	// An Assignment's own size is its whole Folder, subfolders included.
	if a := named(t, got, "Media").Assignments; len(a) != 1 || a[0].Size == nil || *a[0].Size != 10*unit {
		t.Errorf("Media assignments = %s, want data/media at %d", show(a), 10*unit)
	}

	// Moving the middle Assignment elsewhere and removing the inner one.
	assign(t, app, "data/media/movies", kids.ID)
	send(t, app, http.MethodDelete, "/api/assignments?path=data/media/movies/kids", nil, http.StatusNoContent, nil)
	got = list(t, app)
	for name, want := range map[string]int64{"Media": 4 * unit, "Movies": 0, "Kids": 6 * unit} {
		if c := named(t, got, name); c.Size != want {
			t.Errorf("after the moves, %s size = %d, want %d", name, c.Size, want)
		}
	}

	// Deleting a category gives its folders to the Assignment above them.
	send(t, app, http.MethodDelete, fmt.Sprintf("/api/categories/%d", kids.ID), nil, http.StatusNoContent, nil)
	if c := named(t, list(t, app), "Media"); c.Size != 10*unit {
		t.Errorf("after deleting Kids, Media size = %d, want %d", c.Size, 10*unit)
	}
}

func TestAssignmentsNeedAFolderAndACategory(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", unit)
	app.Scan(t)
	media := create(t, app, "Media", "#3e63dd")

	for _, c := range []struct {
		body map[string]any
		want int
	}{
		{map[string]any{"path": "data/media", "categoryId": 999}, http.StatusNotFound},
		{map[string]any{"path": "data/nothing", "categoryId": media.ID}, http.StatusNotFound},
		{map[string]any{"path": "data/media/a.mkv", "categoryId": media.ID}, http.StatusNotFound},
		{map[string]any{"path": "", "categoryId": media.ID}, http.StatusBadRequest},
		{map[string]any{"path": "/data/media", "categoryId": media.ID}, http.StatusBadRequest},
	} {
		app.PostJSON(t, "/api/assignments", c.body, c.want, nil)
	}
	app.SendJSON(t, http.MethodDelete, "/api/assignments?path=data/media", nil, http.StatusNotFound, nil)
	if got := named(t, list(t, app), "Media"); len(got.Assignments) != 0 {
		t.Errorf("Media assignments = %s, want none", show(got.Assignments))
	}
}

func TestCategoriesAndAssignmentsSurviveARestart(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/a.mkv", 2*unit)
	app.Mount.File("disk1", "data/other/b.bin", 1*unit)
	app.Scan(t)
	media := create(t, app, "Media", "#3e63dd")
	create(t, app, "Other", "#30a46c")
	assign(t, app, "data/media", media.ID)

	app.Restart(t)
	got := list(t, app)
	if len(got.Categories) != 2 || got.Categories[0].Name != "Media" || got.Categories[1].Name != "Other" {
		t.Fatalf("categories after a restart = %+v, want Media then Other", got.Categories)
	}
	if m := got.Categories[0]; m.Size != 2*unit || len(m.Assignments) != 1 || m.Colour != "#3e63dd" {
		t.Errorf("Media after a restart = %+v, want #3e63dd, data/media, %d", m, 2*unit)
	}
	if got.Uncategorized.Size != 1*unit {
		t.Errorf("uncategorized after a restart = %d, want %d", got.Uncategorized.Size, unit)
	}
}

func ptr(n int64) *int64 { return &n }

func equalAssignments(a, b []assignment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Path != b[i].Path || a[i].Missing != b[i].Missing || (a[i].Size == nil) != (b[i].Size == nil) ||
			(a[i].Size != nil && *a[i].Size != *b[i].Size) {
			return false
		}
	}
	return true
}

// show prints assignments with their sizes rather than pointers.
func show(as []assignment) string {
	out := "["
	for _, a := range as {
		size := "null"
		if a.Size != nil {
			size = fmt.Sprint(*a.Size)
		}
		out += fmt.Sprintf(" {%s %s missing=%t}", a.Path, size, a.Missing)
	}
	return out + " ]"
}

func TestACategoryCountsAFileHardlinkedAcrossItsFoldersOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 4*unit)
	app.Mount.Link("disk1", "data/media/movies/a.mkv", "Archive/movies/a.mkv")
	app.Mount.Link("disk1", "data/media/movies/a.mkv", "Archive/again/a.mkv")
	app.Mount.File("disk1", "Archive/movies/b.mkv", 1*unit)
	app.Mount.File("disk2", "data/media/movies/c.mkv", 2*unit)
	app.Scan(t)

	movies := create(t, app, "Movies", "#e5484d")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "Archive", movies.ID)

	got := list(t, app)
	m := named(t, got, "Movies")
	if m.Size != 7*unit {
		t.Errorf("Movies size = %d, want %d (a.mkv once over its three names)", m.Size, 7*unit)
	}
	if len(m.SharedWith) != 0 {
		t.Errorf("Movies sharedWith = %+v, want none (every name is inside)", m.SharedWith)
	}
	if got.Uncategorized.Size != 0 {
		t.Errorf("uncategorized = %d, want 0", got.Uncategorized.Size)
	}
}

func TestCategoriesShowTheBytesTheyShareWithOthers(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	// One film seeding in torrents and linked into movies; one seeding only;
	// one in movies only. A show linked into tv and also left in
	// downloads, which no Assignment covers.
	app.Mount.File("disk1", "data/torrents/film.mkv", 4*unit)
	app.Mount.Link("disk1", "data/torrents/film.mkv", "data/media/movies/Film (2020)/film.mkv")
	app.Mount.File("disk1", "data/torrents/seed.iso", 2*unit)
	app.Mount.File("disk2", "data/media/movies/own.mkv", 1*unit)
	app.Mount.File("disk2", "data/downloads/show.mkv", 3*unit)
	app.Mount.Link("disk2", "data/downloads/show.mkv", "data/media/tv/show.mkv")
	app.Mount.File("disk2", "data/downloads/other.bin", 1*unit)
	app.Scan(t)

	movies := create(t, app, "Movies", "#e5484d")
	torrents := create(t, app, "Torrents", "#8e4ec6")
	tv := create(t, app, "TV", "#3e63dd")
	assign(t, app, "data/media/movies", movies.ID)
	assign(t, app, "data/torrents", torrents.ID)
	assign(t, app, "data/media/tv", tv.ID)

	got := list(t, app)
	cases := []struct {
		name   string
		size   int64
		shared []sharedWith
	}{
		{"Movies", 5 * unit, []sharedWith{{&torrents.ID, 4 * unit}}},
		{"Torrents", 6 * unit, []sharedWith{{&movies.ID, 4 * unit}}},
		{"TV", 3 * unit, []sharedWith{{nil, 3 * unit}}}, // also in Uncategorized
	}
	for _, c := range cases {
		cat := named(t, got, c.name)
		if cat.Size != c.size {
			t.Errorf("%s size = %d, want %d (Shared bytes included)", c.name, cat.Size, c.size)
		}
		if !equalShared(cat.SharedWith, c.shared) {
			t.Errorf("%s sharedWith = %s, want %s", c.name, showShared(cat.SharedWith), showShared(c.shared))
		}
	}
	// Uncategorized is the bytes no Assignment covers: show.mkv has a name
	// in tv, so only other.bin is left.
	if got.Uncategorized.Size != 1*unit {
		t.Errorf("uncategorized = %d, want %d", got.Uncategorized.Size, 1*unit)
	}
	var root tree
	app.GetJSON(t, "/api/tree", http.StatusOK, &root)
	var sum int64
	for _, c := range got.Categories {
		sum += c.Size
	}
	if overlap := int64(4 * unit); sum-overlap+got.Uncategorized.Size != root.Folder.Size {
		t.Errorf("categories (%d, %d overlapping) + uncategorized (%d) != scanned (%d)",
			sum, overlap, got.Uncategorized.Size, root.Folder.Size)
	}
}

func equalShared(a, b []sharedWith) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Bytes != b[i].Bytes || (a[i].CategoryID == nil) != (b[i].CategoryID == nil) ||
			(a[i].CategoryID != nil && *a[i].CategoryID != *b[i].CategoryID) {
			return false
		}
	}
	return true
}

func showShared(ss []sharedWith) string {
	out := "["
	for _, s := range ss {
		id := "null"
		if s.CategoryID != nil {
			id = fmt.Sprint(*s.CategoryID)
		}
		out += fmt.Sprintf(" {%s %d}", id, s.Bytes)
	}
	return out + " ]"
}

func TestAnAssignmentWhoseFolderIsGoneIsMissing(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/kids/a.mkv", 2*unit)
	app.Mount.File("disk1", "data/media/tv/b.mkv", 1*unit)
	app.Scan(t)
	kids := create(t, app, "Kids", "#ffb224")
	assign(t, app, "data/media/kids", kids.ID)
	assign(t, app, "data/media/tv", kids.ID)

	app.Mount.Rename("disk1", "data/media/kids", "data/media/children")
	app.Scan(t)

	got := list(t, app)
	k := named(t, got, "Kids")
	want := []assignment{{"data/media/kids", nil, true}, {"data/media/tv", ptr(1 * unit), false}}
	if !equalAssignments(k.Assignments, want) {
		t.Errorf("Kids assignments = %s, want %s", show(k.Assignments), show(want))
	}
	if k.Size != 1*unit || got.Uncategorized.Size != 2*unit {
		t.Errorf("Kids size = %d, uncategorized = %d; want %d and %d (children is unassigned)",
			k.Size, got.Uncategorized.Size, 1*unit, 2*unit)
	}
}
