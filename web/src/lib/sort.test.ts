import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import type { Entry } from '../api/types.ts'
import { sortEntries } from './sort.ts'

const folder = (name: string, size: number, change: number | null, items: number): Entry => ({
  kind: 'folder',
  name,
  path: `x/${name}`,
  size,
  change,
  items,
  sharedBytes: 0,
  disks: [],
  categoryId: null,
})
const file = (name: string, size: number): Entry => ({
  kind: 'file',
  name,
  path: `x/${name}`,
  size,
  disk: 'disk1',
  links: 1,
  categoryId: null,
})

const entries = [folder('b', 30, 5, 2), file('a.mkv', 50), folder('C', 10, null, 9), folder('d', 20, -4, 1)]
const names = (list: Entry[]) => list.map((e) => e.name)

describe('sortEntries', () => {
  test('sorts folders and files together by Size', () => {
    assert.deepEqual(names(sortEntries(entries, 'size', 'desc')), ['a.mkv', 'b', 'd', 'C'])
    assert.deepEqual(names(sortEntries(entries, 'size', 'asc')), ['C', 'd', 'b', 'a.mkv'])
  })

  test('sorts names case-insensitively', () => {
    assert.deepEqual(names(sortEntries(entries, 'name', 'asc')), ['a.mkv', 'b', 'C', 'd'])
  })

  test('puts an unknown change last either way, files too (they have no History)', () => {
    assert.deepEqual(names(sortEntries(entries, 'change', 'desc')), ['b', 'd', 'a.mkv', 'C'])
    assert.deepEqual(names(sortEntries(entries, 'change', 'asc')), ['d', 'b', 'a.mkv', 'C'])
  })

  test('counts a file as one item; ties go to the larger', () => {
    assert.deepEqual(names(sortEntries(entries, 'items', 'desc')), ['C', 'b', 'a.mkv', 'd'])
  })
})
