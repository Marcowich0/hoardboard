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
  overview: fake.overview, // fake until the category engine and API ticket
  tree: fake.tree,
  largest: fake.largest,
  folderHistory: fake.folderHistory,
  categoryHistory: fake.categoryHistory,
  categories: fake.categories,
  createCategory: fake.createCategory,
  updateCategory: fake.updateCategory,
  deleteCategory: fake.deleteCategory,
  reorderCategories: fake.reorderCategories,
  assign: fake.assign,
  unassign: fake.unassign,
  repoint: fake.repoint,
  scanStatus: fake.scanStatus,
  scanNow: fake.scanNow,
}

export { ApiError } from './errors.ts'
export type * from './types.ts'
