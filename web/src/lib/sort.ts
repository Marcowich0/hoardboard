import type { Entry } from '../api/types.ts'

export type SortKey = 'name' | 'size' | 'items' | 'change'
export type SortDir = 'asc' | 'desc'

const collator = new Intl.Collator('en', { sensitivity: 'base', numeric: true })

function valueOf(entry: Entry, key: 'size' | 'items' | 'change'): number | null {
  if (key === 'size') return entry.size
  if (entry.kind === 'file') return key === 'items' ? 1 : null
  return entry[key]
}

/**
 * The Explorer table's order. Folders and files sort together; a change
 * nobody knows (History too short, or a file) goes last either way. Ties
 * fall back to Size, largest first.
 */
export function sortEntries(entries: Entry[], key: SortKey, dir: SortDir): Entry[] {
  const sign = dir === 'asc' ? 1 : -1
  return [...entries].sort((a, b) => {
    if (key === 'name') return sign * collator.compare(a.name, b.name)
    const va = valueOf(a, key)
    const vb = valueOf(b, key)
    if (va === null || vb === null) {
      if (va !== vb) return va === null ? 1 : -1
      return b.size - a.size
    }
    return sign * (va - vb) || b.size - a.size
  })
}
