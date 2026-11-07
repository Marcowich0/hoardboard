import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import type { Category } from '../api/index.ts'
import { makeBook } from '../categories.ts'
import { groupParts, splitParts } from './parts.ts'

const category = (id: number, name: string, groupId: number | null = null): Category => ({
  id,
  name,
  colour: '#2a78d6',
  rank: id,
  groupId,
  size: 0,
  sharedWith: [],
  assignments: [],
})

describe('groupParts', () => {
  const book = makeBook(
    {
      categories: [category(1, 'Media'), category(2, 'Movies', 1), category(3, 'TV', 1), category(4, 'Backups')],
      uncategorized: { size: 0 },
    },
    'dark',
  )
  const parts = splitParts(
    {
      categories: [
        { categoryId: 1, bytes: 1 },
        { categoryId: 2, bytes: 10 },
        { categoryId: 3, bytes: 20 },
        { categoryId: 4, bytes: 5 },
      ],
      uncategorized: 7,
      unaccounted: 3,
    },
    book,
  )

  test('gathers a group with its own folders and its categories, and adds them up', () => {
    const groups = groupParts(parts)
    assert.deepEqual(
      groups.map((g) => [g.key, g.name, g.bytes, g.members?.map((m) => m.name) ?? null]),
      [
        [1, 'Media', 31, ['Other Media', 'Movies', 'TV']],
        [4, 'Backups', 5, null],
        [null, 'Uncategorized', 7, null],
        ['unaccounted', 'Unaccounted', 3, null],
      ],
    )
  })

  test('leaves out a group\'s own part while it is empty', () => {
    const empty = splitParts({ categories: [{ categoryId: 1, bytes: 0 }, { categoryId: 2, bytes: 4 }], uncategorized: 0, unaccounted: 0 }, book)
    assert.deepEqual(
      groupParts(empty)[0].members?.map((m) => m.name),
      ['Movies'],
    )
  })

  test('marks each part with its group, and names a group\'s own part', () => {
    assert.deepEqual(
      parts.map((p) => [p.name, p.group]),
      [
        ['Other Media', 1],
        ['Movies', 1],
        ['TV', 1],
        ['Backups', null],
        ['Uncategorized', null],
        ['Unaccounted', null],
      ],
    )
  })
})
