# Testing

## What a test checks

Tests check **external behaviour only**. They drive the HTTP API and assert on
its JSON, never on the scanner, the SQL, the in-memory tree or any other
internals, so those can be rewritten freely. If something can't be observed
through the API, it doesn't need a test of its own.

## Seam 1: the HTTP API in-process (`internal/apitest`)

```go
// An example of the shape (the scan and tree routes come with later tickets).
func TestMoviesSpreadOverTwoDisksAreOneFolder(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	app.Mount.File("disk1", "data/media/movies/a.mkv", 4*apitest.MiB)
	app.Mount.File("disk2", "data/media/movies/b.mkv", 2*apitest.MiB)

	app.PostJSON(t, "/api/scan", nil, http.StatusOK, nil)
	var tree treeResponse
	app.GetJSON(t, "/api/tree?path=data/media", http.StatusOK, &tree)
	// assert on tree
}
```

`apitest.Start` builds, per test:

- **`app.Mount`**: a temporary mount root with `disk1/`, `disk2/` and `cache/`.
  `File` writes real (random, non-sparse) bytes, so sizes in whole multiples of
  `apitest.BlockSize` are exact on disk. `Link` makes hardlinks, `Symlink`
  symlinks, `RootOnly` a folder an ordinary user can't list, `Remove` and
  `Rename` change the tree between Scans, `AddDisk` adds a Disk's folder.
- **`app.Status`**: a temporary Unraid status folder with `disks.ini` and
  `var.ini` for an idle, started array (parity, disk1, disk2, an empty disk3
  slot, cache, flash). `StartParityCheck`, `PauseParityCheck`,
  `StartRebuild`, `StartMover`, `StopArray`, `Idle`, `AddDataDisk`, `SetVar`
  and `SetDisk` change it. As on the real server, `fsMountpoint` is always
  `/mnt/<name>`: the app finds a Disk at `<mount root>/<name>`.
- **`app.Clock`**: a fake clock at `apitest.Epoch`; `Advance` and `Set` move it.
- **`app.Data`**: a fresh data folder. `app.Restart(t)` restarts the server over
  the same mount, status, clock and data.

The server is configured through the same `HOARDBOARD_*` variables as
production (`WithEnv` adds more). Pass `apitest.WithLogs(t.Output())` to see its
logs while debugging.

**Limit**: every fake Disk is on one real filesystem, so assert that totals
are consistent (the parts sum to Used space), not exact free-space numbers.

**Root-only folders**: an ordinary test run can't list a `RootOnly` folder,
which is the point; the container can, thanks to `CAP_DAC_READ_SEARCH`. A test
that needs that capability skips unless it has it, and CI can run it inside
the image (see Gamash's "tests on the image" step for the pattern).

## Seam 2: the container (`scripts/smoke-test.sh`)

CI builds the image and starts it as deployed (capabilities dropped,
read-only root filesystem, `/mnt` and the status folder read-only) over a tiny
fixture, then checks `/api/health` and the app's routes.

## Rules

- `t.Parallel()` in every test; no fixed sleeps. Time moves through `app.Clock`.
- Expected values come from the fixture you built (a literal), never recomputed
  the way the code computes them.
- The web app has lint and typecheck only in v1, no browser tests.
