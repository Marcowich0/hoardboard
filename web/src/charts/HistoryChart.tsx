import { useMemo } from 'react'
import type { History } from '../api/index.ts'
import { byteAxis, formatAxisBytes } from '../lib/axis.ts'
import { formatBytes } from '../lib/format.ts'
import { escapeHtml } from '../lib/html.ts'
import { useCurrentTheme } from '../theme.ts'
import { chartFont, chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'
import { dayLabel, timeAxis } from './time.ts'

/** One Folder's or category's Size per day: a 2px line over a faint wash, from zero. */
export default function HistoryChart({
  history,
  colour,
  name,
  height = 220,
}: {
  history: History
  colour: string
  name: string
  height?: number
}) {
  const theme = useCurrentTheme()
  const option = useMemo((): ChartOption => {
    const c = chrome(theme)
    const days = history.points.map((p) => p.day)
    const sizes = history.points.map((p) => p.size)
    const axis = byteAxis(Math.max(0, ...sizes))
    return {
      grid: { left: 8, right: 12, top: 10, bottom: 4, containLabel: true },
      tooltip: {
        ...c.tooltip,
        trigger: 'axis',
        axisPointer: { type: 'line', lineStyle: { color: c.axis, width: 1 } },
        formatter: (raw) => {
          const i = (raw as { dataIndex: number }[])[0]?.dataIndex ?? 0
          return `<div style="color:${c.muted}">${dayLabel(days[i])}</div><b>${formatBytes(sizes[i])}</b> <span style="color:${c.muted}">${escapeHtml(name)}</span>`
        },
      },
      xAxis: timeAxis(days, c),
      yAxis: {
        type: 'value',
        min: 0,
        max: axis.max,
        interval: axis.interval,
        axisLabel: {
          color: c.muted,
          fontFamily: chartFont,
          fontSize: 11,
          formatter: (v: number) => formatAxisBytes(v),
        },
        splitLine: { lineStyle: { color: c.grid } },
      },
      series: [
        {
          type: 'line',
          data: sizes,
          symbol: 'circle',
          symbolSize: 8,
          showSymbol: false,
          lineStyle: { width: 2, color: colour, cap: 'round', join: 'round' },
          itemStyle: { color: colour, borderColor: c.surface, borderWidth: 2 },
          areaStyle: { color: colour, opacity: 0.1 },
        },
      ],
    }
  }, [colour, history, name, theme])

  return <EChart option={option} height={height} label={`${name}: Size over time`} />
}
