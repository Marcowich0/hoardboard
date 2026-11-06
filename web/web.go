// Package web embeds the built single-page app for the Go server.
//
// The real app (web/dist, made by `npm run build`) is embedded only when
// building with `-tags webdist`. Without the tag, a small committed stub page
// (web/stub) is embedded instead, so `go build`, `go vet` and `go test` work in
// a fresh checkout without Node. The Dockerfile and CI build with the tag.
package web

import "io/fs"

// Files returns the app's static files, rooted at the folder holding index.html.
func Files() fs.FS { return files }

func mustSub(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
