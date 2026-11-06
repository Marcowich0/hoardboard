package apitest

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// Sizes for Mount.File. Use whole multiples of BlockSize, so that a file's
// Size (the blocks it occupies) equals the bytes written on any filesystem the
// tests run on.
const (
	BlockSize = 4096
	KiB       = 1024
	MiB       = 1024 * KiB
)

// DefaultDisks are the Disks a NewMount starts with.
var DefaultDisks = []string{"disk1", "disk2", "cache"}

// Mount is a fake Unraid mount root (/mnt): one folder per Disk, each holding
// Share trees. Every Disk lives on the same real filesystem (the test's temp
// folder), so tests can assert that totals are consistent but not exact free
// space.
//
// Paths given to its methods are relative to the Disk, so "data/media/a.mkv"
// on "disk1" is <Root>/disk1/data/media/a.mkv, the Folder data/media of the
// merged tree. Parent folders are created as needed.
//
// It also fakes the kernel's list of mount points (the server's mountinfo
// file), which says where a Scan must stop: see MountPoint.
type Mount struct {
	Root string
	t    testing.TB

	mu        sync.Mutex
	mountInfo string   // the fake mountinfo file, outside Root
	mounts    []string // its mount points, absolute
}

// NewMount makes an empty mount root with the DefaultDisks.
func NewMount(t testing.TB) *Mount {
	t.Helper()
	m := &Mount{Root: t.TempDir(), t: t}
	m.mountInfo = filepath.Join(t.TempDir(), "mountinfo")
	// As on Unraid, every Disk is a filesystem of its own, and so is /.
	m.mounts = []string{"/"}
	for _, d := range DefaultDisks {
		m.AddDisk(d)
		m.mounts = append(m.mounts, m.realPath(d, ""))
	}
	m.writeMountInfo()
	return m
}

// MountPoint makes the folder rel on disk a mount point: another filesystem
// (a backup drive, say) is mounted there, so a Scan must not go in.
func (m *Mount) MountPoint(disk, rel string) {
	m.t.Helper()
	m.Dir(disk, rel)
	m.mu.Lock()
	m.mounts = append(m.mounts, m.realPath(disk, rel))
	m.mu.Unlock()
	m.writeMountInfo()
}

// HoldScans makes every Scan wait at its start, as on a very slow disk, until
// release is called (at the latest when the test ends). Meanwhile a test can
// see a Scan running, join it, or move the clock. Don't call MountPoint
// while scans are held.
func (m *Mount) HoldScans() (release func()) {
	m.t.Helper()
	// The server reads the mountinfo file when a Scan starts. Opening a
	// FIFO for reading blocks until something writes to it.
	m.must(os.Remove(m.mountInfo))
	m.must(syscall.Mkfifo(m.mountInfo, 0o600))
	var once sync.Once
	release = func() {
		once.Do(func() {
			// O_RDWR never blocks on a FIFO, whether a Scan waits or not.
			f, err := os.OpenFile(m.mountInfo, os.O_RDWR, 0)
			m.must(err)
			defer f.Close()
			// From now on Scans read the plain file; one that already opened
			// the FIFO reads the same lines from it.
			m.writeMountInfo()
			_, err = f.WriteString(m.mountInfoText())
			m.must(err)
		})
	}
	m.t.Cleanup(release)
	return release
}

// realPath is the absolute path of rel on disk, symlinks resolved, as the
// kernel lists mount points.
func (m *Mount) realPath(disk, rel string) string {
	m.t.Helper()
	root, err := filepath.EvalSymlinks(m.Root)
	m.must(err)
	return filepath.Join(root, disk, filepath.FromSlash(rel))
}

func (m *Mount) mountInfoText() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var b strings.Builder
	for i, p := range m.mounts {
		// proc_pid_mountinfo(5): space, tab, newline and backslash are octal.
		esc := strings.NewReplacer(`\`, `\134`, " ", `\040`, "\t", `\011`, "\n", `\012`).Replace(p)
		fmt.Fprintf(&b, "%d 1 0:%d / %s ro,relatime shared:%d - xfs /dev/md%dp1 ro\n", 100+i, 30+i, esc, i+1, i+1)
	}
	return b.String()
}

// writeMountInfo replaces the mountinfo file in one step.
func (m *Mount) writeMountInfo() {
	m.t.Helper()
	tmp := m.mountInfo + ".new"
	m.must(os.WriteFile(tmp, []byte(m.mountInfoText()), 0o644))
	m.must(os.Rename(tmp, m.mountInfo))
}

// Path is the absolute path of rel on disk.
func (m *Mount) Path(disk, rel string) string {
	return filepath.Join(m.Root, disk, filepath.FromSlash(rel))
}

// AddDisk makes the empty folder of a new Disk. Unraid lists it only once the
// Status says so: see Status.AddDataDisk.
func (m *Mount) AddDisk(disk string) {
	m.t.Helper()
	m.must(os.MkdirAll(filepath.Join(m.Root, disk), 0o755))
}

// Dir makes the folder rel on disk.
func (m *Mount) Dir(disk, rel string) {
	m.t.Helper()
	m.must(os.MkdirAll(m.Path(disk, rel), 0o755))
}

// File writes a file of size bytes at rel on disk. The bytes are random, so
// that no filesystem stores the file in fewer blocks by compressing it, and
// nothing is sparse.
func (m *Mount) File(disk, rel string, size int) {
	m.t.Helper()
	p := m.Path(disk, rel)
	m.must(os.MkdirAll(filepath.Dir(p), 0o755))
	buf := make([]byte, size)
	r := rand.NewChaCha8([32]byte{})
	_, _ = r.Read(buf)
	m.must(os.WriteFile(p, buf, 0o644))
}

// Link makes rel on disk a hardlink of the existing file there. Hardlinks
// never cross Disks, as on Unraid.
func (m *Mount) Link(disk, existing, rel string) {
	m.t.Helper()
	p := m.Path(disk, rel)
	m.must(os.MkdirAll(filepath.Dir(p), 0o755))
	m.must(os.Link(m.Path(disk, existing), p))
}

// Symlink makes rel on disk a symbolic link to target, which is written as
// given (relative or absolute).
func (m *Mount) Symlink(disk, target, rel string) {
	m.t.Helper()
	p := m.Path(disk, rel)
	m.must(os.MkdirAll(filepath.Dir(p), 0o755))
	m.must(os.Symlink(target, p))
}

// RootOnly takes every permission off the folder rel on disk, as with a
// root-only appdata folder: a process without CAP_DAC_READ_SEARCH (an ordinary
// test run) can no longer list it. The permissions come back at cleanup, so
// the temp folder can be removed.
func (m *Mount) RootOnly(disk, rel string) {
	m.t.Helper()
	p := m.Path(disk, rel)
	m.must(os.MkdirAll(p, 0o755))
	m.must(os.Chmod(p, 0))
	m.t.Cleanup(func() { _ = os.Chmod(p, 0o755) })
}

// Remove deletes rel on disk and everything under it.
func (m *Mount) Remove(disk, rel string) {
	m.t.Helper()
	m.must(os.RemoveAll(m.Path(disk, rel)))
}

// Rename moves rel on disk to newRel on the same disk.
func (m *Mount) Rename(disk, rel, newRel string) {
	m.t.Helper()
	p := m.Path(disk, newRel)
	m.must(os.MkdirAll(filepath.Dir(p), 0o755))
	m.must(os.Rename(m.Path(disk, rel), p))
}

func (m *Mount) must(err error) {
	m.t.Helper()
	if err != nil {
		m.t.Fatal(err)
	}
}

// Fifo makes a named pipe at rel on disk. Opening it for reading blocks
// until something writes, so a Scan that opened files would never end.
func (m *Mount) Fifo(disk, rel string) {
	m.t.Helper()
	p := m.Path(disk, rel)
	m.must(os.MkdirAll(filepath.Dir(p), 0o755))
	m.must(syscall.Mkfifo(p, 0o644))
}
