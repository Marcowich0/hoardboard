// The JSON the hoardboard API sends and accepts, one section per endpoint of
// spec #1. Pages use only these shapes, through `api` in ./index.ts. Keep
// them in step with the Go handlers (internal/server).
//
// Sizes are bytes (numbers: JSON numbers are exact up to 8 PiB). Days are
// "YYYY-MM-DD", times are RFC 3339. Paths are Share paths relative to the
// merged tree, without a leading slash: "" is the root, "data/media/movies"
// a Folder. Responses name categories by id only; their names and colours
// come from GET /api/categories (see ../categories.tsx).

// GET /api/health ------------------------------------------------------------

export interface HealthReport {
  status: 'ok' | 'unhealthy'
  error?: string
  revision?: string
}

// GET /api/overview ----------------------------------------------------------

export interface Overview {
  /** The server's day, the last point of every series. */
  today: string
  /** Data disks plus the cache pool, live from the filesystems. */
  totals: { size: number; used: number; free: number }
  disks: Disk[]
  parity: ParityDisk[]
  /** Used space split into parts that sum exactly to totals.used. */
  donut: UsedSplit
  /** The same split per day, oldest first. */
  trend: Trend
  /** Null with fewer than 7 days of History. */
  forecast: Forecast | null
  /** The daily points of total Used space the forecast was fitted to (at most 90). */
  forecastDays: number
}

export interface Disk {
  /** "disk1"… or "cache". */
  name: string
  kind: 'data' | 'cache'
  filesystem: string
  size: number
  used: number
  free: number
  /**
   * This Disk's Used space split like the donut, from the bytes the latest
   * Scan found on it (a hardlinked file is on one Disk only). The parts add
   * up to `used`.
   */
  split: UsedSplit
}

/** Shown for information only: parity is never usable space. */
export interface ParityDisk {
  name: string
  size: number
  status: 'valid' | 'invalid' | 'missing'
  /** The day the last parity check finished, if Unraid knows. */
  lastCheck: string | null
}

/**
 * Used space in parts: a byte hardlinked into several categories goes to the
 * highest-ranked one, so the parts never overlap. They add up to Used space
 * exactly: when the Scan saw more, the categories and Uncategorized shrink in
 * proportion and Unaccounted is 0.
 */
export interface UsedSplit {
  /** Every category, in Rank order. */
  categories: { categoryId: number; bytes: number }[]
  uncategorized: number
  unaccounted: number
}

/**
 * Used space per day from the first Scan to today, split like the donut under
 * today's Assignments and Rank: each day's parts sum to that day's Used space,
 * the last day's are the donut. A day whose Used space wasn't recorded has
 * its parts as scanned and Unaccounted 0.
 */
export interface Trend {
  days: string[]
  /** One series per category in Rank order, then Uncategorized and Unaccounted. */
  categories: { categoryId: number; bytes: number[] }[]
  uncategorized: number[]
  unaccounted: number[]
}

export interface Forecast {
  /** The fitted growth over the last 90 days, per average month (365.25/12 days). */
  bytesPerMonth: number
  /** The day the fitted line reaches the total size; null when it is flat or falling. */
  fullOn: string | null
}

// GET /api/tree?path=&period= ------------------------------------------------

export type Period = '7d' | '30d' | '1y'

export interface Tree {
  path: string
  period: Period
  folder: FolderInfo
  /** The Folder's subfolders, then its files (latest Scan only). */
  children: Entry[]
}

export interface FolderInfo {
  path: string
  /** "" for the root. */
  name: string
  size: number
  /** Size now minus Size one period ago; null when History doesn't reach back that far. */
  change: number | null
  /** Folders and files below it, at any depth. */
  items: number
  /** Bytes also reachable from outside this folder through hardlinks. */
  sharedBytes: number
  /** Visible seeding bytes counted only toward a non-torrents Folder. */
  countedElsewhereBytes?: number
  /** Where its bytes are, largest first. */
  disks: { disk: string; bytes: number }[]
  /** The category its bytes fall into, and the Assignment that decides it. */
  category: { categoryId: number; assignedPath: string } | null
}

export type Entry = FolderEntry | FileEntry

export interface FolderEntry {
  kind: 'folder'
  name: string
  path: string
  size: number
  change: number | null
  items: number
  sharedBytes: number
  /** Visible seeding bytes counted only toward a non-torrents Folder. */
  countedElsewhereBytes?: number
  disks: { disk: string; bytes: number }[]
  /** Its effective category (most specific Assignment); null is Uncategorized. */
  categoryId: number | null
}

export interface FileEntry {
  kind: 'file'
  name: string
  path: string
  size: number
  disk: string
  /** How many names the file has; above 1 it is hardlinked. */
  links: number
  /** Present only for a torrents name counted toward another Folder. */
  countedIn?: CountedIn
  categoryId: number | null
}

export interface CountedIn {
  path: string
  categoryId: number | null
}

// GET /api/treemap?path= ----------------------------------------------------

/**
 * A Folder of the latest Scan as a nested treemap: its children, and theirs,
 * the largest Folders opened first, up to about 3000 nodes. Items under
 * 1/20000 of the Folder fold into one "rest" node per Folder. The chart
 * shows what fits its size (lib/treemap.ts).
 */
export interface Treemap {
  countedElsewhereBytes?: number
  path: string
  size: number
  /** Largest first. */
  children: TreemapNode[]
}

export interface TreemapNode {
  /** Extra area for seeding names when drilling into torrents; excluded from Size. */
  countedElsewhereBytes?: number
  countedIn?: CountedIn
  kind: 'folder' | 'file' | 'rest'
  /** "" for the rest. */
  name: string
  path: string
  size: number
  /** Its effective category; null is Uncategorized (and the rest). */
  categoryId: number | null
  /** For the rest: how many items it stands for. */
  items?: number
  /** A Folder the treemap opened; a closed one has none. */
  children?: TreemapNode[]
}

// GET /api/largest?path= -----------------------------------------------------

export interface Largest {
  path: string
  files: FileEntry[]
}

// GET /api/history?path= and ?category= --------------------------------------

export interface History {
  /** One point per day from the first Scan to today. */
  points: { day: string; size: number }[]
}

/**
 * GET /api/history?path=: a Folder's History, one point per day from the
 * first Scan that saw it to today, each day carrying the last Scan's values
 * forward. A deleted Folder keeps its History, at zero since it went.
 */
export interface FolderHistory extends History {
  path: string
  points: { day: string; size: number; sharedBytes: number }[]
}

/**
 * GET /api/history?category=: a category's full Size per day, each physical
 * file once, computed from History under today's Assignments, so a
 * reassigned folder changes the past as well.
 */
export interface CategoryHistory extends History {
  categoryId: number
}

// /api/categories ------------------------------------------------------------

export interface Categories {
  /** In Rank order, highest first. */
  categories: Category[]
  uncategorized: { size: number }
}

export interface Category {
  id: number
  name: string
  /** "#rrggbb". */
  colour: string
  /**
   * 1 is the highest. Ranks run in group order: a group, then the
   * categories in it, then the next top-level category.
   */
  rank: number
  /** The group it is in; null at the top level. Groups are one level deep. */
  groupId: number | null
  /**
   * The full Size, Shared bytes included, each physical file counted once.
   * A group's covers the categories in it too.
   */
  size: number
  /**
   * "also in X": bytes this category (a group: with the categories in it)
   * shares with others. categoryId null is Uncategorized.
   */
  sharedWith: { categoryId: number | null; bytes: number }[]
  assignments: Assignment[]
}

export interface Assignment {
  path: string
  /** Null when the folder is Missing. */
  size: number | null
  /** The folder no longer exists (renamed or deleted since it was assigned). */
  missing: boolean
}

/** POST /api/categories, PATCH /api/categories/{id} */
export interface CategoryInput {
  name: string
  colour: string
  /** The group to put it in, null for the top level; left out, it stays where it is. */
  groupId?: number | null
}

// /api/scan -------------------------------------------------------------------

export interface ScanStatus {
  /** Set while a Scan runs. */
  running: { id: number; startedAt: string } | null
  /** The last Scan that finished. */
  last: { id: number; startedAt: string; finishedAt: string; durationMs: number } | null
  /** The latest skipped Scan, if it came after `last`. */
  skipped: { at: string; reason: string } | null
  /** When the next scheduled Scan is due; null while nothing is scheduled. */
  next: string | null
}

/**
 * POST /api/scan: started (or joined the Scan already running), or skipped
 * with the reason. It answers at once; GET /api/scan tells when it ends.
 */
export interface ScanRequestResult {
  status: ScanStatus
  started: boolean
}

// The whole API, as the pages see it. -----------------------------------------

export interface Api {
  health(): Promise<HealthReport>
  overview(): Promise<Overview>
  tree(path: string, period: Period): Promise<Tree>
  treemap(path: string): Promise<Treemap>
  largest(path: string): Promise<Largest>
  folderHistory(path: string): Promise<FolderHistory>
  categoryHistory(categoryId: number): Promise<History>
  categories(): Promise<Categories>
  createCategory(input: CategoryInput): Promise<Category>
  updateCategory(id: number, input: Partial<CategoryInput>): Promise<Category>
  deleteCategory(id: number): Promise<void>
  /** PUT /api/categories/order with `{ ids }`: every category id once, highest Rank first. */
  reorderCategories(ids: number[]): Promise<void>
  /** POST /api/assignments: puts a folder into a category (replacing its current one). */
  assign(path: string, categoryId: number): Promise<void>
  /** DELETE /api/assignments?path= */
  unassign(path: string): Promise<void>
  /**
   * POST /api/assignments/repoint with `{ from, to }`: moves a Missing Assignment to the folder
   * it is called now (in the latest Scan, with no Assignment of its own). Before that folder
   * appeared, its History is the old folder's.
   */
  repoint(from: string, to: string): Promise<void>
  scanStatus(): Promise<ScanStatus>
  scanNow(): Promise<ScanRequestResult>
}
