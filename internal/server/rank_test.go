package server_test

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// reorder is PUT /api/categories/order: every category id, highest Rank first.
func reorder(t *testing.T, app *apitest.App, ids ...int64) {
	t.Helper()
	send(t, app, http.MethodPut, "/api/categories/order", map[string]any{"ids": ids}, http.StatusNoContent, nil)
}

// ranked is the categories' names in Rank order, each with its rank.
func ranked(got categories) []string {
	var out []string
	for _, c := range got.Categories {
		out = append(out, fmt.Sprintf("%s#%d", c.Name, c.Rank))
	}
	return out
}

func TestCategoriesCanBeReorderedAndTheRankPersists(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	movies := create(t, app, "Movies", "#e5484d")
	tv := create(t, app, "TV", "#3e63dd")
	torrents := create(t, app, "Torrents", "#8e4ec6")

	reorder(t, app, torrents.ID, movies.ID, tv.ID)

	want := []string{"Torrents#1", "Movies#2", "TV#3"}
	if got := ranked(list(t, app)); !slices.Equal(got, want) {
		t.Fatalf("after reordering = %v, want %v", got, want)
	}
	app.Restart(t)
	if got := ranked(list(t, app)); !slices.Equal(got, want) {
		t.Fatalf("after a restart = %v, want %v", got, want)
	}

	// Creating appends below; deleting closes the gap.
	create(t, app, "Music", "#30a46c")
	send(t, app, http.MethodDelete, fmt.Sprintf("/api/categories/%d", movies.ID), nil, http.StatusNoContent, nil)
	want = []string{"Torrents#1", "TV#2", "Music#3"}
	if got := ranked(list(t, app)); !slices.Equal(got, want) {
		t.Fatalf("after create and delete = %v, want %v", got, want)
	}
}

func TestAReorderMustNameEveryCategoryOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	movies := create(t, app, "Movies", "#e5484d")
	tv := create(t, app, "TV", "#3e63dd")

	for name, body := range map[string]any{
		"one missing":  map[string]any{"ids": []int64{tv.ID}},
		"twice":        map[string]any{"ids": []int64{tv.ID, tv.ID}},
		"unknown":      map[string]any{"ids": []int64{tv.ID, movies.ID, 999}},
		"unknown swap": map[string]any{"ids": []int64{tv.ID, 999}},
		"no ids":       map[string]any{},
	} {
		var got struct {
			Error string `json:"error"`
		}
		app.SendJSON(t, http.MethodPut, "/api/categories/order", body, http.StatusBadRequest, &got)
		if got.Error == "" {
			t.Errorf("%s: no error message", name)
		}
	}
	if got, want := ranked(list(t, app)), []string{"Movies#1", "TV#2"}; !slices.Equal(got, want) {
		t.Fatalf("after rejected reorders = %v, want %v unchanged", got, want)
	}
}
