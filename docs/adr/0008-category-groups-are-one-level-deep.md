# Category groups are one level deep

A category may be in one other category, its group (Movies and TV in Media):
`categories.group_id`, NULL at the top level. A group is still a category of
its own: it can have Assignments (`data/media` in Media, `data/media/movies`
in Movies; the most specific Assignment wins as before), a colour and a Rank.
A category with others in it can't go into a group, and a category in a group
can't hold others: groups are one level deep.

Ranks always run in group order (`category.normalize`): each top-level
category, then the categories in it. Creating, moving, reordering and
deleting all renumber to that order, so a group's categories always stand
next to each other, under it, and the flat Rank the engine splits Used space
by never changes meaning.

A group's Size, Shared bytes and History cover the categories in it, each
physical file once (`Sizes.Size(ids...)`, `Sizes.SharedWith(ids...)`). The
donut and the trend stay one part per category: a group's own part is its
own folders only ("Other Media"), and the web app adds the group up from its
parts.

## Why

- The owner groups media (Movies, TV, Audiobooks, Music) and wants to watch
  the media stack as one thing as well as each part.
- One level covers that, and keeps the Rank list draggable (a group moves as
  a block, its categories move within it) and every chart readable: an inner
  ring of categories, an outer ring of groups. Deeper trees would need a
  tree editor and a sunburst for little gain.
- Keeping one flat Rank in group order means shared bytes still go to the
  highest-ranked category, with no second ordering to explain.

## Consequences

- Deleting a group moves its categories to the top level, where it stood
  (`ON DELETE SET NULL`, then renumbering).
- A reorder that tears a group apart is put back in group order rather than
  refused, so the web app can send any order.
