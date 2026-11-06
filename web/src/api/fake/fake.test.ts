import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import type { UsedSplit } from '../types.ts'
import { createFakeApi, type FakeWorld } from './fake.ts'

const GiB = 1024 ** 3

// A small server: two data disks and the cache. A movie is hardlinked from
// torrents into media, and media/movies is spread over both disks.
function world(): FakeWorld {
  return {
    firstDay: '2026-10-01',
    disks: [
      { name: 'disk1', kind: 'data', filesystem: 'xfs', size: 100 * GiB, overhead: 1 * GiB },
      { name: 'disk2', kind: 'data', filesystem: 'xfs', size: 100 * GiB, overhead: 1 * GiB },
      { name: 'cache', kind: 'cache', filesystem: 'btrfs', size: 50 * GiB, overhead: 2 * GiB },
    ],
    parity: [{ name: 'parity', size: 100 * GiB, status: 'valid', lastCheck: '2026-10-20' }],
    files: [
      { path: 'data/media/movies/A/a.mkv', inode: 1, size: 10 * GiB, disk: 'disk1', added: '2026-10-01' },
      { path: 'data/torrents/A/a.mkv', inode: 1, size: 10 * GiB, disk: 'disk1', added: '2026-10-01' },
      { path: 'data/media/movies/B/b.mkv', inode: 2, size: 6 * GiB, disk: 'disk2', added: '2026-11-01' },
      { path: 'data/media/tv/S/e1.mkv', inode: 3, size: 2 * GiB, disk: 'disk2', added: '2026-10-01' },
      { path: 'appdata/plex/db', inode: 4, size: 3 * GiB, disk: 'cache', added: '2026-10-01' },
    ],
    categories: [
      { id: 1, name: 'Movies', colour: '#2a78d6' },
      { id: 2, name: 'Torrents', colour: '#e87ba4' },
    ],
    assignments: [
      { path: 'data/media/movies', categoryId: 1 },
      { path: 'data/torrents', categoryId: 2 },
    ],
    scan: { finishedAt: '2026-11-06T11:46:00Z', durationMs: 3000 },
  }
}

const now = () => new Date('2026-11-06T12:00:00Z')

function sum(split: UsedSplit): number {
  return split.categories.reduce((s, c) => s + c.bytes, 0) + split.uncategorized + split.unaccounted
}

describe('overview', () => {
  test('the donut sums exactly to Used space', async () => {
    const api = createFakeApi(world(), { now })
    const overview = await api.overview()
    // 10 + 6 + 2 + 3 GiB of files (the hardlink once) plus 4 GiB of overhead.
    assert.equal(overview.totals.used, 25 * GiB)
    assert.equal(sum(overview.donut), 25 * GiB)
    assert.equal(overview.donut.unaccounted, 4 * GiB)
  })
})

describe('the donut follows Rank', () => {
  test('a hardlinked file goes to the higher-ranked category', async () => {
    const api = createFakeApi(world(), { now })
    const donut = (await api.overview()).donut
    // Movies (rank 1) owns the 10 GiB movie, Torrents keeps nothing.
    assert.deepEqual(donut.categories, [
      { categoryId: 1, bytes: 16 * GiB },
      { categoryId: 2, bytes: 0 },
    ])
    assert.equal(donut.uncategorized, 5 * GiB)
  })

  test('reordering moves the Shared bytes to the new higher rank', async () => {
    const api = createFakeApi(world(), { now })
    await api.reorderCategories([2, 1])
    const donut = (await api.overview()).donut
    assert.deepEqual(donut.categories, [
      { categoryId: 2, bytes: 10 * GiB },
      { categoryId: 1, bytes: 6 * GiB },
    ])
  })
})

describe('categories', () => {
  test('a category shows its full Size and what it shares with others', async () => {
    const api = createFakeApi(world(), { now })
    const { categories, uncategorized } = await api.categories()
    const torrents = categories.find((c) => c.name === 'Torrents')
    assert.equal(torrents?.size, 10 * GiB)
    assert.deepEqual(torrents?.sharedWith, [{ categoryId: 1, bytes: 10 * GiB }])
    assert.equal(uncategorized.size, 5 * GiB)
  })

  test('the most specific Assignment wins', async () => {
    const api = createFakeApi(world(), { now })
    const media = await api.createCategory({ name: 'Media', colour: '#1baf7a' })
    await api.assign('data/media', media.id)
    const { categories } = await api.categories()
    // media/movies stays in Movies; only tv falls to Media.
    assert.equal(categories.find((c) => c.name === 'Media')?.size, 2 * GiB)
    assert.equal(categories.find((c) => c.name === 'Movies')?.size, 16 * GiB)
  })

  test('an Assignment whose folder is gone is Missing until re-pointed', async () => {
    const api = createFakeApi(world(), { now })
    await api.assign('data/media/kids', 1)
    let movies = (await api.categories()).categories[0]
    assert.deepEqual(
      movies.assignments.find((a) => a.path === 'data/media/kids'),
      { path: 'data/media/kids', size: null, missing: true },
    )
    await api.repoint('data/media/kids', 'data/media/tv')
    movies = (await api.categories()).categories[0]
    assert.deepEqual(
      movies.assignments.map((a) => a.path),
      ['data/media/movies', 'data/media/tv'],
    )
    assert.equal(movies.size, 18 * GiB)
  })

  test('names must be unique and Uncategorized is taken', async () => {
    const api = createFakeApi(world(), { now })
    await assert.rejects(api.createCategory({ name: 'movies', colour: '#000000' }), { status: 409 })
    await assert.rejects(api.createCategory({ name: 'Uncategorized', colour: '#000000' }), { status: 400 })
  })
})

describe('tree', () => {
  test('a folder spread over disks is one folder, and a hardlink counts once', async () => {
    const api = createFakeApi(world(), { now })
    const tree = await api.tree('data', '30d')
    assert.equal(tree.folder.size, 18 * GiB)
    assert.deepEqual(tree.folder.disks, [
      { disk: 'disk1', bytes: 10 * GiB },
      { disk: 'disk2', bytes: 8 * GiB },
    ])
    assert.deepEqual(
      tree.children.map((c) => [c.name, c.size]),
      [
        ['media', 18 * GiB],
        ['torrents', 10 * GiB],
      ],
    )
  })

  test('Shared bytes are what is also reachable from outside the folder', async () => {
    const api = createFakeApi(world(), { now })
    const media = (await api.tree('data', '30d')).children.find((c) => c.name === 'media')
    assert.equal(media?.kind === 'folder' && media.sharedBytes, 10 * GiB)
  })

  test('change over a period, unknown before the first Scan', async () => {
    const api = createFakeApi(world(), { now })
    const movies = await api.tree('data/media/movies', '30d')
    // b.mkv (6 GiB) appeared on 2026-11-01.
    assert.equal(movies.folder.change, 6 * GiB)
    assert.equal((await api.tree('data/media/movies', '7d')).folder.change, 6 * GiB)
    assert.equal((await api.tree('data/media/movies', '1y')).folder.change, null)
  })

  test('files are listed with their hardlink count', async () => {
    const api = createFakeApi(world(), { now })
    const tree = await api.tree('data/torrents/A', '7d')
    assert.deepEqual(tree.children, [
      {
        kind: 'file',
        name: 'a.mkv',
        path: 'data/torrents/A/a.mkv',
        size: 10 * GiB,
        disk: 'disk1',
        links: 2,
        categoryId: 2,
      },
    ])
  })

  test('an unknown folder is a 404', async () => {
    const api = createFakeApi(world(), { now })
    await assert.rejects(api.tree('nope', '7d'), { status: 404 })
  })
})

describe('history', () => {
  test('one point a day from the first Scan', async () => {
    const api = createFakeApi(world(), { now })
    const { points } = await api.folderHistory('data/media/movies')
    assert.equal(points.length, 37)
    assert.deepEqual(points[0], { day: '2026-10-01', size: 10 * GiB })
    assert.deepEqual(points.at(-1), { day: '2026-11-06', size: 16 * GiB })
    assert.deepEqual(
      points.find((p) => p.day === '2026-11-01'),
      { day: '2026-11-01', size: 16 * GiB },
    )
    assert.deepEqual(
      points.find((p) => p.day === '2026-10-31'),
      { day: '2026-10-31', size: 10 * GiB },
    )
  })
})
