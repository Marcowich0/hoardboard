// The hoardboard server, endpoint by endpoint; ./index.ts puts them together as `api`.

import { ApiError } from './errors.ts'
import type {
  Categories,
  Category,
  CategoryHistory,
  CategoryInput,
  FolderHistory,
  HealthReport,
  Largest,
  Overview,
  Period,
  ScanRequestResult,
  ScanStatus,
  Tree,
  Treemap,
} from './types.ts'

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const response = await fetch(url, {
    method,
    headers:
      body === undefined
        ? { Accept: 'application/json' }
        : { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await response.text()
  let data: unknown = undefined
  try {
    data = text === '' ? undefined : JSON.parse(text)
  } catch {
    // Not JSON (a proxy's error page, say): the status says enough.
  }
  if (!response.ok) {
    const message =
      typeof data === 'object' && data !== null && 'error' in data && typeof data.error === 'string'
        ? data.error
        : `${method} ${url}: ${response.status} ${response.statusText}`
    throw new ApiError(response.status, message)
  }
  return data as T
}

export const getJSON = <T>(url: string) => request<T>('GET', url)
export const sendJSON = <T>(method: 'POST' | 'PUT' | 'PATCH' | 'DELETE', url: string, body?: unknown) =>
  request<T>(method, url, body)

/** GET /api/health. It answers 503 with a report when unhealthy, which is still a report. */
export async function health(): Promise<HealthReport> {
  try {
    return await getJSON<HealthReport>('/api/health')
  } catch (err) {
    if (err instanceof ApiError) return { status: 'unhealthy', error: err.message }
    throw err
  }
}

/**
 * GET /api/overview: the Disks, parity and totals, live from the filesystems;
 * the donut, that Used space split by category (Shared bytes to the higher
 * Rank); the same split per day (the trend); and the forecast.
 */
export const overview = () => getJSON<Overview>('/api/overview')

/** GET /api/tree: a Folder of the latest Scan, its subfolders and its files. */
export const tree = (path: string, period: Period) =>
  getJSON<Tree>(`/api/tree?${new URLSearchParams({ path, period }).toString()}`)

/** GET /api/treemap: a Folder of the latest Scan, nested a few levels deep, the largest first. */
export const treemap = (path: string) =>
  getJSON<Treemap>(`/api/treemap?${new URLSearchParams({ path }).toString()}`)

/**
 * GET /api/largest: the 20 largest files anywhere below a Folder of the latest
 * Scan, largest first; a hardlinked file once, by its first path in name order.
 */
export const largest = (path: string) => getJSON<Largest>(`/api/largest?${new URLSearchParams({ path }).toString()}`)

/** GET /api/history?path=: a Folder's Size and Shared bytes per day, from the first Scan that saw it to today. */
export const folderHistory = (path: string) =>
  getJSON<FolderHistory>(`/api/history?${new URLSearchParams({ path }).toString()}`)

/** GET /api/history?category=: a category's Size per day, from the first Scan to today, under today's Assignments. */
export const categoryHistory = (categoryId: number) =>
  getJSON<CategoryHistory>(`/api/history?${new URLSearchParams({ category: String(categoryId) }).toString()}`)

/** GET /api/scan: whether a Scan runs, the last one that finished, the latest skip and the next one due. */
export const scanStatus = () => getJSON<ScanStatus>('/api/scan')

/**
 * POST /api/scan: starts a Scan, or joins the one already running; during array
 * work (parity check, rebuild, mover) it is skipped, `started` false.
 */
export const scanNow = () => sendJSON<ScanRequestResult>('POST', '/api/scan')

/** GET /api/categories: every category in Rank order, measured in the latest Scan, and Uncategorized. */
export const categories = () => getJSON<Categories>('/api/categories')

/** POST /api/categories: a new category, ranked below the others. */
export const createCategory = (input: CategoryInput) => sendJSON<Category>('POST', '/api/categories', input)

/** PATCH /api/categories/{id}: renames, recolours and/or moves a category into a group (or out). */
export const updateCategory = (id: number, input: Partial<CategoryInput>) =>
  sendJSON<Category>('PATCH', `/api/categories/${id}`, input)

/** DELETE /api/categories/{id}: its folders go back to Uncategorized (or the category above them). */
export const deleteCategory = (id: number) => sendJSON<void>('DELETE', `/api/categories/${id}`)

/** PUT /api/categories/order: sets every category's Rank, highest first. */
export const reorderCategories = (ids: number[]) => sendJSON<void>('PUT', '/api/categories/order', { ids })

/** POST /api/assignments: puts a Folder into a category, replacing its own Assignment. */
export const assign = (path: string, categoryId: number) =>
  sendJSON<void>('POST', '/api/assignments', { path, categoryId })

/** DELETE /api/assignments?path=: removes a Folder's Assignment. */
export const unassign = (path: string) =>
  sendJSON<void>('DELETE', `/api/assignments?${new URLSearchParams({ path }).toString()}`)

/**
 * POST /api/assignments/repoint: moves a Missing Assignment to the folder it is
 * called now; that folder's History before it appeared becomes the old one's.
 */
export const repoint = (from: string, to: string) =>
  sendJSON<void>('POST', '/api/assignments/repoint', { from, to })
