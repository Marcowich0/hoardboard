import { useMemo } from 'react'
import type { UsedSplit } from '../api/index.ts'
import { type CategoryBook, useCategories } from '../categories.ts'

export interface Part {
  /** A category id, null for Uncategorized, "unaccounted" for Unaccounted. */
  key: number | null | 'unaccounted'
  name: string
  colour: string
  hatch: boolean
  bytes: number
  /** The group the part belongs to (its category's, or its own if it is a group); null for none. */
  group: number | null
  /** For a group's own part (named "Other Media"), the group's name ("Media"). */
  groupName?: string
}

/**
 * A top-level part of Used space: a category on its own, or a group with
 * its parts (its own folders first, then each category in it), or
 * Uncategorized or Unaccounted.
 */
export interface PartGroup {
  key: Part['key']
  name: string
  colour: string
  hatch: boolean
  /** All its parts together. */
  bytes: number
  /** For a group, its parts in Rank order; null for anything else. */
  members: Part[] | null
}

/**
 * The donut's parts in Rank order, then Uncategorized and Unaccounted. A
 * group's own part, the folders assigned to the group itself, is named
 * "Other <group>".
 */
export function splitParts(split: UsedSplit, book: CategoryBook): Part[] {
  return [
    ...split.categories.map((c): Part => {
      const look = book.look(c.categoryId)
      const part = { key: c.categoryId, ...look, bytes: c.bytes, group: book.groupOf(c.categoryId)?.id ?? null }
      return book.membersOf(c.categoryId).length > 0
        ? { ...part, name: `Other ${look.name}`, group: c.categoryId, groupName: look.name }
        : part
    }),
    { key: null, ...book.look(null), bytes: split.uncategorized, group: null },
    { key: 'unaccounted' as const, ...book.unaccounted, bytes: split.unaccounted, group: null },
  ]
}

/**
 * The parts gathered into their groups, in Rank order. A group's own part
 * is left out of its members while it is empty: most groups only gather
 * their categories.
 */
export function groupParts(parts: Part[]): PartGroup[] {
  const out: PartGroup[] = []
  for (const p of parts) {
    const last = out.at(-1)
    if (p.group !== null && p.group !== p.key && last?.key === p.group && last.members) {
      last.members.push(p)
      last.bytes += p.bytes
      continue
    }
    if (p.group !== null && p.group === p.key) {
      out.push({ ...p, name: p.groupName ?? p.name, members: p.bytes > 0 ? [p] : [] })
      continue
    }
    out.push({ ...p, members: null })
  }
  return out
}

export function useParts(split: UsedSplit): Part[] {
  const book = useCategories()
  return useMemo(() => splitParts(split, book), [book, split])
}

export function useGroupedParts(split: UsedSplit): PartGroup[] {
  const parts = useParts(split)
  return useMemo(() => groupParts(parts), [parts])
}
