# Testing

## What a test checks

Tests check **external behaviour only**. They drive the HTTP API and assert on
its JSON, never on the scanner, the SQL, the in-memory tree or any other
internals, so those can be rewritten freely. If something can't be observed
through the API, it doesn't need a test of its own.

## Seam 1: the HTTP API in-process (`internal/apitest`)

```go
func TestAFolderSpreadOverDisksIsOneFolder(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 4*unit)
	app.Mount.File("disk2", "data/media/movies/b.mkv", 2*unit)

	app.Scan(t) // POST /api/scan?wait=true
	var got tree
	app.GetJSON(t, "/api/tree?path=data/media", http.StatusOK, &got)
	// assert on got
}
```

`apitest.Start` builds, per test:

- **`app.Mount`**: a temporary mount root with `disk1/`, `disk2/` and `cache/`.
  `File` writes real (random, non-sparse) bytes, so sizes in whole multiples of
  `apitest.BlockSize` are exact on disk. `Link` makes hardlinks, `Symlink`
  symlinks, `RootOnly` a folder an ordinary user can't list, `Fifo` a named
  pipe (a Scan that opened files would hang on it), `Remove` and `Rename`
  change the tree between Scans, `AddDisk` adds a Disk's folder. It also fakes
  the kernel's mountinfo: `MountPoint` makes a folder another filesystem's
  mount point, which a Scan must not enter, and `HoldScans` makes Scans wait
  at their start until released, so a test can see one running, join it, or
  move the clock while it runs.
- **`app.Status`**: a temporary Unraid status folder with `disks.ini` and
  `var.ini` for an idle, started array (parity, disk1, disk2, an empty disk3
  slot, cache, flash). `StartParityCheck`, `PauseParityCheck`,
  `StartRebuild`, `StartMover`, `StopArray`, `Idle`, `AddDataDisk`, `SetVar`
  and `SetDisk` change it. As on the real server, `fsMountpoint` is always
  `/mnt/<name>`: the app finds a Disk at `<mount root>/<name>`.
- **`app.Clock`**: a fake clock at `apitest.Epoch`; `Advance` and `Set` move it.
  Timers set on it (the scan schedule's) fire inside `Advance`/`Set`, in
  order, each with the clock at its time, so the test sees their effect as
  soon as the call returns.
- **No schedule** unless `apitest.WithSchedule()`: by default a test server
  never scans on its own, however far the clock moves. With it, the server
  schedules Scans every `HOARDBOARD_SCAN_INTERVAL` on the fake clock, and
  starts one at once if none ran in the last interval (so pair it with
  `HoldScans` to see that one running).
- **`app.Scan(t)`**: runs a Scan through `POST /api/scan?wait=true`, which
  answers once the Scan has ended, and fails the test if it was skipped for
  array work. To check a skip, `PostJSON` to `/api/scan` and read `started`.
- **`app.Data`**: a fresh data folder. `app.Restart(t)` restarts the server over
  the same mount, status, clock and data.

The server is configured through the same `HOARDBOARD_*` variables as
production (`WithEnv` adds more). Pass `apitest.WithLogs(t.Output())` to see its
logs while debugging.

**Limit**: every fake Disk is on one real filesystem, so assert that totals
are consistent (the parts sum to Used space), not exact free-space numbers.

**Root-only folders**: an ordinary test run can't list a `RootOnly` folder,
which is the point; the container can, thanks to `CAP_DAC_READ_SEARCH`. A test
that needs that capability skips unless it has it; the container smoke test
covers it in CI, with a root-only folder in its fixture.

## Seam 2: the container (`scripts/smoke-test.sh`)

CI builds the image and starts it as deployed (capabilities dropped,
read-only root filesystem, `/mnt` and the status folder read-only) over a tiny
fixture, then checks `/api/health`, the overview, and a Scan's tree: a
Folder merged across Disks, a root-only folder counted, a symlink not
followed, and a filesystem mounted inside a Disk not entered. These need the
real capability and real mounts, which Seam 1 can only fake. It uses `jq`.

## Rules

- `t.Parallel()` in every test; no fixed sleeps. Time moves through `app.Clock`.
- Expected values come from the fixture you built (a literal), never recomputed
  the way the code computes them.
- Keep files small (a few `apitest.BlockSize`): every test shares one real
  filesystem, and the overview tests watch its live Used space.
- The web app has lint, typecheck and unit tests of pure logic in v1, no
  browser tests (see below).

## The web app's unit tests

`cd web && npm test` runs `web/src/**/*.test.ts` with Node's own test runner
(`node:test`, TypeScript run as is). They cover pure logic only: the size and
time formatters, axis steps, paths and sorting in `web/src/lib/`, and the
in-browser fake API in `web/src/api/fake/`, tested through its `Api`, the
same way Seam 1 tests the server. Write them like the Go tests: behaviour
through the public function, expected values as literals.
