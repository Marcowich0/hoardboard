import type { Chrome } from './chrome.ts'
import { chartFont } from './chrome.ts'

const long = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' })
const short = new Intl.DateTimeFormat('en-GB', { month: 'short', timeZone: 'UTC' })
const shortYear = new Intl.DateTimeFormat('en-GB', { month: 'short', year: '2-digit', timeZone: 'UTC' })

/** "6 Nov 2026". */
export function dayLabel(day: string): string {
  return long.format(new Date(`${day}T00:00:00Z`))
}

/** A category axis of days, labelled by month at month starts. */
export function timeAxis(days: string[], c: Chrome) {
  const span = days.length
  // Fewer labels on long ranges so they never collide.
  const every = span > 400 ? 3 : span > 200 ? 2 : 1
  let count = 0
  const labelled = new Set(
    days.filter((d) => {
      if (!d.endsWith('-01')) return false
      return count++ % every === 0
    }),
  )
  return {
    type: 'category' as const,
    data: days,
    boundaryGap: false,
    axisLine: { lineStyle: { color: c.axis } },
    axisTick: { show: false },
    axisLabel: {
      color: c.muted,
      fontFamily: chartFont,
      fontSize: 11,
      interval: (_: number, value: string) => labelled.has(value),
      formatter: (value: string) => {
        const date = new Date(`${value}T00:00:00Z`)
        return span > 120 || value.slice(5, 7) === '01' ? shortYear.format(date) : short.format(date)
      },
      hideOverlap: true,
    },
    splitLine: { show: false },
  }
}
