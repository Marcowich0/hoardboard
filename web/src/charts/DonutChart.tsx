import { useMemo, useState } from 'react'
import type { UsedSplit } from '../api/index.ts'
import { escapeHtml } from '../lib/html.ts'
import { formatBytes, formatShare } from '../lib/format.ts'
import { useCurrentTheme } from '../theme.ts'
import { Swatch } from '../components/ui.tsx'
import { chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'
import { hatchDecal } from './hatch.ts'
import { type Part, type PartGroup, groupParts, useParts } from './parts.ts'

/** What the centre shows: a part, or a whole group; id says which legend row is lit. */
interface Focus {
  id: string
  name: string
  bytes: number
}

/**
 * Used space as a donut, with its legend: every part's name, size and share.
 * A thin outer ring spans each group (Media over Movies and TV), and the
 * legend nests a group's categories under it. The legend is the table view;
 * hovering either side shows the part (or group) in the centre.
 */
export default function DonutChart({ split, onSelect }: { split: UsedSplit; onSelect?: (part: Part) => void }) {
  const theme = useCurrentTheme()
  const parts = useParts(split)
  const groups = useMemo(() => groupParts(parts), [parts])
  const total = parts.reduce((s, p) => s + p.bytes, 0)
  const [focus, setFocus] = useState<Focus | null>(null)
  const shown = parts.filter((p) => p.bytes > 0)
  const shownGroups = groups.filter((g) => g.bytes > 0)
  const hasGroups = groups.some((g) => g.members)

  const option = useMemo((): ChartOption => {
    const c = chrome(theme)
    const tip = (name: string, bytes: number) =>
      `<b>${escapeHtml(name)}</b><br/>${formatBytes(bytes)} · ${formatShare(bytes, total)} of Used space`
    return {
      tooltip: { ...c.tooltip, trigger: 'item' },
      series: [
        {
          type: 'pie',
          radius: hasGroups ? ['60%', '87%'] : ['64%', '92%'],
          avoidLabelOverlap: false,
          label: { show: false },
          labelLine: { show: false },
          // A 2px surface gap between parts; no padAngle, so the group ring lines up.
          itemStyle: { borderRadius: 4, borderColor: c.surface, borderWidth: 2 },
          emphasis: { scale: true, scaleSize: 4 },
          tooltip: { formatter: (p) => tip(shown[(p as { dataIndex: number }).dataIndex].name, (p as { value: number }).value) },
          data: shown.map((p) => ({
            name: p.name,
            value: p.bytes,
            itemStyle: { color: p.colour, decal: p.hatch ? hatchDecal : undefined },
          })),
        },
        ...(hasGroups
          ? [
              {
                type: 'pie' as const,
                radius: ['91%', '97%'],
                avoidLabelOverlap: false,
                label: { show: false },
                labelLine: { show: false },
                itemStyle: { borderRadius: 3, borderColor: c.surface, borderWidth: 2 },
                emphasis: { scale: false },
                tooltip: {
                  formatter: (p: unknown) => {
                    const g = shownGroups[(p as { dataIndex: number }).dataIndex]
                    return g.members ? tip(g.name, g.bytes) : ''
                  },
                },
                data: shownGroups.map((g) => ({
                  name: g.name,
                  value: g.bytes,
                  // Only a group has an arc; the rest of the ring is empty.
                  itemStyle: { color: g.members ? g.colour : 'transparent' },
                  silent: !g.members,
                })),
              },
            ]
          : []),
      ],
    }
  }, [hasGroups, shown, shownGroups, theme, total])

  const focusPart = (p: Part): Focus => ({ id: `part:${p.key}`, name: p.name, bytes: p.bytes })
  const focusGroup = (g: PartGroup): Focus => ({
    id: g.members ? `group:${g.key}` : `part:${g.key}`,
    name: g.name,
    bytes: g.bytes,
  })

  return (
    <div className="grid items-center gap-5 sm:grid-cols-[minmax(0,13.5rem)_1fr]">
      <div className="relative mx-auto aspect-square w-full max-w-60">
        <EChart
          option={option}
          height="100%"
          label="Used space by category"
          onHover={(index, series) => {
            if (index === null) return setFocus(null)
            if (series === 1) {
              const g = shownGroups[index]
              return setFocus(g?.members ? focusGroup(g) : null)
            }
            setFocus(shown[index] ? focusPart(shown[index]) : null)
          }}
          onClick={(e) => {
            if (e.dataIndex === undefined) return
            if (e.seriesIndex === 1) {
              const g = shownGroups[e.dataIndex]
              if (g?.members) onSelect?.(g.members[0])
              return
            }
            onSelect?.(shown[e.dataIndex])
          }}
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
        {groups.map((g) => (
          <li key={String(g.key)}>
            <LegendRow
              part={g}
              bytes={g.bytes}
              total={total}
              active={focus?.id === focusGroup(g).id}
              onSelect={onSelect && g.key !== 'unaccounted' ? () => onSelect({ ...g, group: null }) : undefined}
              onFocus={(on) => setFocus(on ? focusGroup(g) : null)}
            />
            {g.members && (
              <ul className="mb-1 ml-[1.1rem] border-l border-line pl-1.5">
                {g.members.map((m) => (
                  <li key={String(m.key)}>
                    <LegendRow
                      part={m}
                      bytes={m.bytes}
                      total={total}
                      small
                      active={focus?.id === focusPart(m).id}
                      onSelect={onSelect ? () => onSelect(m) : undefined}
                      onFocus={(on) => setFocus(on ? focusPart(m) : null)}
                    />
                  </li>
                ))}
              </ul>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

function LegendRow({
  part,
  bytes,
  total,
  small = false,
  active,
  onSelect,
  onFocus,
}: {
  part: { key: Part['key']; name: string; colour: string; hatch: boolean }
  bytes: number
  total: number
  small?: boolean
  active: boolean
  onSelect?: () => void
  onFocus: (on: boolean) => void
}) {
  const quiet = part.key === null || part.key === 'unaccounted'
  return (
    <button
      type="button"
      disabled={!onSelect}
      onClick={onSelect}
      onMouseEnter={() => onFocus(true)}
      onMouseLeave={() => onFocus(false)}
      onFocus={() => onFocus(true)}
      onBlur={() => onFocus(false)}
      title={
        part.key === 'unaccounted' ? 'Filesystem overhead, and anything outside the shares or not readable' : undefined
      }
      className={`grid w-full grid-cols-[auto_1fr_auto_3.2rem] items-center gap-2.5 rounded-lg px-2 text-left transition-colors disabled:cursor-default ${
        small ? 'py-1 text-[13px]' : 'py-1.5'
      } ${active ? 'bg-raised' : 'enabled:hover:bg-raised'}`}
    >
      <Swatch colour={part.colour} hatch={part.hatch} size={small ? 8 : undefined} />
      <span className={`truncate ${quiet || small ? 'text-ink-2' : ''}`}>{part.name}</span>
      <span className="tabular text-ink-2">{formatBytes(bytes)}</span>
      <span className="tabular text-right text-muted">{formatShare(bytes, total)}</span>
    </button>
  )
}
