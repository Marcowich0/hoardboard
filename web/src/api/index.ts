// THE data boundary. Pages get every piece of data through `api` and the
// types in ./types.ts, and nothing else: no page imports ./fake or calls
// fetch itself.
//
// Each endpoint below comes either from the real server (./http.ts) or from
// the in-browser fake (./fake). To move an endpoint to the server, add it to
// http.ts with the same signature and point its line here at it; pages don't
// change. When the last line points at http, delete ./fake.

import { createFakeApi } from './fake/fake.ts'
import { makeWorld } from './fake/world.ts'
import * as http from './http.ts'
import type { Api } from './types.ts'

const fake = createFakeApi(makeWorld(new Date()), { latencyMs: 180 })

export const api: Api = {
  health: http.health, // real
  // Real totals, disk bars, parity, donut and forecast; the trend stays
  // fake (with the fake's own category ids) until its ticket.
  overview: async () => {
    const [live, made] = await Promise.all([http.overview(), fake.overview()])
    return { ...made, ...live }
  },
  // Real folders, files, Size, items, Shared bytes, bytes per Disk, the
  // change over the period and each one's effective category (null for
  // Uncategorized).
  tree: http.tree,
  largest: http.largest, // real, with each file's effective category
  folderHistory: http.folderHistory, // real
  // Fake until category History: the fake only knows its own categories,
  // so a real one shows no points yet.
  categoryHistory: (categoryId) => fake.categoryHistory(categoryId).catch(() => ({ points: [] })),
  // Real: the category engine measures the latest Scan. Re-pointing comes
  // with a later ticket.
  categories: http.categories,
  createCategory: http.createCategory,
  updateCategory: http.updateCategory,
  deleteCategory: http.deleteCategory,
  reorderCategories: http.reorderCategories,
  assign: http.assign,
  unassign: http.unassign,
  repoint: fake.repoint,
  scanStatus: http.scanStatus, // real
  scanNow: http.scanNow, // real
}

export { ApiError } from './errors.ts'
export type * from './types.ts'
