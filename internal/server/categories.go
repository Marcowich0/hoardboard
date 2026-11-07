package server

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/Marcowich0/hoardboard/internal/category"
)

func init() {
	addRoutes(func(s *Server) {
		s.mux.HandleFunc("GET /api/categories", s.handleCategories)
		s.mux.HandleFunc("POST /api/categories", s.handleCreateCategory)
		s.mux.HandleFunc("PATCH /api/categories/{id}", s.handleUpdateCategory)
		s.mux.HandleFunc("DELETE /api/categories/{id}", s.handleDeleteCategory)
		s.mux.HandleFunc("POST /api/assignments", s.handleAssign)
		s.mux.HandleFunc("DELETE /api/assignments", s.handleUnassign)
		s.mux.HandleFunc("POST /api/assignments/repoint", s.handleRepoint)
	})
}

// categoriesJSON is the answer of GET /api/categories (Categories in
// web/src/api/types.ts).
type categoriesJSON struct {
	Categories    []categoryJSON    `json:"categories"`
	Uncategorized uncategorizedJSON `json:"uncategorized"`
}

type uncategorizedJSON struct {
	Size int64 `json:"size"`
}

// categoryJSON is Category.
type categoryJSON struct {
	ID     category.ID `json:"id"`
	Name   string      `json:"name"`
	Colour string      `json:"colour"`
	Rank   int         `json:"rank"`
	// GroupID is the group it is in, null at the top level.
	GroupID *category.ID `json:"groupId"`
	// Size, SharedWith: a group's cover the categories in it too.
	Size        int64            `json:"size"`
	SharedWith  []sharedWithJSON `json:"sharedWith"`
	Assignments []assignmentJSON `json:"assignments"`
}

type sharedWithJSON struct {
	// CategoryID is null for Uncategorized.
	CategoryID *category.ID `json:"categoryId"`
	Bytes      int64        `json:"bytes"`
}

type assignmentJSON struct {
	Path string `json:"path"`
	// Size is the Folder's Size, null when it is Missing.
	Size *int64 `json:"size"`
	// Missing is true when the latest Scan has no Folder at Path.
	Missing bool `json:"missing"`
}

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	out, err := s.categoriesJSON(r)
	if err != nil {
		s.categoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// categoryInputJSON is CategoryInput: name and colour for POST, any field
// for PATCH.
type categoryInputJSON struct {
	Name   *string `json:"name"`
	Colour *string `json:"colour"`
	// GroupID: a category id puts it in that group, null at the top
	// level; left out, it stays where it is.
	GroupID optionalID `json:"groupId"`
}

// optionalID is a JSON field that may be left out (set false), null (set,
// id 0) or a category id.
type optionalID struct {
	set bool
	id  category.ID
}

func (o *optionalID) UnmarshalJSON(b []byte) error {
	o.set = true
	if string(b) == "null" {
		o.id = 0
		return nil
	}
	if err := json.Unmarshal(b, &o.id); err != nil {
		return err
	}
	if o.id <= 0 {
		return errors.New("groupId: want a category id or null")
	}
	return nil
}

// input is in as the Store takes it.
func (in categoryInputJSON) input() category.Input {
	out := category.Input{Name: in.Name, Colour: in.Colour}
	if in.GroupID.set {
		out.Group = &in.GroupID.id
	}
	return out
}

// handleCreateCategory adds a category below the others in Rank and
// answers 201 with it.
func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var in categoryInputJSON
	if !readJSONBody(w, r, &in) {
		return
	}
	id, err := s.cats.Create(r.Context(), in.input())
	if err != nil {
		s.categoryError(w, err)
		return
	}
	s.writeCategory(w, r, http.StatusCreated, id)
}

// handleUpdateCategory renames, recolours and/or regroups a category and
// answers with it.
func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := categoryID(w, r)
	if !ok {
		return
	}
	var in categoryInputJSON
	if !readJSONBody(w, r, &in) {
		return
	}
	if err := s.cats.Update(r.Context(), id, in.input()); err != nil {
		s.categoryError(w, err)
		return
	}
	s.writeCategory(w, r, http.StatusOK, id)
}

// handleDeleteCategory deletes a category and its Assignments: their
// Folders go to Uncategorized, or to a category assigned above them. The
// categories in it move to the top level.
func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := categoryID(w, r)
	if !ok {
		return
	}
	if err := s.cats.Delete(r.Context(), id); err != nil {
		s.categoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// assignJSON is the body of POST /api/assignments.
type assignJSON struct {
	Path       string      `json:"path"`
	CategoryID category.ID `json:"categoryId"`
}

// handleAssign puts a Folder of the latest Scan into a category, replacing
// its own Assignment if it has one, and answers 204.
func (s *Server) handleAssign(w http.ResponseWriter, r *http.Request) {
	var in assignJSON
	if !readJSONBody(w, r, &in) {
		return
	}
	if in.Path == "" || !validPath(in.Path) {
		writeError(w, http.StatusBadRequest, `path: want a Folder's Share path such as "data/media", without leading or trailing "/"`)
		return
	}
	t := s.scans.Tree()
	if t == nil {
		writeError(w, http.StatusNotFound, "no folder "+in.Path+": nothing is scanned yet")
		return
	}
	if _, ok := t.Lookup(in.Path); !ok {
		writeError(w, http.StatusNotFound, "no folder "+in.Path)
		return
	}
	if err := s.cats.Assign(r.Context(), in.Path, in.CategoryID); err != nil {
		s.categoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// repointJSON is the body of POST /api/assignments/repoint.
type repointJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// handleRepoint moves a Missing Assignment to the Folder of the latest Scan
// it is called now, and answers 204. The Folder's History before it
// existed becomes the old Folder's (docs/adr/0006-re-point-carries-history.md).
func (s *Server) handleRepoint(w http.ResponseWriter, r *http.Request) {
	var in repointJSON
	if !readJSONBody(w, r, &in) {
		return
	}
	for _, p := range []string{in.From, in.To} {
		if p == "" || !validPath(p) {
			writeError(w, http.StatusBadRequest, `from, to: want a Folder's Share path such as "data/media", without leading or trailing "/"`)
			return
		}
	}
	t := s.scans.Tree()
	if t == nil {
		writeError(w, http.StatusNotFound, "no folder "+in.To+": nothing is scanned yet")
		return
	}
	if _, ok := t.Lookup(in.To); !ok {
		writeError(w, http.StatusNotFound, "no folder "+in.To)
		return
	}
	if _, ok := t.Lookup(in.From); ok {
		writeError(w, http.StatusConflict, in.From+" is not Missing: the latest Scan found it")
		return
	}
	err := s.cats.Repoint(r.Context(), in.From, in.To, func(tx *sql.Tx) error {
		return s.history.Rename(r.Context(), tx, in.To, in.From)
	})
	if err != nil {
		s.categoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleUnassign removes the Assignment of the Folder at ?path= and
// answers 204: it goes back to the category assigned above it, if any.
func (s *Server) handleUnassign(w http.ResponseWriter, r *http.Request) {
	if err := s.cats.Unassign(r.Context(), r.URL.Query().Get("path")); err != nil {
		s.categoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeCategory answers status with the category id as GET /api/categories
// shows it.
func (s *Server) writeCategory(w http.ResponseWriter, r *http.Request, status int, id category.ID) {
	all, err := s.categoriesJSON(r)
	if err != nil {
		s.categoryError(w, err)
		return
	}
	for _, c := range all.Categories {
		if c.ID == id {
			writeJSON(w, status, c)
			return
		}
	}
	writeError(w, http.StatusNotFound, "No category "+strconv.FormatInt(int64(id), 10))
}

// categoryID reads the {id} of the URL, answering 400 if it is not one.
func categoryID(w http.ResponseWriter, r *http.Request) (category.ID, bool) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || n <= 0 {
		writeError(w, http.StatusBadRequest, "id: want a category id")
		return 0, false
	}
	return category.ID(n), true
}

// readJSONBody decodes the request's JSON body into v, answering 400 if it
// can't.
func readJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "body: "+err.Error())
		return false
	}
	return true
}

// categoriesJSON is every category, measured in the latest Scan's tree.
func (s *Server) categoriesJSON(r *http.Request) (categoriesJSON, error) {
	ctx := r.Context()
	list, err := s.cats.List(ctx)
	if err != nil {
		return categoriesJSON{}, err
	}
	assigned, err := s.cats.Assignments(ctx)
	if err != nil {
		return categoriesJSON{}, err
	}
	t := s.scans.Tree()
	sizes := category.Measure(t, assigned)

	byCategory := map[category.ID][]assignmentJSON{}
	for path, id := range assigned {
		a := assignmentJSON{Path: path}
		if t != nil {
			if f, ok := t.Lookup(path); ok {
				a.Size = &f.Size
			} else {
				a.Missing = true
			}
		}
		byCategory[id] = append(byCategory[id], a)
	}

	out := categoriesJSON{
		Categories:    make([]categoryJSON, 0, len(list)),
		Uncategorized: uncategorizedJSON{Size: sizes.Uncategorized()},
	}
	for i, c := range list {
		members := category.Members(list, c.ID)
		shared := []sharedWithJSON{}
		for _, sh := range sizes.SharedWith(members...) {
			j := sharedWithJSON{Bytes: sh.Bytes}
			if sh.With != category.Uncategorized {
				j.CategoryID = &sh.With
			}
			shared = append(shared, j)
		}
		assignments := byCategory[c.ID]
		if assignments == nil {
			assignments = []assignmentJSON{}
		}
		slices.SortFunc(assignments, func(a, b assignmentJSON) int { return cmp.Compare(a.Path, b.Path) })
		var group *category.ID
		if c.Group != 0 {
			group = &c.Group
		}
		out.Categories = append(out.Categories, categoryJSON{
			ID:          c.ID,
			Name:        c.Name,
			Colour:      c.Colour,
			Rank:        i + 1,
			GroupID:     group,
			Size:        sizes.Size(members...),
			SharedWith:  shared,
			Assignments: assignments,
		})
	}
	return out, nil
}

// categoryError answers the category Store's err: 404, 400 or 409 with its
// message, or 500 for anything else.
func (s *Server) categoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, category.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, category.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, category.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	default:
		s.log.Error("categories", "err", err)
		writeError(w, http.StatusInternalServerError, "categories: "+err.Error())
	}
}
