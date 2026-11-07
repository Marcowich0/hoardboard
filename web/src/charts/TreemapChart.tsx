import { useEffect, useMemo, useRef, useState } from 'react'
import type { TreemapNode } from '../api/index.ts'
import { type CategoryBook, useCategories } from '../categories.ts'
import { labelInk, mix, shade, type Theme, uncategorizedTints } from '../lib/colours.ts'
import { formatBytes, formatCount, formatShare } from '../lib/format.ts'
import { escapeHtml } from '../lib/html.ts'
import { useCurrentTheme } from '../theme.ts'
import { type Chrome, chartFont, chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'

/** A treemap tile: the node it draws, and the colour its label sits on. */
interface Tile {
  id: string
  name: string
  value: number
  itemStyle: { color: string; borderColor: string }
  label: object
  upperLabel: object
  children?: Tile[]
}

/**
 * What a Folder holds, nested: each Folder's tile holds its own subfolders and
 * files, a few levels deep, so the big things show without opening anything
 * (the way WinDirStat draws a disk). Tiles are sized by Size; a Folder's tile
 * has a header with its name. Colour is the category; what no category
 * claims is drawn in muted tints, one per top-level branch, so neighbouring
 * folders still differ. Files are lighter than folders. Clicking a folder
 * opens it; clicking a file opens its folder.
 */
export default function TreemapChart({
  nodes,
  total,
  onOpen,
  height = '100%',
}: {
  nodes: TreemapNode[]
  total: number
  onOpen: (node: TreemapNode) => void
  height?: number | string
}) {
  const theme = useCurrentTheme()
  const book = useCategories()
  // The chart's size, to tell which tiles have room for a label.
  const box = useRef<HTMLDivElement>(null)
  const [area, setArea] = useState(0)
  useEffect(() => {
    const el = box.current
    if (!el) return
    const observer = new ResizeObserver(([entry]) => {
      const next = Math.round(entry.contentRect.width * entry.contentRect.height)
      setArea((a) => (Math.abs(a - next) > a * 0.05 ? next : a))
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const { option, byId } = useMemo(() => {
    const c = chrome(theme)
    const byId = new Map<string, TreemapNode>()
    const depthOf = new Map<string, number>()
    const data = tiles(nodes, book, theme, c, byId, depthOf)
    // A tile's area is about its share of the chart, less what the frames
    // and headers above it take (about a fifth per level). Squarified tiles
    // are near square, so the area says well enough whether a label fits;
    // one that doesn't is left out rather than cut to a stub.
    const roomFor = (id: string, px: number) => {
      const n = byId.get(id)
      if (!n || total <= 0) return false
      return (n.size / total) * area * 0.8 ** ((depthOf.get(id) ?? 1) - 1) >= px
    }
    const option: ChartOption = {
      tooltip: {
        ...c.tooltip,
        formatter: (p) => {
          const n = byId.get((p as unknown as { data: { id: string } }).data.id)
          if (!n) return ''
          if (n.kind === 'rest') {
            return `<b>${formatCount(n.items ?? 0)} smaller items</b><br/>${formatBytes(n.size)} together`
          }
          const look = book.look(n.categoryId)
          const group = book.groupOf(n.categoryId)
          const category = group ? `${escapeHtml(group.name)} › ${escapeHtml(look.name)}` : escapeHtml(look.name)
          return `<b>${escapeHtml(n.name)}</b><br/>${formatBytes(n.size)} · ${formatShare(n.size, total)} of this folder<br/><span style="color:${c.muted}">${n.kind === 'file' ? 'File' : 'Folder'} · ${category}</span><br/><span style="color:${c.muted};font-size:11px">${escapeHtml(n.path)}</span>`
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
          // Tiles under ~100 px² aren't drawn; a Folder under ~2,000 px² shows no insides.
          visibleMin: 100,
          childrenVisibleMin: 2000,
          emphasis: { disabled: true },
          label: {
            show: true,
            position: 'insideTopLeft',
            padding: [5, 7],
            fontFamily: chartFont,
            overflow: 'truncate',
            ellipsis: '…',
            formatter: (p) => {
              const id = (p.data as { id: string }).id
              const n = byId.get(id)
              if (!n || !roomFor(id, 2600)) return ''
              if (n.kind === 'rest') return `{size|${formatCount(n.items ?? 0)} more}`
              return roomFor(id, 5000)
                ? `{name|${richSafe(n.name)}}\n{size|${formatBytes(n.size)}}`
                : `{name|${richSafe(n.name)}}`
            },
          },
          upperLabel: {
            show: true,
            height: headerHeight(5),
            padding: [0, 7],
            fontFamily: chartFont,
            overflow: 'truncate',
            ellipsis: '…',
            formatter: (p) => {
              const id = (p.data as { id: string }).id
              const n = byId.get(id)
              if (!n || !roomFor(id, 3500)) return ''
              return roomFor(id, 9000) ? `{name|${richSafe(n.name)}}  {size|${formatBytes(n.size)}}` : `{name|${richSafe(n.name)}}`
            },
          },
          // levels[1] is the Folder's children, levels[2] theirs, and so on:
          // the frames and gaps get finer with depth.
          levels: [
            // The Folder itself: no frame, no header.
            { itemStyle: { gapWidth: 3, borderWidth: 0, borderColor: c.surface }, upperLabel: { show: false } },
            { itemStyle: { gapWidth: 2, borderWidth: 3, borderRadius: 5 }, upperLabel: { height: headerHeight(1) } },
            { itemStyle: { gapWidth: 1, borderWidth: 2, borderRadius: 4 }, upperLabel: { height: headerHeight(2) } },
            { itemStyle: { gapWidth: 1, borderWidth: 2, borderRadius: 3 }, upperLabel: { height: headerHeight(3) } },
            { itemStyle: { gapWidth: 1, borderWidth: 1, borderRadius: 2 }, upperLabel: { height: headerHeight(4) } },
          ],
          data,
        },
      ],
    }
    return { option, byId }
  }, [area, book, nodes, theme, total])

  return (
    <div ref={box} style={{ height }}>
      <EChart
      option={option}
      height="100%"
      label="What this folder holds, nested a few levels deep, sized by Size"
      onClick={(e) => {
        const node = e.data ? byId.get((e.data as { id: string }).id) : undefined
        if (node && node.kind !== 'rest') onOpen(node)
      }}
      />
    </div>
  )
}

/** Builds the tiles for nodes and records each by id in byId, and its depth (1 for a child) in depthOf. */
function tiles(
  nodes: TreemapNode[],
  book: CategoryBook,
  theme: Theme,
  c: Chrome,
  byId: Map<string, TreemapNode>,
  depthOf: Map<string, number>,
): Tile[] {
  let branch = 0
  const build = (n: TreemapNode, id: string, depth: number, tint: string | null): Tile => {
    byId.set(id, n)
    depthOf.set(id, depth)
    // The base colour: the category's; else the branch's muted tint (a new
    // one for each uncategorized top-level tile), a touch lighter each level
    // down so nested tiles stand off their frame.
    let base: string
    if (n.kind === 'rest') {
      base = c.grid
    } else if (n.categoryId !== null) {
      base = book.look(n.categoryId).colour
    } else {
      tint ??= uncategorizedTints[theme][branch++ % uncategorizedTints[theme].length]
      base = shade(tint, -0.05 * (depth - 1))
    }
    // Files sit back from folders: mixed a little with the surface.
    const fill = n.kind === 'file' ? mix(base, c.surface, 0.3) : base
    // A Folder's frame and header: its colour, deeper.
    const frame = shade(base, theme === 'dark' ? 0.38 : 0.22)
    const ink = labelInk(fill)
    const frameInk = labelInk(frame)
    const tile: Tile = {
      id,
      name: n.name,
      value: n.size,
      itemStyle: { color: fill, borderColor: frame },
      label: {
        rich: {
          name: { color: ink, fontSize: depth === 1 ? 13 : 12, fontWeight: 600, lineHeight: depth === 1 ? 18 : 16 },
          size: { color: n.kind === 'rest' ? c.muted : ink, fontSize: 11, opacity: 0.85, lineHeight: 15 },
        },
      },
      upperLabel: {
        // The line as tall as the header band (levels), so the text sits in its middle.
        rich: {
          name: { color: frameInk, fontSize: depth === 1 ? 12 : 11, fontWeight: 600, lineHeight: headerHeight(depth) },
          size: { color: frameInk, fontSize: 11, opacity: 0.75, lineHeight: headerHeight(depth) },
        },
      },
    }
    if (n.children && n.children.length > 0) {
      const childTint = n.categoryId === null ? tint : null
      tile.children = n.children.map((child, i) => build(child, `${id}.${i}`, depth + 1, childTint))
    }
    return tile
  }
  return nodes.map((n, i) => build(n, String(i), 1, null))
}

/** The height of a Folder's header at depth (1 for a child of the treemap's Folder). */
function headerHeight(depth: number): number {
  return depth <= 1 ? 22 : depth === 2 ? 19 : depth === 3 ? 17 : 16
}

/** A name made safe for ECharts rich text, whose markup is {style|text}. */
function richSafe(name: string): string {
  return name.replace(/[{}|]/g, ' ')
}
