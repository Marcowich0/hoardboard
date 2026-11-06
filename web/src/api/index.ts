// THE data boundary. Pages get every piece of data through `api` and the
// types in ./types.ts, and nothing else: no page calls fetch itself. Every
// endpoint comes from the hoardboard server (./http.ts).

import * as http from './http.ts'
import type { Api } from './types.ts'

export const api: Api = {
  health: http.health,
  // Totals, disk bars, parity, donut, trend and forecast.
  overview: http.overview,
  // Folders, files, Size, items, Shared bytes, bytes per Disk, the change
  // over the period and each one's effective category (null for
  // Uncategorized).
  tree: http.tree,
  largest: http.largest, // with each file's effective category
  folderHistory: http.folderHistory,
  // From folder History under today's Assignments.
  categoryHistory: http.categoryHistory,
  // The category engine measures the latest Scan; Missing Assignments are
  // flagged, and re-pointing one carries its folder's History over.
  categories: http.categories,
  createCategory: http.createCategory,
  updateCategory: http.updateCategory,
  deleteCategory: http.deleteCategory,
  reorderCategories: http.reorderCategories,
  assign: http.assign,
  unassign: http.unassign,
  repoint: http.repoint,
  scanStatus: http.scanStatus,
  scanNow: http.scanNow,
}

export { ApiError } from './errors.ts'
export type * from './types.ts'
