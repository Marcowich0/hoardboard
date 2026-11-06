// Package tree is the merged tree of Share paths that a Scan produces: every
// Folder and file the Disks hold, with each Folder's Size, item count and
// bytes per Disk.
//
// A Tree is built once, with a Builder, and never changes after Finish, so
// any number of goroutines can read it while the next Scan builds another.
//
// The Builder is the merger: walking disk1 and then disk2 into the same
// Builder makes data/media one Folder, whichever Disks hold parts of it.
package tree

import (
	"cmp"
	"slices"
	"strings"
)

// DiskID indexes Tree.Disks.
type DiskID uint16

// File is one name of a file (or a symlink, socket, …: anything that is not
// a folder) on one Disk. A hardlinked file has one File per name.
type File struct {
	Name string
	Disk DiskID
	// Dev and Ino identify the physical file within its Disk; with Disk, they
	// are its hardlink identity. Dev differs between btrfs subvolumes.
	Dev, Ino uint64
	// Links is how many names the file has (st_nlink).
	Links uint32
	// Bytes is the space the file occupies: its allocated blocks
	// (st_blocks*512), not its apparent size, so a sparse file counts what
	// it really takes.
	Bytes int64
}

// Folder is one Folder of the merged tree, or the root ("").
type Folder struct {
	Name   string
	Parent *Folder // nil for the root
	// Folders and Files are sorted by name.
	Folders []*Folder
	Files   []File

	// Computed by Finish.

	// Size is the bytes of every file below it. Folders' own blocks are not
	// in it: they are filesystem overhead, Unaccounted.
	Size int64
	// Items is how many folders and files are below it, at any depth.
	Items int64
	// DiskBytes is Size split by Disk, indexed by DiskID.
	DiskBytes []int64

	index map[string]*Folder // while building only
}

// Tree is a finished merged tree.
type Tree struct {
	// Disks names the Disks the Scan walked ("disk1", "cache"), by DiskID.
	Disks []string
	Root  *Folder
}

// Path is the Folder's Share path: "data/media/movies", "" for the root.
func (f *Folder) Path() string {
	if f.Parent == nil {
		return ""
	}
	var names []string
	for g := f; g.Parent != nil; g = g.Parent {
		names = append(names, g.Name)
	}
	slices.Reverse(names)
	return strings.Join(names, "/")
}

// Lookup finds the Folder at path ("" is the root, "data/media" a Folder).
func (t *Tree) Lookup(path string) (*Folder, bool) {
	f := t.Root
	if path == "" {
		return f, true
	}
	for name := range strings.SplitSeq(path, "/") {
		i, ok := slices.BinarySearchFunc(f.Folders, name, func(c *Folder, n string) int { return strings.Compare(c.Name, n) })
		if !ok {
			return nil, false
		}
		f = f.Folders[i]
	}
	return f, true
}

// Builder builds a Tree, one Disk after another. It is not safe for
// concurrent use.
type Builder struct {
	t Tree
}

// NewBuilder starts an empty Tree.
func NewBuilder() *Builder {
	return &Builder{t: Tree{Root: &Folder{index: map[string]*Folder{}}}}
}

// Disk adds a Disk by name and returns its id.
func (b *Builder) Disk(name string) DiskID {
	b.t.Disks = append(b.t.Disks, name)
	return DiskID(len(b.t.Disks) - 1)
}

// Root is the root folder, which holds the Shares.
func (b *Builder) Root() *Folder { return b.t.Root }

// Folder returns parent's subfolder name, adding it unless an earlier Disk
// already did: this is where Folders merge across Disks.
func (b *Builder) Folder(parent *Folder, name string) *Folder {
	if f, ok := parent.index[name]; ok {
		return f
	}
	f := &Folder{Name: name, Parent: parent, index: map[string]*Folder{}}
	parent.index[name] = f
	parent.Folders = append(parent.Folders, f)
	return f
}

// File adds a file to folder.
func (b *Builder) File(folder *Folder, f File) {
	folder.Files = append(folder.Files, f)
}

// Finish sorts the tree and computes every Folder's Size, Items and
// DiskBytes. The Builder must not be used afterwards.
func (b *Builder) Finish() *Tree {
	t := b.t
	b.t = Tree{}
	finish(t.Root, len(t.Disks))
	return &t
}

func finish(f *Folder, disks int) {
	f.index = nil
	slices.SortFunc(f.Folders, func(a, b *Folder) int { return strings.Compare(a.Name, b.Name) })
	slices.SortStableFunc(f.Files, func(a, b File) int { return cmp.Compare(a.Name, b.Name) })
	f.DiskBytes = make([]int64, disks)
	for i := range f.Files {
		file := &f.Files[i]
		f.Size += file.Bytes
		f.DiskBytes[file.Disk] += file.Bytes
	}
	f.Items = int64(len(f.Files) + len(f.Folders))
	for _, c := range f.Folders {
		finish(c, disks)
		f.Size += c.Size
		f.Items += c.Items
		for d, n := range c.DiskBytes {
			f.DiskBytes[d] += n
		}
	}
}
