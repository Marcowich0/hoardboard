import { useMemo, useState } from 'react'
import type { UsedSplit } from '../api/index.ts'
import { escapeHtml } from '../lib/html.ts'
import { formatBytes, formatShare } from '../lib/format.ts'
import { useCurrentTheme } from '../theme.ts'
import { Swatch } from '../components/ui.tsx'
import { chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'
import { type Part, useParts } from './parts.ts'

/**
 * Used space as a donut, with its legend: every part's name, size and share.
 * The legend is the table view; hovering either side highlights both.
 */
export default function DonutChart({ split, onSelect }: { split: UsedSplit; onSelect?: (part: Part) => void }) {
  const theme = useCurrentTheme()
  const parts = useParts(split)
  const total = parts.reduce((s, p) => s + p.bytes, 0)
  const [hovered, setHovered] = useState<number | null>(null)
  const shown = parts.filter((p) => p.bytes > 0)

  const option = useMemo((): ChartOption => {
    const c = chrome(theme)
    return {
      tooltip: {
        ...c.tooltip,
        trigger: 'item',
        formatter: (p) => {
          const part = shown[(p as { dataIndex: number }).dataIndex]
          return `<b>${escapeHtml(part.name)}</b><br/>${formatBytes(part.bytes)} · ${formatShare(part.bytes, total)} of Used space`
        },
      },
      series: [
        {
          type: 'pie',
          radius: ['64%', '92%'],
          padAngle: 1.2,
          avoidLabelOverlap: false,
          label: { show: false },
          labelLine: { show: false },
          itemStyle: { borderRadius: 4, borderColor: c.surface, borderWidth: 0 },
          emphasis: { scale: true, scaleSize: 5 },
          data: shown.map((p) => ({
            name: p.name,
            value: p.bytes,
            itemStyle: {
              color: p.colour,
              decal: p.hatch
                ? {
                    symbol: 'rect',
                    dashArrayX: [1, 0],
                    dashArrayY: [2, 4],
                    rotation: -Math.PI / 4,
                    color: 'rgba(0,0,0,0.22)',
                  }
                : undefined,
            },
          })),
        },
      ],
    }
  }, [shown, theme, total])

  const focus = hovered === null ? null : shown[hovered]

  return (
    <div className="grid items-center gap-5 sm:grid-cols-[minmax(0,13.5rem)_1fr]">
      <div className="relative mx-auto aspect-square w-full max-w-60">
        <EChart
          option={option}
          height="100%"
          label="Used space by category"
          onHover={setHovered}
          onClick={(e) => e.dataIndex !== undefined && onSelect?.(shown[e.dataIndex])}
        />
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
          <span className="max-w-[60%] truncate text-[12px] font-medium text-muted">
            {focus ? focus.name : 'Used space'}
          </span>
          <span className="text-xl font-semibold tracking-tight">{formatBytes(focus ? focus.bytes : total)}</span>
          {focus && <span className="text-[12px] text-muted">{formatShare(focus.bytes, total)}</span>}
        </div>
      </div>
      <ul className="min-w-0 space-y-0.5 text-sm">
        {parts.map((p) => {
          const index = shown.indexOf(p)
          const clickable = onSelect && p.key !== 'unaccounted'
          return (
            <li key={String(p.key)}>
              <button
                type="button"
                disabled={!clickable}
                onClick={() => clickable && onSelect(p)}
                onMouseEnter={() => setHovered(index < 0 ? null : index)}
                onMouseLeave={() => setHovered(null)}
                onFocus={() => setHovered(index < 0 ? null : index)}
                onBlur={() => setHovered(null)}
                title={
                  p.key === 'unaccounted'
                    ? 'Filesystem overhead, and anything outside the shares or not readable'
                    : undefined
                }
                className={`grid w-full grid-cols-[auto_1fr_auto_3.2rem] items-center gap-2.5 rounded-lg px-2 py-1.5 text-left transition-colors disabled:cursor-default ${
                  index === hovered && index >= 0 ? 'bg-raised' : 'enabled:hover:bg-raised'
                }`}
              >
                <Swatch colour={p.colour} hatch={p.hatch} />
                <span className={`truncate ${p.key === null || p.key === 'unaccounted' ? 'text-ink-2' : ''}`}>
                  {p.name}
                </span>
                <span className="tabular text-ink-2">{formatBytes(p.bytes)}</span>
                <span className="tabular text-right text-muted">{formatShare(p.bytes, total)}</span>
              </button>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
