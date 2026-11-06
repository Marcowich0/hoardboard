import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { breadcrumbs, explorerHref, parentPath, periodFrom } from './paths.ts'

describe('explorerHref', () => {
  test('puts the path in the URL, each name escaped', () => {
    assert.equal(explorerHref('Photos/2024/Trip #2 50%', '7d'), '/explorer/Photos/2024/Trip%20%232%2050%25?period=7d')
  })

  test('leaves out the default period and the root', () => {
    assert.equal(explorerHref('data/media', '30d'), '/explorer/data/media')
    assert.equal(explorerHref('', '30d'), '/explorer')
    assert.equal(explorerHref('', '1y'), '/explorer?period=1y')
  })
})

describe('periodFrom', () => {
  test('reads the period from the query, 30 days when absent or unknown', () => {
    assert.equal(periodFrom(new URLSearchParams('period=1y')), '1y')
    assert.equal(periodFrom(new URLSearchParams('')), '30d')
    assert.equal(periodFrom(new URLSearchParams('period=2w')), '30d')
  })
})

describe('breadcrumbs', () => {
  test('starts at the root and ends at the folder', () => {
    assert.deepEqual(breadcrumbs('data/media/movies'), [
      { name: 'All shares', path: '' },
      { name: 'data', path: 'data' },
      { name: 'media', path: 'data/media' },
      { name: 'movies', path: 'data/media/movies' },
    ])
    assert.deepEqual(breadcrumbs(''), [{ name: 'All shares', path: '' }])
  })
})

describe('parentPath', () => {
  test('goes one folder up, to the root at most', () => {
    assert.equal(parentPath('data/media'), 'data')
    assert.equal(parentPath('data'), '')
    assert.equal(parentPath(''), '')
  })
})
