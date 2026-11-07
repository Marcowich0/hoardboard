import { formatBytes } from './format.ts'

const multiples = [1, 2, 5, 10, 20, 50, 100, 200, 256, 512]
const maxIntervals = 5

/**
 * A byte axis with round ticks: the smallest step (1, 2, 5… or 256, 512 of a
 * binary unit) that covers `largest` in at most five intervals. A tick at or
 * above 1000 of a unit is shown in the next unit, so there only steps that
 * divide 1024 are allowed: ticks read "1 TiB", never "0.98 TiB".
 */
export function byteAxis(largest: number): { max: number; interval: number } {
  const value = largest > 0 ? largest : 1024 ** 3
  for (let unit = 1; ; unit *= 1024) {
    for (const m of multiples) {
      const step = m * unit
      const intervals = Math.ceil(value / step - 1e-9)
      if (intervals > maxIntervals) continue
      const max = intervals * step
      if (max >= 1000 * unit && (1024 * unit) % step !== 0) continue
      return { max, interval: step }
    }
  }
}

/** A tick label: like formatBytes, without trailing zeros ("10 TiB", "2.5 TiB", "0"). */
export function formatAxisBytes(bytes: number): string {
  if (bytes === 0) return '0'
  return formatBytes(bytes)
    .replace(/\.0+ /, ' ')
    .replace(/(\.\d*?)0+ /, '$1 ')
}
