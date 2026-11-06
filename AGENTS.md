# hoardboard

A LAN-only dashboard for an Unraid server's storage: what each folder and
Category takes, how that changes over time, and when the array will be full.
One Go binary serves a JSON API and an embedded React app; state is one SQLite
file. Spec: GitHub issue #1.

## Read first

- **`GLOSSARY.md`**: the domain terms (Disk, Share, Folder, Size, Shared
  bytes, Used space, Unaccounted, Category, Rank, Assignment, Scan, History).
  Use them in code, tests, API fields and UI text; avoid the words it lists
  under _Avoid_.
- **`docs/adr/`**: decisions and why. Respect them; record a new one when you
  make a decision that is hard to reverse.
- **`docs/testing.md`**: how tests are written here.

## Never read file contents

hoardboard reads **metadata only**: names, sizes, inode numbers. It never
opens a file on the scanned disks, and nothing you add may either: no
hashing, no content sniffing, no thumbnails. Directories are opened only to
list them. The same goes for you as an agent: never read the contents of the
owner's files on the server, only their metadata.

## Commands

| What | Command |
|---|---|
| Go tests | `go test ./...` (CI: `TMPDIR=/dev/shm go test -race -gcflags='modernc.org/sqlite/lib=-race=false -d=checkptr=0' ./...`) |
| Go lint | `gofmt -l .` (must print nothing), `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...` |
| Go vulnerabilities | `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` |
| Web install | `cd web && npm ci` |
| Web lint / typecheck / tests / build | `cd web && npm run lint && npm run typecheck && npm test && npm run build` |
| Binary with the real app | `cd web && npm run build && cd .. && go build -tags webdist ./cmd/hoardboard` |
| Image | `docker build --build-arg REVISION=$(git rev-parse HEAD) -t hoardboard:dev .` |
| Container smoke test | `scripts/smoke-test.sh hoardboard:dev` |
| Dev server | `HOARDBOARD_MOUNT_ROOT=… HOARDBOARD_STATUS_DIR=… HOARDBOARD_DATA_DIR=… go run ./cmd/hoardboard`, then `cd web && npm run dev` (Vite proxies `/api` to :8098) |

Without `-tags webdist`, the binary embeds the stub page in `web/stub`, so
`go build`, `go vet` and `go test` work without Node.

## Web app rules

- Pages get data only through `api` from `web/src/api/index.ts`, typed by
  `web/src/api/types.ts`. Until an endpoint exists on the server, its line in
  `index.ts` points at the in-browser fake (`web/src/api/fake/`); wiring a
  page to the server means adding the endpoint to `web/src/api/http.ts` and
  pointing that line at it. Keep `types.ts` and the Go handlers in step.
- Sizes go through `formatBytes`/`formatChange` (`web/src/lib/format.ts`):
  binary units only (KiB, MiB, GiB, TiB).
- Category colours come from `useCategories().look(id)` (`web/src/categories.ts`),
  never from a hex in a page or chart, so every view shows a category alike.
- Names from the disks reach ECharts tooltips as HTML: pass them through
  `escapeHtml` (`web/src/lib/html.ts`).

CI (`.github/workflows/ci.yml`) runs all of the above on GitHub-hosted runners
for every pull request and push, and publishes `ghcr.io/marcowich0/hoardboard`
on pushes to `main`. The repository is public: no job in that workflow may use
a self-hosted runner.

## Settings

All settings are environment variables (`internal/config`); nothing about the
owner's folders is built in. Defaults are the paths inside the container.

| Variable | Default | Meaning |
|---|---|---|
| `HOARDBOARD_MOUNT_ROOT` | `/mnt` | One folder per Disk (`disk1`, …, `cache`), read-only |
| `HOARDBOARD_STATUS_DIR` | `/var/local/emhttp` | Unraid's status folder (`disks.ini`, `var.ini`), read-only |
| `HOARDBOARD_DATA_DIR` | `/data` | Holds `hoardboard.db`, read-write |
| `HOARDBOARD_PORT` | `8098` | HTTP port |
| `HOARDBOARD_SCAN_INTERVAL` | `1h` | Time between scheduled Scans (Go duration, at least `1m`) |

The container runs as root with every capability dropped except
`CAP_DAC_READ_SEARCH` (`--cap-drop ALL --cap-add DAC_READ_SEARCH`), so it can
list and stat root-only folders and do nothing else a normal user can't. `/mnt`
and the status folder are mounted read-only; only `/data` is writable, and it
must be owned by root (root without `CAP_DAC_OVERRIDE` can't write elsewhere).

## Layout

```
cmd/hoardboard/       entry point: serve, or `hoardboard healthcheck` (the image's HEALTHCHECK)
internal/config/      settings from HOARDBOARD_* variables
internal/clock/       Clock (now/today), System and Fake
internal/db/          opens hoardboard.db; Migrate(ctx, db, module, steps) per module
internal/inventory/   the Disks Unraid lists (disks.ini, var.ini) and their live Used/Free space (statfs)
internal/tree/        the merged tree of Share paths: Builder (the merger), Finish (Size with each hardlinked file once, items, bytes per Disk, Shared bytes, Hardlinks), snapshot codec
internal/scan/        Scanner: one Scan at a time (Start joins a running one), metadata-only walk, scans + snapshot tables
internal/server/      HTTP server: one file per feature, routes added from init()
internal/apitest/     Seam 1 test harness: fake mount root, fake Unraid status, fake clock
web/                  React + Vite + TypeScript + Tailwind app; web.go embeds web/dist (-tags webdist)
  src/api/            THE data boundary: types.ts (the API's JSON), index.ts (each endpoint: real or fake), fake/
  src/categories.ts   the one source of category names and colours (colours: src/lib/colours.ts)
  src/lib/            pure helpers with node tests: sizes (binary units), days, paths, sorting, axes
  src/charts/         ECharts wrappers; every chart is ECharts
scripts/smoke-test.sh Seam 2: container smoke test
docs/adr/             architecture decisions
GLOSSARY.md           domain terms
```
