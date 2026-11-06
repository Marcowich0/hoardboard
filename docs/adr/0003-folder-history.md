# History is a row per Folder per changed day, in the server's local day

Every Folder's History is stored as rows of `(folder, day, Size, Shared bytes)`
in `folder_history`, with paths stored once in `folders`. A Scan writes the
day it finished: a Folder's row for that day is written only when its value
differs from the Folder's previous stored day, and removed again if a later
Scan that day brings it back to that value, so the last Scan of a day wins and
an unchanged day has no row. Reads carry the last stored value forward
(`history.Series`). A Folder a Scan no longer finds gets a row of zeros, once;
its earlier rows are kept forever.

"Today" is the day the injected clock's now falls on in the server's local
time zone (`TZ`; the binary embeds tzdata), the same zone the overview's
`today` and parity `lastCheck` use. Days are stored as days since 1970-01-01.

## Why

- Most Folders don't change on most days (about 2.5k Folders in `data`, many
  more in `appdata`, on Tower), so storing only changes keeps the database
  small without losing a day.
- `(folder_id, day)` is the primary key of a `WITHOUT ROWID` table, so one
  Folder's rows in day order are one index range, and a category's History
  over many Folders (`history.Store.Read`) is one query.
- The zero row when a Folder disappears makes carrying forward correct: a
  deleted Folder reads as empty from that day, not as its last Size forever,
  and a re-pointed Assignment can join the old Folder's History up to that day
  with the new Folder's from it.
- The owner thinks in local days: a Scan just after local midnight belongs
  to the new day, whatever UTC says. UTC days would put the evening's Scans
  into the next day east of Greenwich.

## Consequences

- The change over a period is `Size today - Size on (today - 7, 30 or 365)`,
  null when that day is before History began (the first Scan's day, the
  root's first row). A Folder created since then changed by its whole Size.
- Changing `TZ` shifts which day later Scans fall on; days already stored
  stay as they are.
- History has one value per day, so the changes within a day are lost; the
  last Scan's value is kept.
