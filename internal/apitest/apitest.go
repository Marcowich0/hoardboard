// Package apitest runs a real hoardboard server in-process for HTTP API tests
// (Seam 1 in spec #1).
//
// Each test builds a fake Unraid and drives the server only through HTTP,
// asserting on its JSON, the way the web app uses it:
//
//	app := apitest.Start(t)
//	app.Mount.File("disk1", "data/media/movie.mkv", 4*apitest.MiB)
//	app.Mount.File("disk2", "data/media/other.mkv", 2*apitest.MiB)
//	app.Status.StartMover()
//	app.Clock.Advance(24 * time.Hour)
//	var got overview
//	app.GetJSON(t, "/api/overview", http.StatusOK, &got)
//
// Start gives the server, unless options say otherwise:
//   - Mount: a temporary mount root with disk1/, disk2/ and cache/ (NewMount);
//   - Status: a temporary Unraid status folder for an idle, started array
//     listing those Disks (NewStatus);
//   - Clock: a Fake clock at Epoch;
//   - Data: a fresh temporary data folder;
//   - no scheduled Scans: only the test starts them (see WithSchedule).
//
// The server is configured through config.FromEnv, with the HOARDBOARD_*
// variables pointing at those folders, so the tests also cover how the
// binary reads its settings. Never assert on the database, the scanner or
// any other internals: only on what the API answers.
package apitest

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Marcowich0/hoardboard/internal/clock"
	"github.com/Marcowich0/hoardboard/internal/config"
	"github.com/Marcowich0/hoardboard/internal/server"
)

// Revision is the commit the test server reports it was built from.
const Revision = "apitest-revision"

// Epoch is where a Start's Fake clock begins: a Monday, mid-morning UTC.
var Epoch = time.Date(2026, time.March, 2, 10, 0, 0, 0, time.UTC)

// App is a running in-process server plus everything it was started over.
type App struct {
	URL    string       // base URL, no trailing slash
	Client *http.Client // talks to URL

	Mount  *Mount      // the fake mount root (HOARDBOARD_MOUNT_ROOT)
	Status *Status     // the fake Unraid status folder (HOARDBOARD_STATUS_DIR)
	Clock  *clock.Fake // the server's clock
	Data   string      // the data folder (HOARDBOARD_DATA_DIR)

	opts options
	stop func()
}

// Option changes what Start builds.
type Option func(*options)

type options struct {
	mount  *Mount
	status *Status
	clock  *clock.Fake
	data   string
	env    map[string]string
	logs   io.Writer
	sched  bool
}

// WithMount starts the server over m instead of a fresh NewMount.
func WithMount(m *Mount) Option { return func(o *options) { o.mount = m } }

// WithStatus starts the server over s instead of a fresh NewStatus.
func WithStatus(s *Status) Option { return func(o *options) { o.status = s } }

// WithClock gives the server c instead of a fresh Fake at Epoch.
func WithClock(c *clock.Fake) Option { return func(o *options) { o.clock = c } }

// WithDataDir starts the server over the data folder dir instead of a fresh
// one, for example the Data of an App that was stopped.
func WithDataDir(dir string) Option { return func(o *options) { o.data = dir } }

// WithEnv sets an extra environment variable for config.FromEnv, such as
// HOARDBOARD_SCAN_INTERVAL. It wins over what Start sets itself.
func WithEnv(key, value string) Option {
	return func(o *options) {
		if o.env == nil {
			o.env = map[string]string{}
		}
		o.env[key] = value
	}
}

// WithSchedule runs scheduled Scans (every HOARDBOARD_SCAN_INTERVAL) on the
// fake clock, as in production. Without it the server scans only when asked
// to, so that moving the clock starts nothing.
func WithSchedule() Option { return func(o *options) { o.sched = true } }

// WithLogs sends the server's logs to w (they are discarded by default).
// t.Output() is a good choice while debugging a test.
func WithLogs(w io.Writer) Option { return func(o *options) { o.logs = w } }

// Start starts a server and stops it when the test ends.
func Start(t testing.TB, opts ...Option) *App {
	t.Helper()
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.mount == nil {
		o.mount = NewMount(t)
	}
	if o.status == nil {
		o.status = NewStatus(t)
	}
	if o.clock == nil {
		o.clock = clock.NewFake(Epoch)
	}
	if o.data == "" {
		o.data = t.TempDir()
	}
	app := &App{Mount: o.mount, Status: o.status, Clock: o.clock, Data: o.data, opts: o}
	app.start(t)
	return app
}

func (a *App) start(t testing.TB) {
	t.Helper()
	env := map[string]string{
		config.EnvMountRoot: a.Mount.Root,
		config.EnvStatusDir: a.Status.Dir,
		config.EnvDataDir:   a.Data,
	}
	for k, v := range a.opts.env {
		env[k] = v
	}
	cfg, err := config.FromEnv(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	cfg.Revision = Revision
	cfg.Clock = a.Clock
	cfg.MountInfo = a.Mount.mountInfo
	cfg.Statfs = a.Mount.statfs
	cfg.NoSchedule = !a.opts.sched

	logs := a.opts.logs
	if logs == nil {
		logs = io.Discard
	}
	srv, err := server.New(cfg, slog.New(slog.NewTextHandler(logs, nil)))
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	hs := httptest.NewServer(srv.Handler())
	a.URL = hs.URL
	a.Client = hs.Client()
	stopped := false
	a.stop = func() {
		if stopped {
			return
		}
		stopped = true
		hs.Close()
		if err := srv.Close(); err != nil {
			t.Errorf("close server: %v", err)
		}
	}
	t.Cleanup(a.stop)
}

// Stop shuts the server down. Start a new App WithDataDir(a.Data) (or call
// Restart) to bring it back over the same data.
func (a *App) Stop() { a.stop() }

// Restart stops the server and starts a new one over the same mount, status
// folder, clock and data folder, as when the container restarts.
func (a *App) Restart(t testing.TB) {
	t.Helper()
	a.stop()
	a.start(t)
}

// Do sends req (whose URL is relative to the server, such as /api/health)
// and returns the response. The body is closed at the end of the test.
func (a *App) Do(t testing.TB, req *http.Request) *http.Response {
	t.Helper()
	u, err := req.URL.Parse(a.URL + req.URL.RequestURI())
	if err != nil {
		t.Fatal(err)
	}
	req.URL = u
	req.Host = u.Host
	resp, err := a.Client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// Get sends GET path and returns the response.
func (a *App) Get(t testing.TB, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return a.Do(t, req)
}

// GetJSON sends GET path, fails the test unless the status is wantStatus,
// and decodes the JSON body into out (unless out is nil).
func (a *App) GetJSON(t testing.TB, path string, wantStatus int, out any) {
	t.Helper()
	a.expectJSON(t, a.Get(t, path), wantStatus, out)
}

// PostJSON sends body (nil for none) as JSON to path, fails the test unless
// the status is wantStatus, and decodes the JSON answer into out (unless out
// is nil).
func (a *App) PostJSON(t testing.TB, path string, body any, wantStatus int, out any) {
	t.Helper()
	a.SendJSON(t, http.MethodPost, path, body, wantStatus, out)
}

// SendJSON is PostJSON for any method (PUT, PATCH, DELETE, …).
func (a *App) SendJSON(t testing.TB, method, path string, body any, wantStatus int, out any) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, path, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	a.expectJSON(t, a.Do(t, req), wantStatus, out)
}

func (a *App) expectJSON(t testing.TB, resp *http.Response, wantStatus int, out any) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: read body: %v", resp.Request.Method, resp.Request.URL.Path, err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: status %d, want %d; body: %s",
			resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, wantStatus, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("%s %s: Content-Type %q, want application/json; body: %s",
			resp.Request.Method, resp.Request.URL.Path, ct, body)
	}
	if out == nil {
		return
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("%s %s: decode %s: %v", resp.Request.Method, resp.Request.URL.Path, body, err)
	}
}

// Scan runs a Scan through the API, POST /api/scan?wait=true, and fails the
// test unless it finished (rather than failing, or being skipped for array
// work). It joins a Scan that is already running.
func (a *App) Scan(t testing.TB) {
	t.Helper()
	var got struct {
		Started bool `json:"started"`
	}
	a.PostJSON(t, "/api/scan?wait=true", nil, http.StatusOK, &got)
	if !got.Started {
		t.Fatal("POST /api/scan: the Scan was skipped")
	}
}
