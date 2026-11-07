import { useMemo } from 'react'
import type { Disk } from '../api/index.ts'
import { useCategories } from '../categories.ts'
import { formatBytes, formatShare } from '../lib/format.ts'
import { escapeHtml } from '../lib/html.ts'
import { useCurrentTheme } from '../theme.ts'
import { chartFont, chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'
import { hatchDecal } from './hatch.ts'
import { type Part, splitParts } from './parts.ts'

/** How full a disk may get before its share turns to a warning, then to danger. */
export const nearlyFull = 0.9
export const full = 0.95

/**
 * A bar per Disk on one scale, so a bigger disk has a longer bar: its Used
 * space split by category (the donut's colours, from what the latest Scan
 * found on that Disk), then its Free space. The share used is printed
 * beside every bar, with a warning mark as a disk fills, so neither colour
 * nor length says it alone.
 */
export default function DiskChart({ disks, onSelect }: { disks: Disk[]; onSelect?: (part: Part) => void }) {
  const theme = useCurrentTheme()
  const book = useCategories()
  const option = useMemo((): ChartOption => {
    const c = chrome(theme)
    const css = getComputedStyle(document.documentElement)
    const track = css.getPropertyValue('--track').trim() || c.grid
    const warn = css.getPropertyValue('--warn').trim()
    const bad = css.getPropertyValue('--bad').trim()
    const largest = Math.max(...disks.map((d) => d.size))
    // Each Disk's parts, in the same order on every Disk: a series per part.
    const parts = disks.map((d) => splitParts(d.split, book))
    const keys = parts[0]?.map((p) => p.key) ?? []
    // Round the outer ends only: the first part with bytes, and Free space.
    const first = parts.map((ps) => ps.findIndex((p) => p.bytes > 0))
    const level = (d: Disk) => (d.used / d.size >= full ? 'bad' : d.used / d.size >= nearlyFull ? 'warn' : 'ok')

    return {
      grid: { left: 4, right: 56, top: 0, bottom: 0, containLabel: true },
      tooltip: {
        ...c.tooltip,
        trigger: 'axis',
        axisPointer: { type: 'shadow', shadowStyle: { color: theme === 'dark' ? 'rgba(255,255,255,0.04)' : 'rgba(0,0,0,0.04)' } },
        formatter: (raw) => {
          const i = (raw as { dataIndex: number }[])[0]?.dataIndex ?? 0
          const d = disks[i]
          const rows = parts[i]
            .filter((p) => p.bytes > 0)
            .map(
              (p) =>
                `<div style="display:flex;gap:12px;justify-content:space-between"><span style="display:flex;gap:6px;align-items:center"><span style="width:8px;height:8px;border-radius:2px;background:${p.colour}"></span>${escapeHtml(p.name)}</span><span style="font-variant-numeric:tabular-nums">${formatBytes(p.bytes)}</span></div>`,
            )
            .join('')
          const state = level(d) === 'bad' ? ' · almost full' : level(d) === 'warn' ? ' · getting full' : ''
          return `<div style="min-width:200px"><div style="margin-bottom:4px"><b>${escapeHtml(d.name)}</b> <span style="color:${c.muted}">${escapeHtml(d.filesystem)} · ${formatBytes(d.size)}${state}</span></div>${rows}<div style="display:flex;justify-content:space-between;border-top:1px solid ${c.grid};margin-top:4px;padding-top:4px"><span>Free space</span><span style="font-variant-numeric:tabular-nums">${formatBytes(d.free)}</span></div></div>`
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
        ...keys.map((key, k) => ({
          type: 'bar' as const,
          id: String(key),
          name: parts[0][k].name,
          stack: 'disk',
          barWidth: 16,
          emphasis: { disabled: true },
          data: disks.map((_, i) => {
            const p = parts[i][k]
            return {
              value: p.bytes,
              itemStyle: {
                color: p.colour,
                decal: p.hatch ? hatchDecal : undefined,
                // A 2px surface gap between neighbouring parts.
                borderColor: c.surface,
                borderWidth: p.bytes > 0 ? 1 : 0,
                borderRadius: k === first[i] ? [4, 0, 0, 4] : 0,
              },
            }
          }),
        })),
        {
          type: 'bar',
          id: 'free',
          name: 'Free space',
          stack: 'disk',
          barWidth: 16,
          emphasis: { disabled: true },
          data: disks.map((d) => ({
            value: d.free,
            itemStyle: { color: track, borderColor: c.surface, borderWidth: 1, borderRadius: [0, 4, 4, 0] },
          })),
          label: {
            show: true,
            position: 'right',
            distance: 8,
            fontFamily: chartFont,
            fontSize: 12,
            formatter: (p) => {
              const d = disks[p.dataIndex]
              const share = formatShare(d.used, d.size)
              const l = level(d)
              return l === 'ok' ? `{ok|${share}}` : `{${l}|▲ ${share}}`
            },
            rich: {
              ok: { color: c.ink2, fontSize: 12 },
              warn: { color: warn || c.ink, fontSize: 12, fontWeight: 600 },
              bad: { color: bad || c.ink, fontSize: 12, fontWeight: 600 },
            },
          },
        },
      ],
    }
  }, [book, disks, theme])

  return (
    <EChart
      option={option}
      height={disks.length * 46}
      label="Used space per disk by category, and free space"
      onClick={(e) => {
        const parts = splitParts(disks[e.dataIndex]?.split ?? { categories: [], uncategorized: 0, unaccounted: 0 }, book)
        const part = parts.find((p) => String(p.key) === e.seriesId)
        if (part) onSelect?.(part)
      }}
    />
  )
}
