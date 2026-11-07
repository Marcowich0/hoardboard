package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

func TestTheAppIsServedOnEveryClientRoute(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	for _, path := range []string{"/", "/categories", "/explorer", "/explorer/data/media?period=30d"} {
		resp := app.Get(t, path)
		if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
			t.Errorf("GET %s: %d %q, want 200 text/html", path, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
}

func TestMissingFilesAndAPIRoutesAreJSON404s(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	for _, path := range []string{"/api/nope", "/static/gone-1234.js", "/favicon-old.png"} {
		var got struct {
			Error string `json:"error"`
		}
		app.GetJSON(t, path, http.StatusNotFound, &got)
		if got.Error == "" {
			t.Errorf("GET %s: no error message", path)
		}
	}
}
