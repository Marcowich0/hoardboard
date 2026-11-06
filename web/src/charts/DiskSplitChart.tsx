import { useMemo } from 'react'
import { formatBytes, formatShare } from '../lib/format.ts'
import { useCurrentTheme } from '../theme.ts'
import { chartFont, chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'

/** Which Disks hold a folder's bytes: a bar each, largest first, in the folder's colour. */
export default function DiskSplitChart({
  disks,
  colour,
}: {
  disks: { disk: string; bytes: number }[]
  colour: string
}) {
  const theme = useCurrentTheme()
  const total = disks.reduce((s, d) => s + d.bytes, 0)
  const option = useMemo((): ChartOption => {
    const c = chrome(theme)
    return {
      grid: { left: 4, right: 72, top: 0, bottom: 0, containLabel: true },
      tooltip: {
        ...c.tooltip,
        trigger: 'item',
        formatter: (p) => {
          const d = disks[(p as { dataIndex: number }).dataIndex]
          return `<b>${d.disk}</b><br/>${formatBytes(d.bytes)} · ${formatShare(d.bytes, total)} of this folder`
        },
      },
      xAxis: { type: 'value', max: Math.max(...disks.map((d) => d.bytes)), show: false },
      yAxis: {
        type: 'category',
        inverse: true,
        data: disks.map((d) => d.disk),
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { color: c.ink2, fontFamily: chartFont, fontSize: 12 },
      },
      series: [
        {
          type: 'bar',
          barWidth: 12,
          data: disks.map((d) => d.bytes),
          itemStyle: { color: colour, borderRadius: [0, 4, 4, 0] },
          label: {
            show: true,
            position: 'right',
            color: c.ink2,
            fontFamily: chartFont,
            fontSize: 12,
            formatter: (p) => formatBytes(disks[p.dataIndex].bytes),
          },
        },
      ],
      animationDuration: 300,
    }
  }, [colour, disks, theme, total])

  return <EChart option={option} height={Math.max(1, disks.length) * 30} label="Bytes per disk" />
}
