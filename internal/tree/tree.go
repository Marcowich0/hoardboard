// Package tree is the merged tree of Share paths that a Scan produces: every
// Folder and file the Disks hold, with each Folder's Size, item count, bytes
// per Disk and Shared bytes.
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
	// CountedIn is the non-torrents Folder that counts this name's bytes,
	// when this is a torrents hardlink. Computed by Finish, not stored.
	CountedIn string
}

// Folder is one Folder of the merged tree, or the root ("").
type Folder struct {
	Name   string
	Parent *Folder // nil for the root
	// Folders and Files are sorted by name.
	Folders []*Folder
	Files   []File

	// Computed by Finish.

	// Size is the bytes of every file below it, each physical file once
	// however many of its counted names are below it. Folders' own
	// blocks are not in it: they are filesystem overhead, Unaccounted.
	Size int64
	// SingleBytes is the part of Size in files with one name (st_nlink 1).
	// The rest of Size is the Tree's Hardlinks with a name below it, each
	// once. Unlike Size, SingleBytes adds up: a Folder's is its own files'
	// plus its subfolders'.
	SingleBytes int64
	// Items is how many folders and files are below it, at any depth.
	Items int64
	// DiskBytes is Size split by Disk, indexed by DiskID.
	DiskBytes []int64
	// SharedBytes is the part of Size in files that also have a name
	// outside this Folder.
	SharedBytes int64
	// CountedElsewhereBytes are files visible below this Folder whose bytes
	// count only toward a non-torrents Folder outside it, each file once.
	CountedElsewhereBytes int64

	index map[string]*Folder // while building only
}

// Tree is a finished merged tree.
type Tree struct {
	// Disks names the Disks the Scan walked ("disk1", "cache"), by DiskID.
	Disks []string
	Root  *Folder
	// Hardlinks are the files with more than one name, in tree order.
	Hardlinks []Hardlink
}

// Hardlink is one physical file with more than one name (st_nlink > 1).
// Its identity is (Disk, Dev, Ino): the same inode number on two Disks, or
// on two btrfs subvolumes of one Disk, is two files.
type Hardlink struct {
	Disk     DiskID
	Dev, Ino uint64
	Bytes    int64
	// Links is how many names it has: st_nlink, or how many the Scan saw if
	// that is more (it changed during the Scan). Names the Scan did not see,
	// in a folder it could not list or outside the Shares, count too.
	Links int
	// Folders holds the Folder of each name the Scan saw, in tree order; a
	// Folder holding two of its names is in it twice.
	Folders []*Folder
	// CountedFolders excludes torrents names when a non-torrents name exists.
	CountedFolders []*Folder
	files          []*File
}

type identity struct {
	disk     DiskID
	dev, ino uint64
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

// Finish sorts the tree and computes every Folder's Size, SingleBytes,
// Items, DiskBytes and Shared bytes, and the Tree's Hardlinks. The Builder
// must not be used afterwards.
func (b *Builder) Finish() *Tree {
	t := b.t
	b.t = Tree{}
	index := map[identity]int{}
	finish(t.Root, len(t.Disks), &t.Hardlinks, index)
	addHardlinks(t.Hardlinks)
	return &t
}

// finish sorts f's subtree and sums its files with one name; it collects
// the files with several into links, by identity through index.
func finish(f *Folder, disks int, links *[]Hardlink, index map[identity]int) {
	f.index = nil
	slices.SortFunc(f.Folders, func(a, b *Folder) int { return strings.Compare(a.Name, b.Name) })
	slices.SortStableFunc(f.Files, func(a, b File) int { return cmp.Compare(a.Name, b.Name) })
	f.DiskBytes = make([]int64, disks)
	for i := range f.Files {
		file := &f.Files[i]
		if file.Links > 1 {
			id := identity{file.Disk, file.Dev, file.Ino}
			n, ok := index[id]
			if !ok {
				n = len(*links)
				index[id] = n
				*links = append(*links, Hardlink{Disk: file.Disk, Dev: file.Dev, Ino: file.Ino, Bytes: file.Bytes})
			}
			l := &(*links)[n]
			l.Links = max(l.Links, int(file.Links))
			l.Folders = append(l.Folders, f)
			l.files = append(l.files, file)
			continue
		}
		f.Size += file.Bytes
		f.DiskBytes[file.Disk] += file.Bytes
	}
	f.Items = int64(len(f.Files) + len(f.Folders))
	for _, c := range f.Folders {
		finish(c, disks, links, index)
		f.Size += c.Size
		f.Items += c.Items
		for d, n := range c.DiskBytes {
			f.DiskBytes[d] += n
		}
	}
	f.SingleBytes = f.Size
}

// addHardlinks adds each Hardlink once to the Size and DiskBytes of every
// Folder holding at least one counted name, and to the SharedBytes of those
// holding only some of them.
func addHardlinks(links []Hardlink) {
	seen := map[*Folder]int{}
	counted := map[*Folder]bool{}
	for i := range links {
		l := &links[i]
		l.Links = max(l.Links, len(l.Folders))
		paths := make([]string, len(l.Folders))
		for j, f := range l.Folders {
			paths[j] = f.Path()
		}
		owners := CountingPaths(paths)
		for j, f := range l.Folders {
			if slices.Contains(owners, paths[j]) {
				l.CountedFolders = append(l.CountedFolders, f)
			} else {
				l.files[j].CountedIn = owners[0]
			}
		}
		l.files = nil
		clear(counted)
		for _, f := range l.CountedFolders {
			for g := f; g != nil && !counted[g]; g = g.Parent {
				counted[g] = true
			}
		}
		clear(seen)
		for _, f := range l.Folders {
			for g := f; g != nil; g = g.Parent {
				seen[g]++
			}
		}
		for g, names := range seen {
			if !counted[g] {
				g.CountedElsewhereBytes += l.Bytes
				continue
			}
			g.Size += l.Bytes
			g.DiskBytes[l.Disk] += l.Bytes
			if names < l.Links {
				g.SharedBytes += l.Bytes
			}
		}
	}
}
