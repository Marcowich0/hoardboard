package server

import (
	"net/http"
	"strings"
)

// contentSecurityPolicy limits what a page of hoardboard can do if a bug ever
// lets something into it (file names come from the disks, and are shown in
// the page). Everything is served by hoardboard itself: nothing from
// elsewhere loads and no inline script runs, so web/index.html keeps its
// early theme script in a file.
//
// style-src allows 'unsafe-inline' because ECharts' HTML tooltips are built
// with inline style attributes. Scripts stay 'self' only, which is what
// matters against injected markup.
var contentSecurityPolicy = strings.Join([]string{
	"default-src 'none'",
	"script-src 'self'",
	"style-src 'self' 'unsafe-inline'",
	"img-src 'self' data:",
	"font-src 'self'",
	"connect-src 'self'",
	"base-uri 'none'",
	"form-action 'none'",
	"frame-ancestors 'none'",
}, "; ")

// securityHeaders sets the hardening headers on every response. It is the
// outermost handler, so that errors and rejections carry them too.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		// same-origin rather than no-referrer: under no-referrer browsers can
		// send "Origin: null" on a same-origin POST, which on plain HTTP
		// cross-origin protection would refuse.
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=(), payment=(), usb=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
