# Size is allocated blocks of files; a Scan stops at mount points, not at st_dev changes

A file's Size is the space it occupies, `st_blocks*512`, not its apparent size (`st_size`), and a Folder's Size is the sum over its files. Folders' own blocks are not counted: they are filesystem overhead and fall into Unaccounted. A Scan stays on a Disk's filesystem by not entering mount points listed in `/proc/self/mountinfo`; it does not stop where `st_dev` changes.

The latest tree is saved as one compressed blob (`snapshot`, one row, `tree.Encode`'s format) rather than one row per entry: it is only ever loaded whole, at startup.

## Why

- Allocated blocks are what `du` and the filesystem's Used space count, so `Unaccounted = Used - scanned` stays meaningful. On Tower, the cache pool's sparse VM disks take 347.5 GB but are 429.5 GB apparent; apparent sizes would exceed real usage and the zero floor on Unaccounted would hide it.
- Leaving folders' own blocks out keeps Size to "the bytes a folder's files occupy" (GLOSSARY.md), and the same on every filesystem (an ext4 folder takes 4 KiB, a tmpfs one none), so tests can expect literal sizes.
- The cache pool is btrfs, and each VM subvolume under `domains/` has its own `st_dev`. A `st_dev` boundary would drop them silently. Mount points are the real boundary.

## Consequences

- A sparse or partly downloaded file shows the space it takes, which can be far below its length.
- Hardlink identity must include `st_dev` as well as the Disk, since each btrfs subvolume has its own inode numbers.
- Changing the snapshot format means adding a version to `internal/tree/codec.go`; older snapshots must still load (or be dropped and rebuilt by the next Scan).
