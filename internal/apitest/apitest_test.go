package apitest_test

import (
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// These check the fakes themselves, so that a test built on them fails for
// the app's reasons and not the harness's.

func TestMountFilesTakeTheirSizeInBlocks(t *testing.T) {
	t.Parallel()
	m := apitest.NewMount(t)
	m.File("disk1", "data/media/a.mkv", 3*apitest.MiB)

	var st syscall.Stat_t
	if err := syscall.Lstat(m.Path("disk1", "data/media/a.mkv"), &st); err != nil {
		t.Fatal(err)
	}
	if got := st.Blocks * 512; got != 3*apitest.MiB {
		t.Fatalf("blocks*512 = %d, want %d", got, 3*apitest.MiB)
	}
}

func TestMountLinksShareOneInode(t *testing.T) {
	t.Parallel()
	m := apitest.NewMount(t)
	m.File("disk1", "data/torrents/a.mkv", apitest.MiB)
	m.Link("disk1", "data/torrents/a.mkv", "data/media/movies/a.mkv")

	a, _ := os.Lstat(m.Path("disk1", "data/torrents/a.mkv"))
	b, _ := os.Lstat(m.Path("disk1", "data/media/movies/a.mkv"))
	if !os.SameFile(a, b) {
		t.Fatal("hardlink is not the same file")
	}
}

func TestMountRootOnlyFoldersCannotBeListed(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can list anything")
	}
	m := apitest.NewMount(t)
	m.File("cache", "appdata/db/private/x", apitest.BlockSize)
	m.RootOnly("cache", "appdata/db/private")

	if _, err := os.ReadDir(m.Path("cache", "appdata/db/private")); !os.IsPermission(err) {
		t.Fatalf("ReadDir = %v, want permission denied", err)
	}
}

func TestStatusFilesAreInEmhttpdFormat(t *testing.T) {
	t.Parallel()
	s := apitest.NewStatus(t)
	s.AddDataDisk("disk3")
	s.StartMover()

	disks := read(t, s.Dir+"/disks.ini")
	for _, want := range []string{
		"[\"disk1\"]\nname=\"disk1\"\n",
		"[\"disk3\"]\nname=\"disk3\"\nidx=\"3\"\n",
		"fsMountpoint=\"/mnt/disk3\"\n",
		"type=\"Cache\"\n",
	} {
		if !strings.Contains(disks, want) {
			t.Errorf("disks.ini lacks %q:\n%s", want, disks)
		}
	}
	if strings.Contains(disks, "DISK_NP\"") {
		t.Errorf("disk3 is still an empty slot:\n%s", disks)
	}
	if vars := read(t, s.Dir+"/var.ini"); !strings.Contains(vars, "shareMoverActive=\"yes\"\n") {
		t.Errorf("var.ini does not show the mover:\n%s", vars)
	}
}

func TestStartGivesAClockAtEpochThatMovesOnlyWhenTold(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	if !app.Clock.Now().Equal(apitest.Epoch) {
		t.Fatalf("clock = %v, want %v", app.Clock.Now(), apitest.Epoch)
	}
	if got := app.Clock.Advance(36 * time.Hour); !got.Equal(apitest.Epoch.Add(36 * time.Hour)) {
		t.Fatalf("after Advance: %v", got)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
