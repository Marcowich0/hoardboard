package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/Marcowich0/hoardboard/web"
)

// webApp serves the embedded single-page app.
//
// Vite names the files it builds under /static/ by their content hash, so a
// browser may keep them for good. Every other file (index.html, and those
// from web/public, which have no hash) is asked for again each time.
//
// A path that is not a file gets index.html when it is a client-side route
// (/categories, /explorer/data/media), so deep links and reloads work. When it
// looks like a file (under /static/, or with an extension) it gets a 404
// instead: after an upgrade, a tab left open asks for chunks that are gone,
// and its import has to fail rather than receive a page.
func webApp() http.Handler {
	files := web.Files()
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		built := strings.HasPrefix(name, "static/")
		if info, err := fs.Stat(files, name); name != "" && err == nil && !info.IsDir() {
			if built {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		if built || path.Ext(name) != "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, files, "index.html")
	})
}
