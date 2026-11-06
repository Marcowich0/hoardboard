# The forecast is fitted to a daily History of each Disk's Used space

The growth rate and the "full in ~N months" forecast are a least-squares line
through the array's total Used space, one point per day over the last 90 days,
omitted with fewer than 7 points or when the line is flat or falling. Spec #1's
schema has no daily series of total Used space, so each Scan also records
every Disk's Used space, as its filesystem reports it when the Scan finishes,
in `disk_history (disk, day, used)`, keyed by the Disk's name.

It follows Folder History's write rule (`docs/adr/0003-folder-history.md`,
shared as `history.changeOn`): a row only when the value changes, removed
again if the day's last Scan brings it back, so the last Scan of a day wins;
reads carry values forward. A Disk Unraid no longer lists gets a row of zeros,
once; a Disk Unraid lists but whose filesystem can't be asked keeps its rows.
The fit runs on the Disks' values added up per day, from the first recorded day
(at most 90 days back) to today, with days without a Scan carried forward.

## Why

- Folder History can't stand in for it: Unaccounted (filesystem overhead,
  anything outside the Shares) is part of Used space, and only the
  filesystem knows it.
- One value per Disk rather than one total lets the total follow Disks that
  join or leave the array, and gives the stacked trend's Unaccounted the same
  per-day Used space.
- The fit is in exact rational arithmetic: Used space is around 2^44 bytes,
  and floats would lose whole bytes a day and could tip the forecast's day.

## Consequences

- The forecast needs a week of Scans after the first; there is no backfill.
- A month is 365.25/12 days in `bytesPerMonth`.
- A Disk is identified by its name: renaming a pool starts a new series and
  ends the old one with a zero row.
