package server_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// repoint is POST /api/assignments/repoint.
func repoint(t *testing.T, app *apitest.App, from, to string, wantStatus int) {
	t.Helper()
	send(t, app, http.MethodPost, "/api/assignments/repoint", map[string]string{"from": from, "to": to}, wantStatus, nil)
}

func TestRePointingMovesAMissingAssignmentToTheNewFolder(t *testing.T) {
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
	repoint(t, app, "data/media/kids", "data/media/children", http.StatusNoContent)

	got := list(t, app)
	k := named(t, got, "Kids")
	want := []assignment{{"data/media/children", ptr(2 * unit), false}, {"data/media/tv", ptr(1 * unit), false}}
	if !equalAssignments(k.Assignments, want) {
		t.Errorf("Kids assignments = %s, want %s", show(k.Assignments), show(want))
	}
	if k.Size != 3*unit || got.Uncategorized.Size != 0 {
		t.Errorf("Kids size = %d, uncategorized = %d; want %d and 0", k.Size, got.Uncategorized.Size, 3*unit)
	}
}

func TestRePointingKeepsTheCategorysPastAcrossTheRename(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02
	app.Mount.File("disk1", "data/media/kids/a.mkv", 2*unit)
	// c.mkv seeds from torrents: a hardlink with a name deep in kids.
	app.Mount.File("disk2", "data/media/kids/show/c.mkv", 1*unit)
	app.Mount.Link("disk2", "data/media/kids/show/c.mkv", "data/torrents/c.mkv")
	app.Mount.File("disk1", "data/media/tv/b.mkv", 1*unit)
	app.Scan(t)
	kids := create(t, app, "Kids", "#ffb224")
	assign(t, app, "data/media/kids", kids.ID)
	assign(t, app, "data/media/tv", kids.ID)

	app.Clock.Advance(day) // 2026-03-03
	app.Mount.File("disk1", "data/media/kids/d.mkv", 1*unit)
	app.Scan(t)
	before := []sizeOn{{"2026-03-02", 4 * unit}, {"2026-03-03", 5 * unit}}
	if got := categoryHistory(t, app, kids.ID); !slices.Equal(got, before) {
		t.Fatalf("Kids history before the rename = %+v, want %+v", got, before)
	}

	app.Clock.Advance(day) // 2026-03-04: kids is renamed
	app.Mount.Rename("disk1", "data/media/kids", "data/media/children")
	app.Mount.Rename("disk2", "data/media/kids", "data/media/children")
	app.Scan(t)
	app.Clock.Advance(day) // 2026-03-05
	app.Mount.File("disk1", "data/media/children/e.mkv", 1*unit)
	app.Scan(t)
	// The owner notices a day later.
	repoint(t, app, "data/media/kids", "data/media/children", http.StatusNoContent)

	want := []sizeOn{
		{"2026-03-02", 4 * unit},
		{"2026-03-03", 5 * unit},
		{"2026-03-04", 5 * unit},
		{"2026-03-05", 6 * unit},
	}
	if got := categoryHistory(t, app, kids.ID); !slices.Equal(got, want) {
		t.Errorf("Kids history after re-pointing = %+v, want %+v", got, want)
	}

	// The new Folder's History starts with the old one's; the old Folder's
	// own History stays on record.
	wantChildren := []point{
		{"2026-03-02", 3 * unit, 1 * unit},
		{"2026-03-03", 4 * unit, 1 * unit},
		{"2026-03-04", 4 * unit, 1 * unit},
		{"2026-03-05", 5 * unit, 1 * unit},
	}
	if got := folderHistory(t, app, "data/media/children"); !slices.Equal(got, wantChildren) {
		t.Errorf("children history = %+v, want %+v", got, wantChildren)
	}
	wantKids := []point{
		{"2026-03-02", 3 * unit, 1 * unit},
		{"2026-03-03", 4 * unit, 1 * unit},
		{"2026-03-04", 0, 0},
		{"2026-03-05", 0, 0},
	}
	if got := folderHistory(t, app, "data/media/kids"); !slices.Equal(got, wantKids) {
		t.Errorf("kids history = %+v, want %+v", got, wantKids)
	}
}

func TestRePointingRejectsWhatItCantDo(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/kids/a.mkv", 1*unit)
	app.Mount.File("disk1", "data/media/tv/b.mkv", 1*unit)
	app.Mount.File("disk1", "data/media/films/c.mkv", 1*unit)
	app.Scan(t)
	kids := create(t, app, "Kids", "#ffb224")
	assign(t, app, "data/media/kids", kids.ID)
	assign(t, app, "data/media/tv", kids.ID)

	// kids is still there: nothing to re-point.
	repoint(t, app, "data/media/kids", "data/media/films", http.StatusConflict)

	app.Mount.Rename("disk1", "data/media/kids", "data/media/children")
	app.Scan(t)
	repoint(t, app, "data/media/kids", "data/media/tv", http.StatusConflict)        // tv has its own
	repoint(t, app, "data/media/kids", "data/media/nowhere", http.StatusNotFound)   // not in the latest Scan
	repoint(t, app, "data/media/cartoons", "data/media/films", http.StatusNotFound) // no such Assignment
	repoint(t, app, "/data/media/kids", "data/media/children", http.StatusBadRequest)
	repoint(t, app, "data/media/kids", "", http.StatusBadRequest)

	// Nothing changed.
	k := named(t, list(t, app), "Kids")
	want := []assignment{{"data/media/kids", nil, true}, {"data/media/tv", ptr(1 * unit), false}}
	if !equalAssignments(k.Assignments, want) {
		t.Errorf("Kids assignments = %s, want %s", show(k.Assignments), show(want))
	}
	if got := folderHistory(t, app, "data/media/tv"); len(got) != 1 || got[0].Size != 1*unit {
		t.Errorf("tv history = %+v, want its own single point of %d", got, 1*unit)
	}
}

func TestAFolderRenamedTwiceKeepsItsWholePast(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02
	app.Mount.File("disk1", "data/media/kids/a.mkv", 2*unit)
	app.Scan(t)
	kids := create(t, app, "Kids", "#ffb224")
	assign(t, app, "data/media/kids", kids.ID)

	app.Clock.Advance(day) // 2026-03-03
	app.Mount.Rename("disk1", "data/media/kids", "data/media/children")
	app.Mount.File("disk1", "data/media/children/d.mkv", 1*unit)
	app.Scan(t)
	repoint(t, app, "data/media/kids", "data/media/children", http.StatusNoContent)

	app.Clock.Advance(day) // 2026-03-04: and back
	app.Mount.Rename("disk1", "data/media/children", "data/media/kids")
	app.Mount.File("disk1", "data/media/kids/e.mkv", 1*unit)
	app.Scan(t)
	repoint(t, app, "data/media/children", "data/media/kids", http.StatusNoContent)

	want := []sizeOn{{"2026-03-02", 2 * unit}, {"2026-03-03", 3 * unit}, {"2026-03-04", 4 * unit}}
	if got := categoryHistory(t, app, kids.ID); !slices.Equal(got, want) {
		t.Errorf("Kids history = %+v, want %+v", got, want)
	}
	wantKids := []point{
		{"2026-03-02", 2 * unit, 0},
		{"2026-03-03", 3 * unit, 0},
		{"2026-03-04", 4 * unit, 0},
	}
	if got := folderHistory(t, app, "data/media/kids"); !slices.Equal(got, wantKids) {
		t.Errorf("kids history = %+v, want %+v", got, wantKids)
	}
	wantChildren := []point{
		{"2026-03-02", 2 * unit, 0},
		{"2026-03-03", 3 * unit, 0},
		{"2026-03-04", 0, 0},
	}
	if got := folderHistory(t, app, "data/media/children"); !slices.Equal(got, wantChildren) {
		t.Errorf("children history = %+v, want %+v", got, wantChildren)
	}
}

func TestARePointedFoldersChangeIsMeasuredFromItsOldPath(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02
	app.Mount.File("disk1", "data/media/kids/a.mkv", 2*unit)
	app.Scan(t)
	kids := create(t, app, "Kids", "#ffb224")
	assign(t, app, "data/media/kids", kids.ID)

	app.Clock.Advance(2 * day) // 2026-03-04
	app.Mount.Rename("disk1", "data/media/kids", "data/media/children")
	app.Mount.File("disk1", "data/media/children/d.mkv", 1*unit)
	app.Scan(t)
	app.Clock.Advance(5 * day) // 2026-03-09, a week after the first Scan
	app.Scan(t)

	// Unlinked, children looks new: it grew by its whole Size.
	checkChanges(t, app, "data/media", "7d", map[string]*int64{".": delta(1 * unit), "children": delta(3 * unit)})
	repoint(t, app, "data/media/kids", "data/media/children", http.StatusNoContent)
	checkChanges(t, app, "data/media", "7d", map[string]*int64{".": delta(1 * unit), "children": delta(1 * unit)})
}
