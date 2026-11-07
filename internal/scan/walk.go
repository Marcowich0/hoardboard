package scan

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"github.com/Marcowich0/hoardboard/internal/tree"
)

// walker walks Disks into a tree.Builder, reading metadata only.
//
// It never opens a file: the only open is of a folder, to list it, with
// O_DIRECTORY|O_NOFOLLOW so that the kernel itself refuses anything that is
// not a folder (a file, or a symlink swapped in since the stat). Every entry
// is stat'ed without following symlinks, and a symlink is recorded as the
// entry it is, never followed.
//
// It doesn't leave a Disk's filesystem: it doesn't descend into a mount point
// below the Disk's root. It does not stop where st_dev changes, because a
// btrfs subvolume (the cache pool's VM folders) has its own st_dev without
// being another filesystem.
type walker struct {
	ctx    context.Context
	b      *tree.Builder
	mounts mountPoints
	// unreadable counts folders that could not be listed; they are in the
	// tree, empty.
	unreadable int
}

// disk walks the Disk name, whose folder is root, into the builder.
func (w *walker) disk(name, root string) error {
	fd, err := openDir(unix.AT_FDCWD, root)
	if err != nil {
		return fmt.Errorf("disk folder %s: %w", root, err)
	}
	return w.folder(fd, root, w.b.Disk(name), w.b.Root())
}

// folder lists the open folder fd (at path), adds its entries to into and
// walks its subfolders. It closes fd.
func (w *walker) folder(fd int, path string, disk tree.DiskID, into *tree.Folder) error {
	dir := os.NewFile(uintptr(fd), path)
	defer dir.Close()
	if err := w.ctx.Err(); err != nil {
		return err
	}
	// Names and types only: ReadDir on a folder stats nothing.
	entries, err := dir.ReadDir(-1)
	if err != nil {
		w.unreadable++
		return nil
	}
	var sub []string
	for _, e := range entries {
		name := e.Name()
		var st unix.Stat_t
		if err := unix.Fstatat(fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			continue // gone since the listing, or unreadable: skip it
		}
		if st.Mode&unix.S_IFMT == unix.S_IFDIR {
			sub = append(sub, name)
			continue
		}
		w.b.File(into, tree.File{
			Name:  name,
			Disk:  disk,
			Dev:   st.Dev,
			Ino:   st.Ino,
			Links: uint32(st.Nlink),
			Bytes: st.Blocks * 512,
		})
	}
	for _, name := range sub {
		subPath := path + "/" + name
		if w.mounts.has(subPath) {
			continue // another filesystem is mounted here
		}
		cfd, err := openDir(fd, name)
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR) || errors.Is(err, unix.ELOOP) {
			continue // gone, or no longer a folder, since the listing
		}
		child := w.b.Folder(into, name)
		if err != nil {
			// Listed but not readable (a root-only folder without
			// CAP_DAC_READ_SEARCH): it is there, its contents are not.
			w.unreadable++
			continue
		}
		if err := w.folder(cfd, subPath, disk, child); err != nil {
			return err
		}
	}
	return nil
}

// openDir opens the folder name (relative to the folder dirfd) for listing,
// and nothing but a folder: O_DIRECTORY fails on a file, O_NOFOLLOW on a
// symlink.
func openDir(dirfd int, name string) (int, error) {
	for {
		fd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err == unix.EINTR {
			continue
		}
		return fd, err
	}
}
