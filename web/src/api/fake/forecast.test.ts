import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { forecast } from './forecast.ts'

const GiB = 1024 ** 3
const TiB = 1024 * GiB

// Daily points of total Used space, oldest first, ending today.
function days(count: number, start: number, perDay: number): number[] {
  return Array.from({ length: count }, (_, i) => start + i * perDay)
}

describe('forecast', () => {
  test('needs at least 7 daily points', () => {
    assert.equal(forecast(days(6, 10 * TiB, 10 * GiB), 20 * TiB, '2026-11-06'), null)
    assert.notEqual(forecast(days(7, 10 * TiB, 10 * GiB), 20 * TiB, '2026-11-06'), null)
  })

  test('gives the growth per month and the day the array is full', () => {
    // 10 GiB a day (304.4 GiB in an average month of 30.44 days); 1 TiB
    // (1024 GiB) left, so the line reaches capacity 102.4 days from today.
    const result = forecast(days(30, 19 * TiB, 10 * GiB), 20 * TiB + 290 * GiB, '2026-11-06')
    assert.deepEqual(result, { bytesPerMonth: 326_847_011_226, fullOn: '2027-02-17' })
  })

  test('fits only the last 90 days', () => {
    // Shrinking for 100 days, then growing at 1 GiB a day for 90.
    const points = [...days(100, 30 * TiB, -GiB), ...days(90, 10 * TiB, GiB)]
    const result = forecast(points, 20 * TiB, '2026-11-06')
    assert.equal(result?.bytesPerMonth, 32_684_701_123)
  })

  test('is omitted when Used space is flat or shrinking', () => {
    assert.equal(forecast(days(30, 10 * TiB, 0), 20 * TiB, '2026-11-06'), null)
    assert.equal(forecast(days(30, 10 * TiB, -GiB), 20 * TiB, '2026-11-06'), null)
  })
})
