import { useMemo } from 'react'
import type { UsedSplit } from '../api/index.ts'
import { useCategories } from '../categories.ts'

export interface Part {
  /** A category id, null for Uncategorized, "unaccounted" for Unaccounted. */
  key: number | null | 'unaccounted'
  name: string
  colour: string
  hatch: boolean
  bytes: number
}

/** The donut's parts in Rank order, then Uncategorized and Unaccounted. */
export function useParts(split: UsedSplit): Part[] {
  const book = useCategories()
  return useMemo(() => {
    const unaccounted = book.unaccounted
    const uncategorized = book.look(null)
    return [
      ...split.categories.map((c) => ({ key: c.categoryId, ...book.look(c.categoryId), bytes: c.bytes })),
      { key: null, ...uncategorized, bytes: split.uncategorized },
      { key: 'unaccounted' as const, ...unaccounted, bytes: split.unaccounted },
    ]
  }, [book, split])
}
