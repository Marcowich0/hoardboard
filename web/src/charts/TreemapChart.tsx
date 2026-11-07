import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { TreemapNode } from '../api/index.ts'
import { type CategoryBook, useCategories } from '../categories.ts'
import { labelInk, mix, shade, type Theme, uncategorizedTints } from '../lib/colours.ts'
import { formatBytes, formatCount, formatShare } from '../lib/format.ts'
import { escapeHtml } from '../lib/html.ts'
import { type FitNode, type FitRules, fitTreemap, tileBytes } from '../lib/treemap.ts'
import { useCurrentTheme } from '../theme.ts'
import { type Chrome, chartFont, chrome } from './chrome.ts'
import EChart, { type ChartOption } from './EChart.tsx'

/** A treemap tile: the node it draws, and the colour its label sits on. */
interface Tile {
  id: string
  name: string
  value: number
  itemStyle: { color: string; borderColor: string; borderWidth?: number; decal?: object }
  label: object
  upperLabel: object
  children?: Tile[]
}

/** What is drawn, for the chart's size: see fitTreemap. */
const rules: FitRules = {
  // About 33×33 px: smaller tiles join their Folder's rest, unless that
  // would leave most of the Folder to it; then down to about 17×17 px.
  minTile: 1100,
  minDenseTile: 300,
  denseShare: 0.4,
  // About 95×95 px inside: a smaller Folder stays one tile.
  minOpen: 9000,
  maxDepth: 4,
  frame: (depth) => ({ border: borderWidth(depth), header: headerHeight(depth) }),
}

/**
 * What a Folder holds, nested: each Folder's tile holds its own subfolders and
 * files, a few levels deep, so the big things show without opening anything
 * (the way WinDirStat draws a disk). Only what the chart has room for is
 * drawn; each Folder's small items are one quiet "rest" tile in its colour.
 * Tiles are sized by Size; an opened Folder's tile has a header with its
 * name. Colour is the category; what no category claims is drawn in muted
 * tints, one per top-level branch, so neighbouring folders still differ.
 * Files are lighter than folders. Clicking a folder opens it; clicking a file
 * opens its folder.
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
  // The chart's size, to fit the tiles to it: measured before the first
  // paint (a ResizeObserver's first call can come late), then on resizes.
  const box = useRef<HTMLDivElement>(null)
  const [area, setArea] = useState(0)
  useLayoutEffect(() => {
    const el = box.current
    if (!el) return
    const measure = () => {
      const next = Math.round(el.clientWidth * el.clientHeight)
      setArea((a) => (Math.abs(a - next) > a * 0.05 ? next : a))
    }
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const { option, byId } = useMemo(() => {
    const c = chrome(theme)
    const byId = new Map<string, FitNode>()
    const data = tiles(fitTreemap(nodes, area, rules), book, theme, c, byId)
    // Squarified tiles are near square, so a tile's area says well enough
    // whether a label fits; one that doesn't is left out, not cut to a stub.
    const roomFor = (id: string, px: number) => (byId.get(id)?.px ?? 0) >= px
    const option: ChartOption = {
      tooltip: {
        ...c.tooltip,
        formatter: (p) => {
          const n = byId.get((p as unknown as { data: { id: string } }).data.id)
          if (!n) return ''
          if (n.kind === 'rest') {
            return `<b>${formatCount(n.items ?? 0)} smaller items</b><br/>${formatBytes(tileBytes(n))} visible · ${formatBytes(n.countedElsewhereBytes ?? 0)} counted elsewhere`
          }
          const look = book.look(n.countedIn ? n.countedIn.categoryId : n.categoryId)
          const group = book.groupOf(n.countedIn ? n.countedIn.categoryId : n.categoryId)
          const category = group ? `${escapeHtml(group.name)} › ${escapeHtml(look.name)}` : escapeHtml(look.name)
          if (n.countedIn) {
            return `<b>${escapeHtml(n.name)}</b><br/>${formatBytes(tileBytes(n))} · counted in ${category}<br/><span style="color:${c.muted}">Seeding here · excluded from this folder’s Size</span><br/>${escapeHtml(n.countedIn.path)}`
          }
          const elsewhere = (n.countedElsewhereBytes ?? 0) > 0
            ? `<br/>${formatBytes(n.countedElsewhereBytes ?? 0)} seeding here · counted elsewhere` : ''
          return `<b>${escapeHtml(n.name)}</b><br/>${formatBytes(n.size)} · ${formatShare(n.size, total)} of this folder<br/><span style="color:${c.muted}">${n.kind === 'file' ? 'File' : 'Folder'} · ${category}</span><br/><span style="color:${c.muted};font-size:11px">${escapeHtml(n.path)}</span>${elsewhere}`
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
          // fitTreemap already left out what is too small.
          visibleMin: 0,
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
              if (!n || !roomFor(id, 2400)) return ''
              if (n.kind === 'rest') return roomFor(id, 5500) ? `{size|${formatCount(n.items ?? 0)} more}` : ''
              return roomFor(id, 5500)
                ? `{name|${richSafe(n.name)}}\n{size|${n.size === 0 && (n.countedElsewhereBytes ?? 0) > 0 ? '↗ Counted elsewhere' : formatBytes(n.size)}}`
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
              if (!n || !roomFor(id, 4000)) return ''
              return roomFor(id, 14000) ? `{name|${richSafe(n.name)}}  {size|${n.size === 0 && (n.countedElsewhereBytes ?? 0) > 0 ? '↗ Counted elsewhere' : formatBytes(n.size)}}` : `{name|${richSafe(n.name)}}`
            },
          },
          // levels[1] is the Folder's children, levels[2] theirs, and so on:
          // the frames and gaps get finer with depth.
          levels: [
            // The Folder itself: no frame, no header.
            { itemStyle: { gapWidth: 3, borderWidth: 0, borderColor: c.surface }, upperLabel: { show: false } },
            ...[1, 2, 3, 4].map((depth) => ({
              itemStyle: { gapWidth: depth === 1 ? 2 : 1, borderWidth: borderWidth(depth), borderRadius: depth === 1 ? 4 : 2 },
              upperLabel: { height: headerHeight(depth) },
            })),
          ],
          data,
        },
      ],
    }
    return { option, byId }
  }, [area, book, nodes, theme, total])

  return (
    <div ref={box} style={{ height }}>
      {/* Mounted once the box is measured, so the chart's first option has its tiles. */}
      {area > 0 && <EChart
      option={option}
      height="100%"
      label="What this folder holds; hatched seeding files count elsewhere"
      onClick={(e) => {
        const node = e.data ? byId.get((e.data as { id: string }).id) : undefined
        if (node && node.kind !== 'rest') onOpen({ ...node, children: undefined })
      }}
      />}
    </div>
  )
}

/** Builds the tiles for nodes and records each by id in byId. */
function tiles(nodes: FitNode[], book: CategoryBook, theme: Theme, c: Chrome, byId: Map<string, FitNode>): Tile[] {
  let branch = 0
  // within is the base colour of the Folder the node is in; null at the top.
  const build = (n: FitNode, id: string, depth: number, tint: string | null, within: string | null): Tile => {
    byId.set(id, n)
    // The base colour: the category's; else the branch's muted tint (a new
    // one for each uncategorized top-level tile), a touch lighter each level
    // down so nested tiles stand off their frame. The rest of a Folder is a
    // quiet version of the Folder's colour: part of it, too small to show.
    let base: string
    if (n.kind === 'rest') {
      base = within ? mix(within, c.surface, 0.55) : c.grid
    } else if (n.countedIn) {
      base = book.look(n.countedIn.categoryId).colour
    } else if (n.categoryId !== null) {
      base = book.look(n.categoryId).colour
    } else {
      tint ??= uncategorizedTints[theme][branch++ % uncategorizedTints[theme].length]
      base = shade(tint, -0.05 * (depth - 1))
    }
    // Files sit back from folders: mixed a little with the surface.
    const elsewhere = n.size === 0 && (n.countedElsewhereBytes ?? 0) > 0
    const fill = elsewhere ? mix(base, c.surface, 0.88) : n.kind === 'file' ? mix(base, c.surface, 0.3) : base
    // A Folder's frame and header: its colour, deeper.
    const frame = shade(base, theme === 'dark' ? 0.38 : 0.22)
    const ink = labelInk(fill)
    const frameInk = labelInk(frame)
    const tile: Tile = {
      id,
      name: n.name,
      value: tileBytes(n),
      itemStyle: { color: fill, borderColor: elsewhere ? base : frame, ...(elsewhere ? { borderWidth: 2, decal: { symbol: 'rect', dashArrayX: [1, 0], dashArrayY: [1, 7], rotation: -Math.PI / 4, color: mix(base, c.surface, 0.65) } } : {}) },
      label: {
        rich: {
          name: { color: ink, fontSize: depth === 1 ? 13 : 12, fontWeight: 600, lineHeight: depth === 1 ? 18 : 16 },
          size: { color: ink, fontSize: 11, opacity: n.kind === 'rest' ? 0.7 : 0.85, lineHeight: 15 },
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
      tile.children = n.children.map((child, i) => build(child, `${id}.${i}`, depth + 1, childTint, base))
    }
    return tile
  }
  return nodes.map((n, i) => build(n, String(i), 1, null, null))
}

/** The width of a Folder's frame at depth (1 for a child of the treemap's Folder). */
function borderWidth(depth: number): number {
  return depth <= 1 ? 3 : depth <= 3 ? 2 : 1
}

/** The height of a Folder's header at depth (1 for a child of the treemap's Folder). */
function headerHeight(depth: number): number {
  return depth <= 1 ? 22 : depth === 2 ? 19 : depth === 3 ? 17 : 16
}

/** A name made safe for ECharts rich text, whose markup is {style|text}. */
function richSafe(name: string): string {
  return name.replace(/[{}|]/g, ' ')
}
