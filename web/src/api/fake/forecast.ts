import { addDays } from '../../lib/days.ts'
import type { Forecast } from '../types.ts'

/** The days the fit looks back over, and the fewest it needs. */
const window = 90
const minPoints = 7
const daysPerMonth = 30.44

/**
 * The spec's forecast: a least-squares line through the last 90 daily points
 * of total Used space (oldest first, the last one is `today`). Null with
 * fewer than 7 points, or when the line is flat or falling.
 */
export function forecast(usedPerDay: number[], capacity: number, today: string): Forecast | null {
  const recent = usedPerDay.slice(-window)
  const n = recent.length
  if (n < minPoints) return null

  // Fit relative to the first point: sizes are around 2^44, and squaring
  // their deviations from the mean in floats would lose whole bytes per day.
  const base = recent[0]
  const points = recent.map((y) => y - base)

  const meanX = (n - 1) / 2
  const meanY = points.reduce((sum, y) => sum + y, 0) / n
  let sxy = 0
  let sxx = 0
  points.forEach((y, x) => {
    sxy += (x - meanX) * (y - meanY)
    sxx += (x - meanX) ** 2
  })
  const perDay = sxy / sxx
  if (!(perDay > 0)) return null

  const usedToday = base + meanY + perDay * (n - 1 - meanX)
  const daysLeft = Math.max(0, Math.ceil((capacity - usedToday) / perDay - 1e-9))
  return { bytesPerMonth: Math.round(perDay * daysPerMonth), fullOn: addDays(today, daysLeft) }
}
