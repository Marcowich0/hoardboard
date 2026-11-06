import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { byteAxis, formatAxisBytes } from './axis.ts'

const GiB = 1024 ** 3
const TiB = 1024 * GiB

describe('byteAxis', () => {
  test('steps in round binary amounts and covers the largest value', () => {
    assert.deepEqual(byteAxis(36.6 * TiB), { max: 40 * TiB, interval: 10 * TiB })
    assert.deepEqual(byteAxis(730 * GiB), { max: 800 * GiB, interval: 200 * GiB })
    assert.deepEqual(byteAxis(9.09 * TiB), { max: 10 * TiB, interval: 2 * TiB })
  })

  test('crosses into the next unit when the step does', () => {
    assert.deepEqual(byteAxis(3.4 * TiB), { max: 4 * TiB, interval: 1 * TiB })
    assert.deepEqual(byteAxis(1900 * GiB), { max: 2 * TiB, interval: 512 * GiB })
  })

  test('has a scale even for nothing', () => {
    assert.deepEqual(byteAxis(0), { max: 1 * GiB, interval: 256 * 1024 ** 2 })
  })
})

describe('formatAxisBytes', () => {
  test('drops trailing zeros so ticks stay short', () => {
    assert.equal(formatAxisBytes(10 * TiB), '10 TiB')
    assert.equal(formatAxisBytes(2.5 * TiB), '2.5 TiB')
    assert.equal(formatAxisBytes(512 * GiB), '512 GiB')
    assert.equal(formatAxisBytes(0), '0')
  })
})
