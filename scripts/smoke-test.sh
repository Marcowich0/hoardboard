#!/usr/bin/env bash
# Container smoke test (Seam 2 in spec #1): starts IMAGE the way the owner's
# compose file does (every capability dropped except DAC_READ_SEARCH, a
# read-only root filesystem, /mnt and the Unraid status folder mounted
# read-only) over a tiny fixture, and checks GET /api/health, GET /api/overview,
# a Scan and the tree it makes (a Folder merged across Disks, a root-only
# folder counted, a nested mount not crossed), and the app's routes.
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
  chmod -R u+rwX "$fixture" 2>/dev/null || true
  rm -rf "$fixture"
  exit "$status"
}
trap cleanup EXIT

# A tiny fake Unraid: two data disks and a cache pool with share folders, and
# the status files emhttpd writes. data/media/movies is on disk1 and disk2;
# appdata/private is root-only; data/elsewhere is where another filesystem
# gets mounted, which a Scan must not enter; data/media/loop is a symlink.
mkdir -p "$fixture/mnt/disk1/data/media/movies" "$fixture/mnt/disk2/data/media/tv" \
  "$fixture/mnt/disk2/data/media/movies" "$fixture/mnt/cache/appdata/hoardboard" \
  "$fixture/mnt/cache/appdata/private" "$fixture/mnt/disk1/data/elsewhere" \
  "$fixture/elsewhere" "$fixture/emhttp"
head -c 1048576 /dev/urandom > "$fixture/mnt/disk1/data/media/movies/a.mkv"
head -c 262144 /dev/urandom > "$fixture/mnt/disk2/data/media/movies/c.mkv"
head -c 524288 /dev/urandom > "$fixture/mnt/disk2/data/media/tv/b.mkv"
head -c 4096 /dev/urandom > "$fixture/mnt/cache/appdata/hoardboard/c.db"
head -c 8192 /dev/urandom > "$fixture/mnt/cache/appdata/private/secret.db"
head -c 65536 /dev/urandom > "$fixture/elsewhere/big.img"
ln -s /mnt "$fixture/mnt/disk1/data/media/loop"
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
# Only CAP_DAC_READ_SEARCH lets the container list it.
chmod 000 "$fixture/mnt/cache/appdata/private"

docker run -d --name "$name" \
  --cap-drop ALL --cap-add DAC_READ_SEARCH \
  --security-opt no-new-privileges:true \
  --read-only --tmpfs /tmp \
  -p 127.0.0.1::8098 \
  -v "$fixture/mnt:/mnt:ro" \
  -v "$fixture/elsewhere:/mnt/disk1/data/elsewhere:ro" \
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

# The overview lists the fixture's Disks with live figures from their
# filesystems (statfs works with every capability but DAC_READ_SEARCH
# dropped), and parity apart from them.
overview=$(curl -fsS "$url/api/overview")
echo "overview: $overview"
for disk in disk1 disk2 cache; do
  case "$overview" in
    *"\"name\":\"$disk\""*) ;;
    *) echo "overview does not list $disk" >&2; exit 1 ;;
  esac
done
case "$overview" in
  *'"parity":[{"name":"parity",'*) ;;
  *) echo "overview does not list parity" >&2; exit 1 ;;
esac

# A Scan, waited for (the schedule started one at startup; this joins it or
# runs another), and the tree it makes.
scan=$(curl -fsS -X POST "$url/api/scan?wait=true")
echo "scan: $scan"
jq -e '.started and .status.last != null and .status.running == null and .status.next != null' \
  <<<"$scan" >/dev/null || { echo "the scan did not finish, or none is scheduled" >&2; exit 1; }

# One Folder data/media/movies, its bytes on disk1 and disk2.
media=$(curl -fsS "$url/api/tree?path=data/media")
echo "tree data/media: $media"
jq -e '.children[] | select(.name == "movies")
  | .kind == "folder" and .size == 1310720
    and .disks == [{"disk": "disk1", "bytes": 1048576}, {"disk": "disk2", "bytes": 262144}]' \
  <<<"$media" >/dev/null || { echo "data/media/movies is not one Folder over disk1 and disk2" >&2; exit 1; }
# The symlink is listed, not followed.
jq -e '.folder.size == 1835008 and ([.children[] | select(.name == "loop")][0].kind == "file")' \
  <<<"$media" >/dev/null || { echo "data/media: wrong size, or the symlink was followed" >&2; exit 1; }

# The root-only folder is counted.
private=$(curl -fsS "$url/api/tree?path=appdata/private")
echo "tree appdata/private: $private"
jq -e '.folder.size == 8192 and .children[0].name == "secret.db"' <<<"$private" >/dev/null \
  || { echo "the root-only folder appdata/private was not counted" >&2; exit 1; }

# The filesystem mounted at data/elsewhere is not walked.
data=$(curl -fsS "$url/api/tree?path=data")
echo "tree data: $data"
jq -e '[.children[].name] == ["media"] and .folder.size == 1835008' <<<"$data" >/dev/null \
  || { echo "the scan crossed into the mount at data/elsewhere" >&2; exit 1; }

# While the mover runs (var.ini says so, read afresh), a Scan is skipped.
sed 's/^shareMoverActive="no"$/shareMoverActive="yes"/' "$fixture/emhttp/var.ini" >"$fixture/emhttp/var.ini.new"
mv "$fixture/emhttp/var.ini.new" "$fixture/emhttp/var.ini"
skipped=$(curl -fsS -X POST "$url/api/scan?wait=true")
echo "scan during the mover: $skipped"
jq -e '.started == false and .status.skipped.reason == "Mover is running"' <<<"$skipped" >/dev/null \
  || { echo "the scan was not skipped while the mover ran" >&2; exit 1; }

# The embedded app answers its client routes.
for path in / /categories /explorer; do
  curl -fsS "$url$path" | grep -q '<div id="root">' || { echo "GET $path is not the app" >&2; exit 1; }
done

echo "smoke test passed"
