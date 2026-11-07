package server

import (
	"cmp"
	"container/heap"
	"net/http"
	"slices"

	"github.com/Marcowich0/hoardboard/internal/category"
	"github.com/Marcowich0/hoardboard/internal/tree"
)

func init() {
	addRoutes(func(s *Server) { s.mux.HandleFunc("GET /api/treemap", s.handleTreemap) })
}

// Treemap limits: at most treemapNodes nodes, at most treemapDepth levels
// below the Folder, and nothing smaller than 1/treemapFraction of it; what is
// left out of a Folder is one "rest" node. The web app fits what it gets to
// the chart's size (web/src/lib/treemap.ts), so this sends more than any one
// chart shows: a big chart can then show a folder of many movies as a grid
// of small tiles rather than one rest.
const (
	treemapNodes    = 3000
	treemapDepth    = 5
	treemapFraction = 20000
)

// treemapJSON is the answer of GET /api/treemap (Treemap in
// web/src/api/types.ts).
type treemapJSON struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	// Children are the Folder's, nested, largest first.
	Children []*treemapNodeJSON `json:"children"`
}

// treemapNodeJSON is TreemapNode: a Folder, a file, or the rest of a
// Folder's items, too small or too many to show one by one.
type treemapNodeJSON struct {
	Kind string `json:"kind"` // "folder", "file" or "rest"
	// Name and Path are "" for the rest.
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	// CategoryID is its effective category, null for Uncategorized (and
	// for the rest).
	CategoryID *category.ID `json:"categoryId"`
	// Items is, for the rest, how many items it stands for.
	Items int `json:"items,omitempty"`
	// Children are a Folder's, when the treemap opens it; a Folder left
	// closed has none.
	Children []*treemapNodeJSON `json:"children,omitempty"`
}

// handleTreemap answers the Folder at ?path= of the latest Scan as a nested
// treemap: its subfolders and files, and theirs, opening the largest Folders
// first until the treemap holds treemapNodes nodes.
func (s *Server) handleTreemap(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !validPath(path) {
		writeError(w, http.StatusBadRequest, `path: want a Share path such as "data/media", without leading or trailing "/"`)
		return
	}
	t := s.scans.Tree()
	if t == nil {
		t = tree.NewBuilder().Finish()
	}
	f, ok := t.Lookup(path)
	if !ok {
		writeError(w, http.StatusNotFound, "no folder "+path)
		return
	}
	assigned, err := s.cats.Assignments(r.Context())
	if err != nil {
		s.categoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, treemapJSON{
		Path:     path,
		Size:     f.Size,
		Children: buildTreemap(f, path, assigned),
	})
}

// opening is a Folder the treemap may open: its node, at its depth below
// the treemap's Folder (1 for a child).
type opening struct {
	folder *tree.Folder
	node   *treemapNodeJSON
	depth  int
}

// openings is a max-heap of Folders by Size: the treemap opens the
// largest first, as they have the most room to show what they hold.
type openings []opening

func (o openings) Len() int { return len(o) }
func (o openings) Less(i, j int) bool {
	if o[i].folder.Size != o[j].folder.Size {
		return o[i].folder.Size > o[j].folder.Size
	}
	return o[i].node.Path < o[j].node.Path
}
func (o openings) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o *openings) Push(x any)   { *o = append(*o, x.(opening)) }
func (o *openings) Pop() any {
	old := *o
	x := old[len(old)-1]
	*o = old[:len(old)-1]
	return x
}

// buildTreemap lists root's children (root is at path) and opens Folders,
// largest first, while the node budget lasts.
func buildTreemap(root *tree.Folder, path string, assigned category.Assignments) []*treemapNodeJSON {
	least := max(1, root.Size/treemapFraction)
	budget := treemapNodes
	top := &treemapNodeJSON{Path: path}
	queue := &openings{{folder: root, node: top, depth: 0}}
	for queue.Len() > 0 && budget > 0 {
		o := heap.Pop(queue).(opening)
		kids, folders := treemapChildren(o.folder, o.node.Path, least, assigned)
		if o.depth > 0 && len(kids) > budget {
			continue // too many to show: it stays one tile
		}
		budget -= len(kids)
		o.node.Children = kids
		if o.depth+1 >= treemapDepth {
			continue
		}
		for _, node := range kids {
			// A Folder too small for its items to be seen stays one tile.
			if sub, ok := folders[node]; ok && sub.Size >= least*4 && len(sub.Folders)+len(sub.Files) > 0 {
				heap.Push(queue, opening{folder: sub, node: node, depth: o.depth + 1})
			}
		}
	}
	if top.Children == nil {
		return []*treemapNodeJSON{}
	}
	return top.Children
}

// treemapChildren is f's subfolders and files of at least least bytes,
// largest first, and a rest node for the others; folders maps each
// subfolder's node to it.
func treemapChildren(f *tree.Folder, dir string, least int64, assigned category.Assignments) (out []*treemapNodeJSON, folders map[*treemapNodeJSON]*tree.Folder) {
	out = []*treemapNodeJSON{}
	folders = map[*treemapNodeJSON]*tree.Folder{}
	var shown int64
	rest := 0
	for _, c := range f.Folders {
		if c.Size < least {
			if c.Size > 0 {
				rest++
			}
			continue
		}
		p := tree.Join(dir, c.Name)
		n := &treemapNodeJSON{Kind: "folder", Name: c.Name, Path: p, Size: c.Size, CategoryID: categoryOf(assigned, p)}
		out = append(out, n)
		folders[n] = c
		shown += c.Size
	}
	fileCategory := categoryOf(assigned, dir)
	for _, file := range f.Files {
		if file.Bytes < least {
			if file.Bytes > 0 {
				rest++
			}
			continue
		}
		out = append(out, &treemapNodeJSON{Kind: "file", Name: file.Name, Path: tree.Join(dir, file.Name), Size: file.Bytes, CategoryID: fileCategory})
		shown += file.Bytes
	}
	slices.SortStableFunc(out, func(a, b *treemapNodeJSON) int { return cmp.Compare(b.Size, a.Size) })
	// A hardlinked file counts once in f's Size but once per name in its
	// children's, so what they show can pass it.
	if left := f.Size - shown; rest > 0 && left > 0 {
		out = append(out, &treemapNodeJSON{Kind: "rest", Size: left, Items: rest})
	}
	return out, folders
}
