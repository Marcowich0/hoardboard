# Sizes come from a periodic full metadata scan of each disk

Folder sizes come from a full, lowest-I/O-priority metadata walk of each array disk and the cache pool (`/mnt/diskN`, `/mnt/cache`), merged into share paths, instead of walking the `/mnt/user` FUSE mount or watching for changes with inotify/fanotify. Free and used space are read live from the filesystems, which costs no disk I/O.

## Considered Options

- **Walk `/mnt/user`**: simpler paths, but the FUSE layer hides real inode numbers (so hardlinks can't be counted once), adds overhead, and loses the per-disk breakdown.
- **inotify/fanotify + incremental updates**: almost no I/O after the first scan, but inotify on the FUSE mount misses writes made through `/mnt/diskN`, fanotify needs a privileged container, and both add much more code. Measured 2026-11-06: `data` is about 203k entries and `appdata` about 127k, and walking `appdata` took about 2 s, so a full walk is cheap.

## Consequences

Disks on Tower never spin down (`spindownDelay=0`), so the cost to keep low is I/O contention with Plex and seeding, not spin-ups. If the file count grows by orders of magnitude, revisit incremental scanning.
