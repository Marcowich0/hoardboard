import { useMemo } from 'react'
import type { Entry } from '../api/index.ts'
import { useCategories } from '../categories.ts'
import { labelInk } from '../lib/colours.ts'
import { formatBytes } from '../lib/format.ts'
import { escapeHtml } from '../lib/html.ts'
import { useCurrentTheme } from '../theme.ts'
import { chartFont, chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'

/** Past this many children, the smallest fold into one "more" tile. */
const maxTiles = 1500

/**
 * The current folder's children as tiles sized by Size and coloured by their
 * category. Files are drawn lighter than folders. Clicking a folder opens it.
 */
export default function TreemapChart({
  entries,
  total,
  onOpen,
  height = '100%',
}: {
  entries: Entry[]
  total: number
  onOpen: (entry: Entry) => void
  height?: number | string
}) {
  const theme = useCurrentTheme()
  const book = useCategories()
  const shown = useMemo(
    () =>
      entries
        .filter((e) => e.size > 0)
        .sort((a, b) => b.size - a.size)
        .slice(0, maxTiles),
    [entries],
  )
  const rest = useMemo(() => {
    const others = entries.filter((e) => e.size > 0).length - shown.length
    const bytes = entries.reduce((s, e) => s + e.size, 0) - shown.reduce((s, e) => s + e.size, 0)
    return { count: others, bytes }
  }, [entries, shown])

  const option = useMemo((): ChartOption => {
    const c = chrome(theme)
    return {
      tooltip: {
        ...c.tooltip,
        formatter: (p) => {
          const e = shown[tileIndex((p as unknown as { data: { id: string } }).data)]
          if (!e) return `<b>${rest.count} smaller items</b><br/>${formatBytes(rest.bytes)} together`
          const look = book.look(e.categoryId)
          return `<b>${escapeHtml(e.name)}</b><br/>${formatBytes(e.size)} · ${total > 0 ? Math.max(1, Math.round((e.size / total) * 100)) : 0}%<br/><span style="color:${c.muted}">${e.kind === 'file' ? 'File' : 'Folder'} · ${escapeHtml(look.name)}</span>`
        },
      },
      series: [
        {
          type: 'treemap',
          left: 0,
          right: 0,
          top: 0,
          bottom: 0,
          roam: false,
          nodeClick: false,
          breadcrumb: { show: false },
          squareRatio: 0.75,
          visibleMin: 64,
          itemStyle: { borderColor: c.surface, borderWidth: 0, gapWidth: 2, borderRadius: 5 },
          emphasis: { itemStyle: { borderColor: c.ink, borderWidth: 0 } },
          label: {
            show: true,
            position: 'insideTopLeft',
            padding: [6, 8],
            fontFamily: chartFont,
            overflow: 'truncate',
            ellipsis: '…',
            formatter: (p) => {
              const e = shown[tileIndex(p.data as { id: string })]
              return e
                ? `{name|${e.name.replace(/[{}|]/g, ' ')}}\n{size|${formatBytes(e.size)}}`
                : `{name|${rest.count} more}\n{size|${formatBytes(rest.bytes)}}`
            },
          },
          data: [
            ...shown.map((e, index) => {
              const colour = book.look(e.categoryId).colour
              return {
                id: String(index),
                name: e.name,
                value: e.size,
                itemStyle: { color: colour, opacity: e.kind === 'file' ? 0.72 : 1 },
                label: {
                  rich: {
                    name: { color: labelInk(colour), fontSize: 13, fontWeight: 600, lineHeight: 18 },
                    size: { color: labelInk(colour), fontSize: 12, opacity: 0.85, lineHeight: 16 },
                  },
                },
              }
            }),
            ...(rest.count > 0
              ? [
                  {
                    id: 'more',
                    name: 'more',
                    value: rest.bytes,
                    itemStyle: { color: c.grid, opacity: 1 },
                    label: {
                      rich: {
                        name: { color: c.ink2, fontSize: 13, fontWeight: 600, lineHeight: 18 },
                        size: { color: c.muted, fontSize: 12, lineHeight: 16 },
                      },
                    },
                  },
                ]
              : []),
          ],
        },
      ],
    }
  }, [book, rest, shown, theme, total])

  return (
    <EChart
      option={option}
      height={height}
      label="What this folder holds, sized by Size"
      onClick={(e) => {
        const entry = e.data ? shown[tileIndex(e.data as { id: string })] : undefined
        if (entry) onOpen(entry)
      }}
    />
  )
}

/** A tile's position in `shown`, from the id it carries ("more" is none). */
function tileIndex(data: { id: string }): number {
  return data.id === 'more' ? -1 : Number(data.id)
}
