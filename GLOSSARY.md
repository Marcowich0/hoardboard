# hoardboard

A dashboard for the Unraid server Poseidon that shows what its storage is used for, how that changes over time, and how much space is left.

## Storage

**Disk**:
One array data disk (`disk1`…`disk5`) or the cache pool, each with its own filesystem.
_Avoid_: Drive, device, volume

**Share**:
A top-level folder that Unraid merges across disks into one tree (`data`, `appdata`, `Photos`, …).
_Avoid_: Volume, root

**Folder**:
A directory inside a share, identified by its path relative to the merged tree, regardless of which disks hold its contents.
_Avoid_: Directory, path, node

**Size**:
The bytes a folder's files occupy, counting each physical file once even if it appears under several names inside the folder.
_Avoid_: Usage, apparent size

**Shared bytes**:
Bytes that count toward two places at once because the same physical file is hardlinked into both (typically `torrents/` and `media/`).
_Avoid_: Duplicates, double counting

**Used space** / **Free space**:
What the disks' filesystems themselves report as occupied and available; the source of truth for totals.
_Avoid_: Capacity (means total size only)

**Unaccounted**:
Used space that no scan sees: filesystem overhead, plus anything outside the shares or not readable.
_Avoid_: Other, overhead, system

## Organising

**Category**:
A named, coloured group of folders from any share, defined by the owner, such as Movies or TV.
_Avoid_: Type, tag, group, label

**Rank**:
A category's position in the owner's ordering; where categories share bytes, the higher-ranked one owns them in any view that splits used space into parts.
_Avoid_: Priority, weight

**Assignment**:
The link putting one folder into one category; a folder has at most one, and the most specific assignment wins over an ancestor's.
_Avoid_: Mapping, membership

**Missing**:
An assignment whose folder the latest scan no longer finds, usually because it was renamed. Re-pointing it moves it to the folder's new path, which takes over the old path's history for the days before it appeared.
_Avoid_: Broken, orphaned, stale

**Uncategorized**:
The automatic category holding every byte not covered by an assignment.
_Avoid_: Other, unassigned

## Time

**Scan**:
One pass that reads the metadata (names and sizes, never contents) of every folder on every disk; skipped while Unraid runs a parity check, a rebuild or the mover.
_Avoid_: Crawl, index, refresh

**History**:
The recorded size of every folder over time, at one point per day.
_Avoid_: Snapshots, timeline, log
