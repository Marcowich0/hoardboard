package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// overview is the part of GET /api/overview that comes from the disks.
type overview struct {
	Today  string `json:"today"`
	Totals space  `json:"totals"`
	Disks  []struct {
		Name       string `json:"name"`
		Kind       string `json:"kind"`
		Filesystem string `json:"filesystem"`
		space
	} `json:"disks"`
	Parity []parity `json:"parity"`
}

type space struct {
	Size int64 `json:"size"`
	Used int64 `json:"used"`
	Free int64 `json:"free"`
}

type parity struct {
	Name      string  `json:"name"`
	Size      int64   `json:"size"`
	Status    string  `json:"status"`
	LastCheck *string `json:"lastCheck"`
}

func TestOverviewListsTheDataDisksAndTheCachePool(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	var got overview
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)

	want := []struct{ name, kind, fs string }{
		{"disk1", "data", "xfs"},
		{"disk2", "data", "xfs"},
		{"cache", "cache", "btrfs"},
	}
	if len(got.Disks) != len(want) {
		t.Fatalf("disks = %+v, want %v (no empty slot, no flash)", got.Disks, want)
	}
	for i, w := range want {
		d := got.Disks[i]
		if d.Name != w.name || d.Kind != w.kind || d.Filesystem != w.fs {
			t.Errorf("disk %d = %s %s %s, want %s %s %s", i, d.Name, d.Kind, d.Filesystem, w.name, w.kind, w.fs)
		}
		if d.Size <= 0 || d.Used < 0 || d.Free < 0 || d.Used+d.Free > d.Size {
			t.Errorf("%s: size %d, used %d, free %d: want used+free within size", d.Name, d.Size, d.Used, d.Free)
		}
	}
}

func TestOverviewTotalsAreTheSumOfTheDisks(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	var got overview
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)

	var sum space
	for _, d := range got.Disks {
		sum.Size += d.Size
		sum.Used += d.Used
		sum.Free += d.Free
	}
	// Every fake Disk is the same real filesystem, so the three must report
	// the same figures unless something wrote in between; the parity disk's
	// 12 TB must not be in there either way.
	if got.Totals != sum {
		t.Fatalf("totals = %+v, want the sum of the disks %+v", got.Totals, sum)
	}
	if len(got.Disks) != 3 {
		t.Fatalf("want 3 disks, got %d", len(got.Disks))
	}
}

func TestParityIsShownWithSizeAndStatusOnly(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	var got overview
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)

	// parity2 is an empty slot (DISK_NP_DSBL), so it is left out.
	want := []parity{{Name: "parity", Size: 11718885324 * 1024, Status: "valid"}}
	if len(got.Parity) != 1 || got.Parity[0].Name != want[0].Name || got.Parity[0].Size != want[0].Size ||
		got.Parity[0].Status != want[0].Status || got.Parity[0].LastCheck != nil {
		t.Fatalf("parity = %+v, want %+v with no last check", got.Parity, want)
	}
	for _, d := range got.Disks {
		if d.Name == "parity" {
			t.Fatalf("parity is listed as a disk: %+v", d)
		}
	}
}

func TestParityStatusFollowsUnraid(t *testing.T) {
	t.Parallel()
	for status, want := range map[string]string{
		"DISK_OK":         "valid",
		"DISK_DSBL":       "invalid",
		"DISK_INVALID":    "invalid",
		"DISK_NP_MISSING": "missing",
	} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			app := apitest.Start(t)
			app.Status.SetDisk("parity", "status", status)

			var got overview
			app.GetJSON(t, "/api/overview", http.StatusOK, &got)

			if len(got.Parity) != 1 || got.Parity[0].Status != want {
				t.Fatalf("parity = %+v, want status %q", got.Parity, want)
			}
		})
	}
}

func TestParityShowsTheDayTheLastCheckFinished(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	// 2026-02-20 03:15 UTC, as epoch seconds in var.ini.
	app.Status.SetVar("sbSynced2", "1771557300")

	var got overview
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)

	if len(got.Parity) != 1 || got.Parity[0].LastCheck == nil || *got.Parity[0].LastCheck != "2026-02-20" {
		t.Fatalf("parity = %+v, want last check 2026-02-20", got.Parity)
	}
}

func TestADiskUnraidNewlyListsAppearsInTheNextOverview(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.GetJSON(t, "/api/overview", http.StatusOK, nil)

	app.Mount.AddDisk("disk3")
	app.Status.AddDataDisk("disk3")
	var got overview
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)

	if names := diskNames(got); !slices.Equal(names, []string{"disk1", "disk2", "disk3", "cache"}) {
		t.Fatalf("disks = %v, want disk3 after disk2", names)
	}
}

func TestAListedDiskWithoutItsFolderIsLeftOut(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Status.AddDataDisk("disk4") // but no <mount root>/disk4

	var got overview
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)

	if names := diskNames(got); !slices.Equal(names, []string{"disk1", "disk2", "cache"}) {
		t.Fatalf("disks = %v, want disk4 left out", names)
	}
}

func TestUsedSpaceIsLiveWithoutAScan(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	var before overview
	app.GetJSON(t, "/api/overview", http.StatusOK, &before)

	app.Mount.File("disk1", "data/media/big.mkv", 32*apitest.MiB)
	var after overview
	app.GetJSON(t, "/api/overview", http.StatusOK, &after)

	// Other tests write to the same real filesystem at the same time, so only
	// a lower bound holds: at least half the new file must show.
	grew := after.Disks[0].Used - before.Disks[0].Used
	if grew < 16*apitest.MiB {
		t.Fatalf("disk1 used grew by %d bytes after writing 32 MiB, want at least 16 MiB", grew)
	}
}

func TestOverviewFailsWithoutTheDiskStatus(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	if err := os.Remove(filepath.Join(app.Status.Dir, "disks.ini")); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Error string `json:"error"`
	}
	app.GetJSON(t, "/api/overview", http.StatusInternalServerError, &got)
	if got.Error == "" {
		t.Fatal("no error message")
	}
}

func diskNames(o overview) []string {
	var names []string
	for _, d := range o.Disks {
		names = append(names, d.Name)
	}
	return names
}
