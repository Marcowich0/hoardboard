package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

func TestHealthReportsOK(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	var got struct {
		Status   string `json:"status"`
		Revision string `json:"revision"`
	}
	app.GetJSON(t, "/api/health", http.StatusOK, &got)

	if got.Status != "ok" || got.Revision != apitest.Revision {
		t.Fatalf("health = %+v, want status ok and revision %q", got, apitest.Revision)
	}
}

func TestStateIsOneSQLiteFileInTheDataFolder(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	app.GetJSON(t, "/api/health", http.StatusOK, nil)

	if _, err := os.Stat(filepath.Join(app.Data, "hoardboard.db")); err != nil {
		t.Fatalf("database file: %v", err)
	}
}

func TestHealthFailsWithoutTheStatusFolder(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	if err := os.RemoveAll(app.Status.Dir); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	app.GetJSON(t, "/api/health", http.StatusServiceUnavailable, &got)

	if got.Status != "unhealthy" || got.Error == "" {
		t.Fatalf("health = %+v, want unhealthy with a reason", got)
	}
}

func TestHealthFailsWithoutTheMountRoot(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	if err := os.RemoveAll(app.Mount.Root); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Status string `json:"status"`
	}
	app.GetJSON(t, "/api/health", http.StatusServiceUnavailable, &got)

	if got.Status != "unhealthy" {
		t.Fatalf("health = %+v, want unhealthy", got)
	}
}
