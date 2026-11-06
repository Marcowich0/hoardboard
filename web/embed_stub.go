//go:build !webdist

package web

import "embed"

//go:embed stub
var stub embed.FS

// Built reports whether the real web app is embedded.
const Built = false

var files = mustSub(stub, "stub")
