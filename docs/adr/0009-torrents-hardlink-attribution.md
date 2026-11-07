# Torrents hardlinks count toward their non-torrents Folders

When a Scan sees names for one physical file both under a Folder named
exactly `torrents` (at any depth, including a Share) and outside every such
Folder, only the names outside torrents contribute to Folder and Category
Size, bytes per Disk, and the Rank split of Used space. The match is case
sensitive, like Linux names; `my-torrents` and `Torrents` are unrelated.
Files found only under torrents count there as usual, including hardlinks
whose other names the Scan cannot see. No file contents are read.

## Why

The arr stack imports files by hardlinking them from torrents into the
media library. The library is where those bytes are used; changing Category
Rank should not make them count as torrents again. Other hardlinks retain
their existing behaviour, including Rank deciding between multiple
non-torrents Categories.

## Consequences

- `tree.CountingPaths` is the common rule for the latest tree and Category
  History. Raw names stay in snapshots and History link sets, so no format
  or schema changes are needed. Loading an old snapshot recomputes the new
  attribution. Category History applies the rule to existing link sets too.
- Folder History keeps figures already recorded. New Scans record the new
  Size; an upgrade can therefore show a one-time drop in a torrents Folder.
  No past Folder History is rewritten.
- Shared bytes remain the part of a Folder's counted Size physically linked
  outside it. A library Folder can still have Shared bytes with its seeding
  name; the torrents Folder excludes those bytes from both Size and Shared
  bytes. Categories share only bytes that count toward both of them.
- `countedElsewhereBytes` tracks excluded bytes visible below a Folder,
  each physical file once. `countedIn` on a seeding file names the first
  non-torrents Folder in tree order and its effective Category. If several
  non-torrents names exist, this is a stable navigation destination; they
  all keep their normal counting and Rank rules.
- Explorer shows seeding files with dashed rows, a destination link, and
  the destination Category colour. Inside torrents, the treemap also gives
  them visible area, pale fill and hatching; that area is separate from
  Size. Ancestor treemaps show only counted Size, so torrents does not
  inflate their tiles. Largest files prefer a non-torrents name when both
  names are below the Folder being listed.
