# A category's past is measured again from History under today's Assignments

A category's History (`GET /api/history?category=`) and the Overview's
stacked trend are not stored. Each request rebuilds every day from the first
Scan to today out of History, and measures it with the same category engine
that measures the latest Scan (`category.MeasureBytes`), under the
**current** Assignments and Rank. To make that possible, History stores two
more things per day, with the same write-on-change rule as a Folder's Size
(ADR 0003, `history.changeOn`):

- **`folder_history.single_bytes`**: each Folder's SingleBytes, the part of
  its Size in files with one name. Unlike Size, it adds up over subfolders.
  So a category's share of single-name files on any day is its assigned
  Folders' SingleBytes minus that of the Assignments nearest below them.
- **`link_sets` and `link_history`**: the hardlinked files, grouped by the set
  of Folders their names are in. A row holds that set's bytes on a day; a set
  that disappears gets a zero row, as a Folder does. The engine needs only
  the Folder of each name to tell which categories share a file, so files
  with the same set are summed into one row.

Today's Used space is the live one, so the trend's last day is the donut. A
past day's Used space is the total that day's last Scan recorded
(`history.Store.TotalUsed`, from ticket #15), and that day's
Unaccounted is `max(0, Used − scanned)`. Before Used space was recorded, a
day's Unaccounted is 0.

## Why

- Spec #1 wants reassigning a folder to change the category's past too. So
  the past can't be stored per category: it has to come from per-Folder data.
- A Folder's Size and Shared bytes can't tell which files two Folders share,
  so summing Sizes would count a file hardlinked into two of a category's
  Folders twice, and Shared bytes alone can't say which category owns a file
  in the Rank split. SingleBytes plus the hardlinked files is exactly the
  input the engine already used for the latest tree, so the past and today
  are measured by one piece of code and can't drift apart.
- It stays small. Tower has about 570 hardlinked files, mostly one per
  `media`/`torrents` folder pair, so a few hundred sets. Most change on few
  days. SingleBytes changes when Size does.

## Consequences

- A request costs one read of the assigned Folders' and the link sets'
  History, plus one engine run per day. That is fine for years of days on
  this scale. If it grows slow, cache per (Assignments, Rank, last Scan).
- Rows written before this change have `single_bytes` 0, and those days have
  no link sets. The app had no deployment then, so no backfill exists.
- Link sets name Folders by path, and the engine reads them only through
  `Assignments.Of`. A re-pointed Assignment joins the old Folder's History
  to the new one's, link sets included: see ADR 0006.
- Changing Rank re-splits the whole trend, not only days to come.
