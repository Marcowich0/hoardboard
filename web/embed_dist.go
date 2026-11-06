//go:build webdist

package web

import "embed"

//go:embed all:dist
var dist embed.FS

// Built reports whether the real web app is embedded.
const Built = true

var files = mustSub(dist, "dist")
