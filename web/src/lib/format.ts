// Every number the app shows goes through here, so units and wording stay the
// same on every page. Sizes are always binary (KiB, MiB, GiB, TiB, PiB).

import { daysBetween } from './days.ts'

const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB']

/**
 * A size in binary units with three significant digits: "512 B", "1.50 KiB",
 * "123 GiB", "12.3 TiB". A value moves up a unit before it needs four digits,
 * so 1000 MiB reads "0.98 GiB".
 */
export function formatBytes(bytes: number): string {
  let value = Math.abs(bytes)
  let unit = 0
  while (value >= 1000 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  const sign = bytes < 0 ? '−' : ''
  if (unit === 0) return `${sign}${Math.round(value)} B`
  const digits = value >= 100 ? 0 : value >= 10 ? 1 : 2
  return `${sign}${value.toFixed(digits)} ${units[unit]}`
}

/** A change in size, signed: "+1.50 GiB", "−2.00 TiB", or "0 B". */
export function formatChange(bytes: number): string {
  return bytes > 0 ? `+${formatBytes(bytes)}` : formatBytes(bytes)
}

/** How long ago something happened: "just now", "14 min ago", "3 h ago", "5 days ago". */
export function formatAgo(then: Date, now: Date): string {
  const seconds = Math.max(0, (now.getTime() - then.getTime()) / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} min ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} h ago`
  const days = Math.floor(hours / 24)
  return days === 1 ? '1 day ago' : `${days} days ago`
}

/** How long something took: "3 s", "1 min 12 s", "2 h 5 min". */
export function formatDuration(ms: number): string {
  const seconds = Math.max(1, Math.round(ms / 1000))
  if (seconds < 60) return `${seconds} s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} min ${seconds % 60} s`
  return `${Math.floor(minutes / 60)} h ${minutes % 60} min`
}

/** How far off a day is, roughly: "today", "~20 days", "~13 months", "~2.5 years". */
export function formatTimeLeft(day: string, today: string): string {
  const days = daysBetween(today, day)
  if (days <= 0) return 'today'
  if (days < 60) return days === 1 ? '~1 day' : `~${days} days`
  const months = Math.round(days / 30.44)
  if (months < 24) return `~${months} months`
  const years = days / 365.25
  return `~${years < 10 ? Math.round(years * 10) / 10 : Math.round(years)} years`
}

const counts = new Intl.NumberFormat('en-US')

/** A count of items: "7", "203,481". */
export function formatCount(n: number): string {
  return counts.format(n)
}

/** `part` as a share of `whole`: "73%", or "<1%" for a sliver. */
export function formatShare(part: number, whole: number): string {
  if (whole <= 0 || part <= 0) return '0%'
  const percent = (part / whole) * 100
  return percent < 1 ? '<1%' : `${Math.round(percent)}%`
}
