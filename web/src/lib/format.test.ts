import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  formatAgo,
  formatBytes,
  formatChange,
  formatCount,
  formatDuration,
  formatShare,
  formatTimeLeft,
} from './format.ts'

const KiB = 1024
const MiB = 1024 * KiB
const GiB = 1024 * MiB
const TiB = 1024 * GiB

describe('formatBytes', () => {
  test('shows small amounts in whole bytes', () => {
    assert.equal(formatBytes(0), '0 B')
    assert.equal(formatBytes(512), '512 B')
  })

  test('uses binary units with three significant digits', () => {
    assert.equal(formatBytes(1536), '1.50 KiB')
    assert.equal(formatBytes(123.4 * GiB), '123 GiB')
    assert.equal(formatBytes(12.34 * TiB), '12.3 TiB')
    assert.equal(formatBytes(1.5 * 1024 * TiB), '1.50 PiB')
  })

  test('moves to the next unit before a value needs four digits', () => {
    assert.equal(formatBytes(1000 * MiB), '0.98 GiB')
    assert.equal(formatBytes(1000), '0.98 KiB')
  })
})

describe('formatChange', () => {
  test('signs growth and shrinkage, with a true minus sign', () => {
    assert.equal(formatChange(1.5 * GiB), '+1.50 GiB')
    assert.equal(formatChange(-2 * TiB), '\u22122.00 TiB')
  })

  test('shows no change without a sign', () => {
    assert.equal(formatChange(0), '0 B')
  })
})

describe('formatAgo', () => {
  const now = new Date('2026-11-06T12:00:00Z')
  const before = (seconds: number) => new Date(now.getTime() - seconds * 1000)

  test('says "just now" for the last minute', () => {
    assert.equal(formatAgo(before(20), now), 'just now')
  })

  test('counts minutes, hours, then days', () => {
    assert.equal(formatAgo(before(14 * 60), now), '14 min ago')
    assert.equal(formatAgo(before(3 * 3600 + 600), now), '3 h ago')
    assert.equal(formatAgo(before(26 * 3600), now), '1 day ago')
    assert.equal(formatAgo(before(5 * 86400), now), '5 days ago')
  })

  test('treats a time slightly in the future as now', () => {
    assert.equal(formatAgo(before(-5), now), 'just now')
  })
})

describe('formatDuration', () => {
  test('rounds short runs to whole seconds, never below 1 s', () => {
    assert.equal(formatDuration(3200), '3 s')
    assert.equal(formatDuration(120), '1 s')
  })

  test('adds minutes and hours for long runs', () => {
    assert.equal(formatDuration(72_000), '1 min 12 s')
    assert.equal(formatDuration(2 * 3600_000 + 5 * 60_000), '2 h 5 min')
  })
})

describe('formatTimeLeft', () => {
  test('counts days for the next two months', () => {
    assert.equal(formatTimeLeft('2026-11-06', '2026-11-06'), 'today')
    assert.equal(formatTimeLeft('2026-11-26', '2026-11-06'), '~20 days')
  })

  test('counts months for the next two years, then years', () => {
    assert.equal(formatTimeLeft('2027-02-17', '2026-11-06'), '~3 months')
    assert.equal(formatTimeLeft('2027-12-01', '2026-11-06'), '~13 months')
    assert.equal(formatTimeLeft('2029-05-06', '2026-11-06'), '~2.5 years')
    assert.equal(formatTimeLeft('2040-01-01', '2026-11-06'), '~13 years')
  })
})

describe('formatCount', () => {
  test('groups thousands', () => {
    assert.equal(formatCount(7), '7')
    assert.equal(formatCount(203_481), '203,481')
  })
})

describe('formatShare', () => {
  test('shows a part of a whole in whole percent, tiny parts as "<1%"', () => {
    assert.equal(formatShare(734, 1000), '73%')
    assert.equal(formatShare(3, 1000), '<1%')
    assert.equal(formatShare(0, 1000), '0%')
    assert.equal(formatShare(5, 0), '0%')
  })
})
