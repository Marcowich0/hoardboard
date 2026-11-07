import { useMemo } from 'react'
import type { Trend } from '../api/index.ts'
import { useCategories } from '../categories.ts'
import { formatBytes } from '../lib/format.ts'
import { escapeHtml } from '../lib/html.ts'
import { useCurrentTheme } from '../theme.ts'
import { chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'
import { hatchDecal } from './hatch.ts'
import { byteValueAxis, dayLabel, dayPointer, timeAxis } from './time.ts'

/**
 * Used space over time, stacked by category in Rank order (the donut's split
 * per day), Uncategorized and Unaccounted on top: the top edge is Used space.
 */
export default function TrendChart({ trend, from, height = 300 }: { trend: Trend; from: number; height?: number }) {
  const theme = useCurrentTheme()
  const book = useCategories()

  const option = useMemo((): ChartOption => {
    const c = chrome(theme)
    const days = trend.days.slice(from)
    const layers = [
      ...trend.categories.map((s) => {
        const look = book.look(s.categoryId)
        // A group's own layer is its folders without its categories: "Other Media".
        const name = book.membersOf(s.categoryId).length > 0 ? `Other ${look.name}` : look.name
        return { ...look, name, bytes: s.bytes }
      }),
      { ...book.look(null), bytes: trend.uncategorized },
      { ...book.unaccounted, bytes: trend.unaccounted },
    ].map((l) => ({ ...l, bytes: l.bytes.slice(from) }))
    const tops = days.map((_, i) => layers.reduce((s, l) => s + l.bytes[i], 0))
    return {
      grid: { left: 8, right: 12, top: 12, bottom: 4, containLabel: true },
      tooltip: {
        ...c.tooltip,
        trigger: 'axis',
        axisPointer: dayPointer(c),
        order: 'seriesDesc',
        formatter: (raw) => {
          const params = raw as { dataIndex: number }[]
          const i = params[0]?.dataIndex ?? 0
          const rows = layers
            .map((l) => ({ l, v: l.bytes[i] }))
            .filter((r) => r.v > 0)
            .reverse()
            .map(
              ({ l, v }) =>
                `<div style="display:flex;gap:10px;align-items:center;justify-content:space-between"><span style="display:flex;gap:6px;align-items:center"><span style="width:8px;height:8px;border-radius:2px;background:${l.colour}"></span>${escapeHtml(l.name)}</span><span style="font-variant-numeric:tabular-nums">${formatBytes(v)}</span></div>`,
            )
            .join('')
          return `<div style="min-width:180px"><div style="color:${c.muted};margin-bottom:4px">${dayLabel(days[i])}</div>${rows}<div style="display:flex;justify-content:space-between;border-top:1px solid ${c.grid};margin-top:4px;padding-top:4px;font-weight:600"><span>Used space</span><span>${formatBytes(tops[i])}</span></div></div>`
        },
      },
      xAxis: timeAxis(days, c),
      yAxis: byteValueAxis(Math.max(0, ...tops), c),
      series: layers.map((l) => ({
        type: 'line',
        name: l.name,
        stack: 'used',
        data: l.bytes,
        symbol: 'none',
        smooth: false,
        lineStyle: { width: 1, color: c.surface },
        areaStyle: {
          color: l.colour,
          opacity: 0.92,
          decal: l.hatch ? hatchDecal : undefined,
        },
        emphasis: { disabled: true },
      })),
    }
  }, [book, from, theme, trend])

  return <EChart option={option} height={height} label="Used space by category over time" />
}
