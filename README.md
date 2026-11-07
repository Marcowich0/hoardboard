# hoardboard

A self-hosted storage dashboard for Unraid. See which folders and categories take up space, how they grow over time, and when the array might fill up.

One Go binary serves the API and the embedded React app. Categories, assignments, the latest scan and daily history live in a SQLite database; no separate database service is needed.

[Deploy](#deploy-on-unraid) · [Configuration](#configuration) · [Usage](#first-scan-and-everyday-use) · [Storage accounting](#understanding-the-numbers) · [Maintenance](#updates-backups-and-health) · [Troubleshooting](#troubleshooting) · [Development](#development)

## What it does

- **Overview:** live Used space and Free space, per-disk category breakdowns, parity status, a category donut and a stacked history chart.
- **Explorer:** a merged tree of Shares across Disks, a nested treemap, folder history, changes over 7 days, 30 days or a year, and the largest files below a Folder.
- **Categories:** named, coloured collections of Folders from any Share, optional Groups, and a draggable Rank that decides which Category owns Shared bytes in the Overview.
- **Daily history and forecasts:** the last successful Scan of each local day supplies that day's history. Growth and the fill-up estimate use up to 90 days of Used space and need at least a week of history.
- **Metadata-only scans:** reads names, allocated sizes and inode metadata. It never opens the contents of scanned files, hashes them or generates thumbnails.
- **Low-impact scheduling:** scans default to once an hour, at idle I/O priority and reduced CPU priority. New scans are skipped while the array is stopped, a parity operation or disk rebuild is running or paused, or the mover is active.

hoardboard scans the individual array data disks and cache pools, rather than `/mnt/user`, so it can identify hardlinks and show where a Folder's bytes are stored. It does not follow symlinks or enter other filesystems mounted below a Disk.

## Deploy on Unraid

Run the container **on the Unraid host**, with Docker and a started array. The published image currently targets **Linux/amd64**. You do not need Go or Node.js to deploy it.

The dashboard has **no built-in authentication** and is intended for a trusted LAN. Keep its port off the public internet. The examples publish port `8098`; optionally bind it to a specific LAN address, such as `192.168.1.10:8098:8098`, instead.

### Docker

In an Unraid terminal, create the persistent data folder. It must be writable by root, because the container runs as root with most capabilities removed:

```sh
install -d -o root -g root -m 0755 /mnt/user/appdata/hoardboard
```

Start hoardboard:

```sh
docker run -d --name hoardboard \
  --restart unless-stopped \
  --cap-drop ALL --cap-add DAC_READ_SEARCH \
  --security-opt no-new-privileges:true \
  --read-only --tmpfs /tmp \
  -p 8098:8098 \
  -v /mnt:/mnt:ro \
  -v /var/local/emhttp:/var/local/emhttp:ro \
  -v /mnt/user/appdata/hoardboard:/data \
  -e TZ=Europe/Copenhagen \
  ghcr.io/marcowich0/hoardboard:latest
```

Open **`http://<unraid-host>:8098`** in your browser. Replace `<unraid-host>` with your server's LAN address or hostname. Set `TZ` to your own timezone before collecting history.

### Docker Compose

If you have the Docker Compose plugin installed, you can use it instead of `docker run`. Create the data folder above, then save this as `compose.yaml`:

```yaml
services:
  hoardboard:
    image: ghcr.io/marcowich0/hoardboard:latest
    container_name: hoardboard
    restart: unless-stopped
    cap_drop:
      - ALL
    cap_add:
      - DAC_READ_SEARCH
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
    ports:
      - "8098:8098"
    environment:
      TZ: Europe/Copenhagen
      HOARDBOARD_SCAN_INTERVAL: 1h
    volumes:
      - /mnt:/mnt:ro
      - /var/local/emhttp:/var/local/emhttp:ro
      - /mnt/user/appdata/hoardboard:/data
```

```sh
docker compose up -d
```

Use either deployment method for a given instance; both examples name the container `hoardboard`.

### Mounts and permissions

| Host folder | Container folder | Access | Purpose |
| --- | --- | --- | --- |
| `/mnt` | `/mnt` | Read-only | Individual Disk folders, such as `disk1`, `disk2` and `cache` |
| `/var/local/emhttp` | `/var/local/emhttp` | Read-only | Unraid's `disks.ini` inventory and `var.ini` array state |
| `/mnt/user/appdata/hoardboard` | `/data` | Read-write | `hoardboard.db` and SQLite's working files |

Keep the whole `/mnt` mount: pointing the scanner at `/mnt/user` loses the individual Disk layout it expects. The `/data` folder is the persistent state; keep it when recreating the container.

`DAC_READ_SEARCH` allows the scanner to list and stat root-only Folders. It does not need a privileged container. Both storage inputs stay mounted read-only; only the data folder and temporary filesystem are writable.

## Configuration

Settings are environment variables. The paths below are **inside the container**; changing a host bind-mount source alone does not require changing them.

| Variable | Default | Meaning |
| --- | --- | --- |
| `HOARDBOARD_MOUNT_ROOT` | `/mnt` | Folder containing one folder per Disk, with names matching Unraid's inventory |
| `HOARDBOARD_STATUS_DIR` | `/var/local/emhttp` | Folder containing `disks.ini` and `var.ini` |
| `HOARDBOARD_DATA_DIR` | `/data` | Writable folder for `hoardboard.db` |
| `HOARDBOARD_PORT` | `8098` | HTTP listen port inside the container, from `1` to `65535` |
| `HOARDBOARD_SCAN_INTERVAL` | `1h` | Go duration of at least `1m`, for example `30m`, `2h` or `24h` |
| `TZ` | System timezone | Timezone used to decide which local day a Scan belongs to, for example `Europe/Copenhagen` or `Etc/UTC` |

All configured paths must be absolute. If you only want a different browser-facing port, change the host side of the port mapping, for example `8100:8098`, and leave `HOARDBOARD_PORT` alone. Recreate the container after changing environment variables.

## First scan and everyday use

1. **Wait for the first Scan.** A new installation schedules one immediately. On restart, it loads the saved tree and scans immediately if the last scan attempt is older than the configured interval. The header shows scan status and duration.
2. **Open Explorer.** Start at **All shares**, then open a Folder or click a treemap tile. Breadcrumbs take you back up. Sort Contents by Name, Size, Items or Change; use the period selector to compare growth.
3. **Create Categories.** In **Categories**, choose **New category**, set its name and colour, then use **Add folder**. You can also choose **Add to category…** in Explorer. An Assignment covers the Folder and everything below it unless a more specific Assignment overrides it.
4. **Arrange Groups and Rank.** Put related Categories into a Group using the Category's **Edit** dialog, or use **New category in it** on a Group. Drag the Rank handles to reorder. Groups are one level deep and move with their Categories.
5. **Return to Overview.** Category colours are consistent across the app. Select a Category in a chart to open its details. Use **Scan now** after storage changes when you want fresh Folder sizes; only one Scan runs at a time.

### Categories

A Folder can be assigned to one Category. A more specific Assignment wins: for example, assigning `data/media` to Media and `data/media/movies` to Movies puts the movies in Movies and leaves the remaining content in Media. These are example folder names, not required paths.

If an assigned Folder is renamed or deleted, its Assignment becomes **Missing**. For a rename, choose **Re-point to…** and select the new Folder to preserve its history. Removing an Assignment or deleting a Category changes organisation only; the app does not move or delete your files. Deleting a Group moves its Categories to the top level.

### Explorer

**What it holds** shows nested Folders and files sized by Size. **On disks** shows where a Folder's bytes live; **History** shows its daily Size. **Largest files** lists the largest physical files anywhere below the selected Folder, counting each once even when it has several hardlinks.

## Understanding the numbers

| Term | What it means |
| --- | --- |
| **Size** | Allocated file bytes, counting each physical file once within a Folder or Category. Sparse files can be much smaller than their apparent length. |
| **Used space / Free space** | Live filesystem figures for data disks and cache pools. Parity disks are shown separately and do not add usable storage. |
| **Shared bytes** | Bytes physically hardlinked into more than one place. Two Folders or Categories can each include the same bytes in their own Size. |
| **Rank** | Where Categories share bytes, the higher-ranked Category owns them in the Overview's split of Used space. |
| **Uncategorized** | Scanned bytes not covered by any Assignment. |
| **Unaccounted** | Used space not explained by the Scan, such as filesystem overhead or unreadable content. It cannot be negative. |

The Overview's parts add up to Used space. Folder and Category Sizes come from the latest Scan, while Used space is live. If scanned bytes exceed Used space, for example after deletions or because of filesystem compression, the Overview scales its parts proportionally to fit. Explorer and Categories still show their scanned Sizes.

### Hardlinks and seeding

Hardlinks are recognised from filesystem metadata, without reading file contents. Multiple names for one physical file count once within a Folder or Category.

There is one special attribution rule for a Folder named exactly **`torrents`**, at any depth. If a file is hardlinked both there and outside every `torrents` Folder, its Size counts toward the non-torrents Folder. The seeding name stays visible in Explorer as a dashed row or hatched tile, with a link to where it counts. A file found only under `torrents` still counts there. The match is case-sensitive: `Torrents` and `my-torrents` do not trigger it.

### History and forecasts

- History starts with the first successful Scan; earlier days cannot be reconstructed. A dash in a Change column means there is not enough history for the selected period.
- There is one value per local day. The last successful Scan of that day wins, and days without scans carry the previous value forward. More frequent scans do not create hourly history.
- Category history is recalculated using today's Assignments and Rank, so reorganising Categories changes how their past is presented too. A Group's Size and history include its Categories, counting each physical file once.
- Growth is fitted to daily total Used space over up to the last 90 days, with at least seven daily points. The fill-up estimate appears only for a rising trend. It is an estimate based on past growth, not a reservation of future storage.
- Set `TZ` before collecting history. Changing it later affects which day new scans belong to; stored days keep their existing dates.

## Updates, backups and health

### Update

With Compose:

```sh
docker compose pull
docker compose up -d
```

With `docker run`, pull `ghcr.io/marcowich0/hoardboard:latest`, stop and remove the old container, then repeat the run command with the **same `/data` bind mount**. Back up the data folder first. Database migrations run automatically at startup.

CI publishes `latest` and `sha-<short-commit>` image tags after a successful build of `main`. Use a commit tag if you want to pin a deployment. An older image may refuse a database migrated by a newer one; keep a pre-update backup if you need to roll back.

### Back up and restore

The data folder contains Categories, Assignments, scan state and history. For a simple consistent backup, stop the container and copy the **whole data folder** to a new backup location:

```sh
docker stop hoardboard
sudo cp -a /mnt/user/appdata/hoardboard "./hoardboard-backup-$(date +%Y%m%d-%H%M%S)"
docker start hoardboard
```

SQLite uses WAL mode, so copying only `hoardboard.db` while the app runs can miss committed changes. To restore, stop the container, move the current data folder aside, restore the backed-up folder at the configured bind-mount source, ensure root can write it, and start the container. For a rollback, restore the pre-update backup together with its matching image. Keep backups private: the database contains folder and file names even though it contains no scanned file contents.

### Health and logs

The image includes a Docker healthcheck. You can also check it manually:

```sh
docker exec hoardboard hoardboard healthcheck
curl --fail "http://<unraid-host>:8098/api/health"
docker logs --tail 100 hoardboard
```

`/api/health` reports the server status and build revision, and checks database connectivity and the presence of the mount root and status folder. It is not proof that a Scan completed or that every Disk is readable; check scan status in the header as well.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Scan is skipped | The header gives the reason. Wait for Unraid's array work to finish and try **Scan now**, or let the next scheduled Scan run. Paused parity operations also prevent new scans. |
| No Disks or Shares appear | Confirm the array is started, the status mount exposes Unraid's actual `disks.ini`, and `/mnt` contains the Disk folders named in that inventory. Do not mount only `/mnt/user`. |
| A Folder is empty or missing bytes | Keep `DAC_READ_SEARCH` enabled and check the logs for unreadable Folders. Symlinks are not followed and nested mount points are intentionally excluded. |
| Cannot create or write `hoardboard.db` | Verify that `/data` is mounted read-write and its host folder is owned by root and writable by root. Do not solve this by enabling privileged mode. |
| Folder sizes lag behind Free space | Used space and Free space are live; Folder sizes change after a successful Scan. Use **Scan now**. |
| Growth, forecast or Change is blank | Let daily history accumulate. The forecast needs at least a week; a period's Change needs history reaching back that far. A flat or falling trend has no fill-up date. |
| Browser cannot connect | Check `docker logs hoardboard`, the published port and host firewall. If changing `HOARDBOARD_PORT`, update the container side of the port mapping too. |

## Development

Requirements: Linux, **Go 1.27** and **Node.js 26** with npm. Docker and `jq` are needed for the container smoke test.

```sh
git clone https://github.com/Marcowich0/hoardboard.git
cd hoardboard
cd web
npm ci
cd ..
```

For development, run the Go API against an accessible Disk mount root and Unraid status folder. Use a separate writable data folder rather than a running deployment's database:

```sh
HOARDBOARD_MOUNT_ROOT=/absolute/path/to/disks \
HOARDBOARD_STATUS_DIR=/absolute/path/to/emhttp \
HOARDBOARD_DATA_DIR=/absolute/path/to/dev-data \
  go run ./cmd/hoardboard
```

In a second terminal:

```sh
cd web
npm run dev
```

Open the URL Vite prints. It proxies `/api` to `http://localhost:8098`. A plain Go build embeds a stub page; build with `webdist` to include the real app:

```sh
cd web
npm run build
cd ..
go build -tags webdist ./cmd/hoardboard
```

### Checks

```sh
# Go formatting: must print nothing.
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
TMPDIR=/dev/shm go test -race -gcflags='modernc.org/sqlite/lib=-race=false -d=checkptr=0' ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

# Web lint, types, tests, production build and dependency audit.
cd web
npm run lint
npm run typecheck
npm test
npm run build
npm audit --audit-level=high
cd ..

# Build and test the deployment image over a synthetic fixture.
docker build --build-arg REVISION="$(git rev-parse HEAD)" -t hoardboard:dev .
scripts/smoke-test.sh hoardboard:dev "$(git rev-parse HEAD)"
```

For a quicker Go test run, use `go test ./...`. CI runs the race-enabled suite, web checks, vulnerability checks and container smoke test on GitHub-hosted runners. Successful builds on `main` publish the tested image to GHCR.

See [GLOSSARY.md](GLOSSARY.md) for domain terms, [docs/testing.md](docs/testing.md) for test conventions, [architecture decisions](docs/adr/) for design rationale, and [AGENTS.md](AGENTS.md) for contributor rules and the code layout.
