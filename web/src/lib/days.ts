// Days as the API sends them: "YYYY-MM-DD" strings, one History point each.
// Arithmetic happens in UTC so a day is always 24 hours.

const msPerDay = 86_400_000

/** The day `n` days after `day` (before it when `n` is negative). */
export function addDays(day: string, n: number): string {
  return new Date(Date.parse(`${day}T00:00:00Z`) + n * msPerDay).toISOString().slice(0, 10)
}

/** Whole days from `from` to `to`. */
export function daysBetween(from: string, to: string): number {
  return Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / msPerDay)
}

/** A Date's local calendar day. */
export function dayOf(date: Date): string {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}
