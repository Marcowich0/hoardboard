# Scanned bytes shrink to fit Used space

The donut and each day of the stacked trend split Used space into the
categories, Uncategorized and Unaccounted (`category.Sizes.Split`). Used
space comes live from the filesystems (or, for a past day, from that day's
last Scan); the other parts come from a Scan's tree. When the tree holds more
bytes than Used space, the categories and Uncategorized are scaled down in
proportion so that they add up to Used space exactly, and Unaccounted is 0.

The scaling is in integers: each part times `Used / scanned`, rounded down,
and the bytes that rounding leaves over go to the largest part (the first of
them, in Rank order then Uncategorized, on a tie). A day with no recorded
Used space is not scaled.

## Why

- Spec #1 requires both that the slices "always add up exactly to Used
  space" and that Unaccounted is floored at zero. Flooring alone breaks the
  first rule whenever scanned > Used.
- That happens for real: the tree is older than live Used space (files
  deleted since the Scan), and on btrfs `st_blocks` counts compressed data
  at its full size, so a compressed cache pool scans larger than it is.
- Scaling every part by the same factor keeps the categories' proportions,
  which is what the donut is read for. Giving the whole excess to one part
  (say Uncategorized) would invent a shrinkage that part never had.

## Consequences

- In that case a slice shows a little less than the category's Size in the
  Explorer and in Categories, which stay as scanned. The donut is about
  Used space; the Sizes are about files.
- Unaccounted can't go negative and no slice ever does either.
