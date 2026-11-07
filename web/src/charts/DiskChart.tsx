import { useMemo } from 'react'
import type { Disk } from '../api/index.ts'
import { formatBytes, formatShare } from '../lib/format.ts'
import { escapeHtml } from '../lib/html.ts'
import { useCurrentTheme } from '../theme.ts'
import { chartFont, chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'

/** How full a disk may get before its bar turns to a warning, then to danger. */
export const nearlyFull = 0.9
export const full = 0.95

/**
 * A bar per Disk, used then free, on one scale, so a bigger disk has a longer
 * bar. The used part turns amber, then red, as a disk fills; the share is
 * printed beside every bar, so colour never says it alone.
 */
export default function DiskChart({ disks }: { disks: Disk[] }) {
  const theme = useCurrentTheme()
  const option = useMemo((): ChartOption => {
    const c = chrome(theme)
    const css = getComputedStyle(document.documentElement)
    const meter = css.getPropertyValue('--meter').trim() || c.ink2
    const track = css.getPropertyValue('--track').trim() || c.grid
    const warn = css.getPropertyValue('--warn').trim()
    const bad = css.getPropertyValue('--bad').trim()
    const fill = (d: Disk) => (d.used / d.size >= full ? bad : d.used / d.size >= nearlyFull ? warn : meter)
    const largest = Math.max(...disks.map((d) => d.size))
    return {
      grid: { left: 4, right: 44, top: 0, bottom: 0, containLabel: true },
      tooltip: {
        ...c.tooltip,
        trigger: 'item',
        formatter: (p) => {
          const d = disks[(p as { dataIndex: number }).dataIndex]
          return `<b>${escapeHtml(d.name)}</b> · ${escapeHtml(d.filesystem)}<br/>${formatBytes(d.used)} used · ${formatBytes(d.free)} free<br/><span style="color:${c.muted}">${formatBytes(d.size)} in all</span>`
        },
      },
      xAxis: { type: 'value', max: largest, show: false },
      yAxis: {
        type: 'category',
        inverse: true,
        data: disks.map((d) => d.name),
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: {
          fontFamily: chartFont,
          formatter: (name: string) => {
            const d = disks.find((x) => x.name === name)
            return `{name|${name}}\n{fs|${d ? `${formatBytes(d.size)} · ${d.filesystem}` : ''}}`
          },
          rich: {
            name: { color: c.ink, fontSize: 13, fontWeight: 600, lineHeight: 17, width: 92 },
            fs: { color: c.muted, fontSize: 11, lineHeight: 14, width: 92 },
          },
        },
      },
      series: [
        {
          type: 'bar',
          stack: 'disk',
          barWidth: 14,
          data: disks.map((d) => ({ value: d.used, itemStyle: { color: fill(d), borderRadius: [4, 0, 0, 4] } })),
        },
        {
          type: 'bar',
          stack: 'disk',
          barWidth: 14,
          data: disks.map((d) => ({ value: d.free, itemStyle: { color: track, borderRadius: [0, 4, 4, 0] } })),
          label: {
            show: true,
            position: 'right',
            distance: 8,
            color: c.ink2,
            fontFamily: chartFont,
            fontSize: 12,
            formatter: (p) => formatShare(disks[p.dataIndex].used, disks[p.dataIndex].size),
          },
        },
      ],
    }
  }, [disks, theme])

  return <EChart option={option} height={disks.length * 46} label="Used and free space per disk" />
}
