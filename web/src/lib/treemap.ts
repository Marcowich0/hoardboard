// What of a nested treemap fits the chart: the server sends a Folder a few
// levels deep (up to ~1500 nodes), but how much of that is readable depends
// on the chart's size. fitTreemap keeps the tiles big enough to see, folds the
// rest of each Folder into one "rest" tile, and opens a Folder only when its
// tile has room to show what is inside.

import type { TreemapNode } from '../api/types.ts'

export interface FitRules {
  /** Smallest tile kept on its own, in px²; smaller ones join their Folder's rest. */
  minTile: number
  /**
   * Where that would leave more than `denseShare` of a Folder to its rest
   * (a folder of many similar items, such as one per movie), the smallest
   * tile kept instead: a grid of small tiles says more than one big rest.
   */
  minDenseTile: number
  denseShare: number
  /** Smallest inside (after frame and header) a Folder needs to be opened, in px². */
  minOpen: number
  /** Deepest level shown; 1 is the Folder's children. */
  maxDepth: number
  /** A Folder's frame width and header height at each depth (1 for a child). */
  frame: (depth: number) => { border: number; header: number }
}

/** A node that fits, with its estimated area in px². */
export type FitNode = Omit<TreemapNode, 'children'> & { px: number; children?: FitNode[] }

/** Visible area includes seeding files only in a torrents drill-down. */
export function tileBytes(node: TreemapNode): number {
  return node.size + (node.countedElsewhereBytes ?? 0)
}

/**
 * The part of nodes (the children of a Folder, which together fill `area`
 * px²) that fits. Each tile's area is its share of what its Folder's tiles
 * fill; a Folder's children fill its tile less its frame and header,
 * reckoned as if the tile were square, which squarified tiles nearly are.
 */
export function fitTreemap(nodes: TreemapNode[], area: number, rules: FitRules, depth = 1): FitNode[] {
  const sum = nodes.reduce((s, n) => s + tileBytes(n), 0)
  if (sum <= 0 || area <= 0) return []
  const small = nodes.reduce((s, n) => s + (n.kind === 'rest' || (tileBytes(n) / sum) * area < rules.minTile ? tileBytes(n) : 0), 0)
  const minTile = small / sum > rules.denseShare ? rules.minDenseTile : rules.minTile
  const out: FitNode[] = []
  let restSize = 0
  let restItems = 0
  let restElsewhere = 0
  for (const n of nodes) {
    const px = (tileBytes(n) / sum) * area
    if (n.kind === 'rest' || px < minTile) {
      restSize += n.size
      restElsewhere += n.countedElsewhereBytes ?? 0
      restItems += n.kind === 'rest' ? (n.items ?? 0) : 1
      continue
    }
    const { children, ...rest } = n
    const node: FitNode = { ...rest, px }
    if (children && children.length > 0 && depth < rules.maxDepth) {
      const { border, header } = rules.frame(depth)
      const side = Math.sqrt(px)
      const inside = Math.max(0, side - 2 * border) * Math.max(0, side - header - border)
      if (inside >= rules.minOpen) {
        const kids = fitTreemap(children, inside, rules, depth + 1)
        // Only a rest inside is no better than the closed tile.
        if (kids.some((k) => k.kind !== 'rest')) node.children = kids
      }
    }
    out.push(node)
  }
  if (restSize + restElsewhere > 0) {
    out.push({
      kind: 'rest',
      name: '',
      path: '',
      size: restSize,
      countedElsewhereBytes: restElsewhere,
      categoryId: null,
      items: restItems,
      px: ((restSize + restElsewhere) / sum) * area,
    })
  }
  return out
}
