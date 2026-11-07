package server_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// treemapNode is TreemapNode in web/src/api/types.ts.
type treemapNode struct {
	Kind                  string `json:"kind"`
	Name                  string `json:"name"`
	Path                  string `json:"path"`
	Size                  int64  `json:"size"`
	CountedElsewhereBytes int64  `json:"countedElsewhereBytes"`
	CountedIn             *struct {
		Path       string `json:"path"`
		CategoryID *int64 `json:"categoryId"`
	} `json:"countedIn"`
	CategoryID *int64        `json:"categoryId"`
	Items      int           `json:"items"`
	Children   []treemapNode `json:"children"`
}

func getTreemap(t *testing.T, app *apitest.App, path string) (size int64, children []treemapNode) {
	t.Helper()
	var got struct {
		Path     string        `json:"path"`
		Size     int64         `json:"size"`
		Children []treemapNode `json:"children"`
	}
	app.GetJSON(t, "/api/treemap?path="+path, http.StatusOK, &got)
	if got.Path != path || got.Children == nil {
		t.Fatalf("treemap = %+v, want path %q and a list of children", got, path)
	}
	return got.Size, got.Children
}

// outline is nodes as "name(size)[children]", sizes in units, for messages
// and comparisons.
func outline(nodes []treemapNode) string {
	var parts []string
	for _, n := range nodes {
		s := fmt.Sprintf("%s(%d)", n.Name, n.Size/unit)
		if n.Kind == "rest" {
			s = fmt.Sprintf("rest:%d(%d)", n.Items, n.Size/unit)
		}
		if n.Kind == "folder" && n.CategoryID != nil {
			s += fmt.Sprintf("#%d", *n.CategoryID)
		}
		if len(n.Children) > 0 {
			s += "[" + outline(n.Children) + "]"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

func TestTheTreemapNestsFoldersLargestFirstWithTheirCategory(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a/a.mkv", 6*unit)
	app.Mount.File("disk2", "data/media/movies/b.mkv", 3*unit)
	app.Mount.File("disk1", "data/media/tv/s01/e1.mkv", 2*unit)
	app.Mount.File("disk1", "data/notes.txt", 1*unit)
	app.Scan(t)
	movies := create(t, app, "Movies", "#2a78d6")
	assign(t, app, "data/media/movies", movies.ID)

	size, got := getTreemap(t, app, "data")
	if size != 12*unit {
		t.Errorf("size = %d, want %d", size, 12*unit)
	}
	want := fmt.Sprintf("media(11)[movies(9)#%d[a(6)#%[1]d[a.mkv(6)] b.mkv(3)] tv(2)[s01(2)[e1.mkv(2)]]] notes.txt(1)", movies.ID)
	if o := outline(got); o != want {
		t.Errorf("treemap of data =\n  %s\nwant\n  %s", o, want)
	}
	// Files carry their Folder's category.
	if f := got[0].Children[0].Children[1]; f.Kind != "file" || f.Path != "data/media/movies/b.mkv" || f.CategoryID == nil || *f.CategoryID != movies.ID {
		t.Errorf("b.mkv = %+v, want a file at data/media/movies/b.mkv in Movies", f)
	}
}

func TestTheTreemapFoldsWhatIsTooSmallToSeeIntoTheRest(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	// One big file of 24,000 blocks, and 3 crumbs of 1 block: below 1/20000 of the Share.
	app.Mount.File("disk1", "data/big.bin", 6000*unit)
	for i := range 3 {
		app.Mount.File("disk1", fmt.Sprintf("data/crumb%d", i), apitest.BlockSize)
	}
	app.Scan(t)

	_, got := getTreemap(t, app, "")
	if len(got) != 1 || got[0].Name != "data" {
		t.Fatalf("root = %s, want the one Share data", outline(got))
	}
	kids := got[0].Children
	if len(kids) != 2 || kids[0].Name != "big.bin" || kids[1].Kind != "rest" || kids[1].Items != 3 || kids[1].Size != 3*apitest.BlockSize {
		t.Errorf("data = %+v, want big.bin then the rest: 3 items, %d bytes", kids, 3*apitest.BlockSize)
	}
}

func TestTreemapRejectsBadRequests(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	if _, got := getTreemap(t, app, ""); len(got) != 0 {
		t.Errorf("before the first Scan = %s, want nothing", outline(got))
	}
	var e struct{ Error string }
	app.GetJSON(t, "/api/treemap?path=data/../etc", http.StatusBadRequest, &e)
	app.GetJSON(t, "/api/treemap?path=nope", http.StatusNotFound, &e)
}
