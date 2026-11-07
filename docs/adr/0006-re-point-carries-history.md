# Re-pointing records a rename that every History read follows

When the owner re-points a Missing Assignment from `old` to `new`
(`POST /api/assignments/repoint`), the Assignment moves to `new`, and History
records a rename in `renames (path, was, day)`: the Folder at `new` was the
Folder at `old` before `day`. `day` is when `new` appeared: its first row
after its last row of zeros, or its first row. A rename covers the subtree
too: `new/x` was `old/x`. Nothing in `folder_history` or `link_history` is
copied or rewritten. The old path keeps its own History.

Reads follow the renames (`history.Renames.At(path, day)`, the path a Folder
had on a day):

- **A Folder's History** (`history.Store.Read`, so `GET /api/history?path=`,
  the Explorer's change over a period and a category's past) is spliced: the
  rows of the path it had on each day.
- **A category's past** (`category.MeasurePast`) measures each day under the
  Assignments moved back to the paths their Folders had that day. Link sets,
  recorded by the paths names had then, then meet the right category with
  no change to the engine or to the link sets.

Renames are applied in the order they took effect (`day`, then the order
they were made). Going back in time, a rename leads to the old path, where
only earlier renames apply, so a Folder renamed and renamed back resolves
correctly and the lookup always ends.

## Why

- Spec #1: re-pointing must keep the category's past, and the old Folder's
  History stays on record. Copying rows to the new path would keep two
  copies, and copying link sets would mean rewriting their keys.
- One rule (`Renames.At`) serves the Folder reads and the category engine,
  so a category's past and its Folders' History can't disagree.
- Taking `day` from `new`'s History, not from the day of the re-point, means
  an owner who notices the rename a week later gets the same past as one
  who notices at once.

## Consequences

- Re-pointing to a Folder that existed long before the old one disappeared
  carries nothing over: before `day`, the new path is itself.
- A rename record is permanent. Re-pointing the same path again adds a
  newer record, which wins for the days before its `day`.
- If the old path is assigned again later, a renamed Folder's Assignment
  takes the old path's past on the days before the rename, and the old
  path's Assignment gets nothing there.
- Each History read costs one more small query (`renames`).
