#!/usr/bin/env bash
# Container smoke test (Seam 2 in spec #1): starts IMAGE the way the owner's
# compose file does (every capability dropped except DAC_READ_SEARCH, a
# read-only root filesystem, /mnt and the Unraid status folder mounted
# read-only) over a tiny fixture, and checks GET /api/health.
#
#   scripts/smoke-test.sh IMAGE [REVISION]
#
# With REVISION, /api/health must also report that commit. Used by CI and
# runnable locally after `docker build -t hoardboard:dev .`.
set -euo pipefail

image=${1:?usage: smoke-test.sh IMAGE [REVISION]}
want_revision=${2:-}
name=hoardboard-smoke-$$-$RANDOM
fixture=$(mktemp -d)

cleanup() {
  status=$?
  if [ "$status" -ne 0 ]; then
    echo "--- container logs ---" >&2
    docker logs "$name" >&2 || true
  fi
  docker rm -f -v "$name" >/dev/null 2>&1 || true
  rm -rf "$fixture"
  exit "$status"
}
trap cleanup EXIT

# A tiny fake Unraid: two data disks and a cache pool with share folders, and
# the status files emhttpd writes.
mkdir -p "$fixture/mnt/disk1/data/media/movies" "$fixture/mnt/disk2/data/media/tv" \
  "$fixture/mnt/cache/appdata/hoardboard" "$fixture/emhttp"
head -c 1048576 /dev/urandom > "$fixture/mnt/disk1/data/media/movies/a.mkv"
head -c 524288 /dev/urandom > "$fixture/mnt/disk2/data/media/tv/b.mkv"
head -c 4096 /dev/urandom > "$fixture/mnt/cache/appdata/hoardboard/c.db"
section() { # name idx type mounted
  printf '["%s"]\nname="%s"\nidx="%s"\ndevice="sd%s"\nsize="3907018532"\nstatus="DISK_OK"\ntype="%s"\n' "$1" "$1" "$2" "$2" "$3"
  if [ "$4" = yes ]; then printf 'fsStatus="Mounted"\nfsMountpoint="/mnt/%s"\n' "$1"; fi
}
{
  section parity 0 Parity no
  section disk1 1 Data yes
  section disk2 2 Data yes
  section cache 30 Cache yes
} > "$fixture/emhttp/disks.ini"
printf 'mdState="STARTED"\nfsState="Started"\nmdResync="0"\nmdResyncPos="0"\nmdResyncAction="check P"\nshareMoverActive="no"\n' \
  > "$fixture/emhttp/var.ini"
chmod -R a+rX "$fixture"

docker run -d --name "$name" \
  --cap-drop ALL --cap-add DAC_READ_SEARCH \
  --security-opt no-new-privileges:true \
  --read-only --tmpfs /tmp \
  -p 127.0.0.1::8098 \
  -v "$fixture/mnt:/mnt:ro" \
  -v "$fixture/emhttp:/var/local/emhttp:ro" \
  "$image" >/dev/null

port=$(docker port "$name" 8098/tcp | head -n1 | sed 's/.*://')
url=http://127.0.0.1:$port

for _ in $(seq 1 30); do
  status=$(docker inspect -f '{{.State.Health.Status}}' "$name")
  [ "$status" = healthy ] && break
  if [ "$(docker inspect -f '{{.State.Running}}' "$name")" != true ]; then
    echo "container exited" >&2
    exit 1
  fi
  sleep 1
done
if [ "$status" != healthy ]; then
  echo "container did not become healthy (last: $status)" >&2
  exit 1
fi

health=$(curl -fsS "$url/api/health")
echo "health: $health"
case "$health" in
  *'"status":"ok"'*) ;;
  *) echo "health is not ok" >&2; exit 1 ;;
esac
if [ -n "$want_revision" ]; then
  case "$health" in
    *"\"revision\":\"$want_revision\""*) ;;
    *) echo "health does not report revision $want_revision" >&2; exit 1 ;;
  esac
fi

# The embedded app answers its client routes.
for path in / /categories /explorer; do
  curl -fsS "$url$path" | grep -q '<div id="root">' || { echo "GET $path is not the app" >&2; exit 1; }
done

echo "smoke test passed"
