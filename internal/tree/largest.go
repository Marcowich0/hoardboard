package tree

import (
	"cmp"
	"slices"
)

// Named is one name of a file in the tree: the File and the Share path of
// the Folder holding that name.
type Named struct {
	*File
	// Dir is the Share path of the Folder holding this name.
	Dir string
	// Path is the Share path of this name.
	Path string
}

// Largest lists the n largest physical files below f, largest first, then
// by path. A hardlinked file is listed once, by its first path in name
// order.
func (f *Folder) Largest(n int) []Named {
	walk := func(visit func(dir string, file *File)) {
		var rec func(f *Folder, dir string)
		rec = func(f *Folder, dir string) {
			for i := range f.Files {
				visit(dir, &f.Files[i])
			}
			for _, c := range f.Folders {
				rec(c, Join(dir, c.Name))
			}
		}
		rec(f, f.Path())
	}
	idOf := func(file *File) identity { return identity{file.Disk, file.Dev, file.Ino} }

	// The name each file with several names is listed by.
	firstName := map[identity]string{}
	walk(func(dir string, file *File) {
		if file.Links > 1 {
			p := Join(dir, file.Name)
			if q, ok := firstName[idOf(file)]; !ok || p < q {
				firstName[idOf(file)] = p
			}
		}
	})

	// top holds the largest so far, in answer order; a file smaller than its
	// last needs no path.
	top := make([]Named, 0, n+1)
	before := func(a, b Named) int {
		return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), cmp.Compare(a.Path, b.Path))
	}
	walk(func(dir string, file *File) {
		if len(top) == n && file.Bytes < top[len(top)-1].Bytes {
			return
		}
		e := Named{File: file, Dir: dir, Path: Join(dir, file.Name)}
		if file.Links > 1 && firstName[idOf(file)] != e.Path {
			return
		}
		if i, _ := slices.BinarySearchFunc(top, e, before); i < n {
			top = slices.Insert(top, i, e)[:min(len(top)+1, n)]
		}
	})
	return top
}

// Join is the Share path of name inside the Folder at path.
func Join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "/" + name
}
