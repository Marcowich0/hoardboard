// A stand-in for the hoardboard server, in the browser: the same Api, worked
// out from a FakeWorld (disks, files with inode numbers, categories) the way
// spec #1 says the server works it out. Later tickets replace it endpoint by
// endpoint (see ../index.ts); it only has to be believable, not fast.

import { addDays, dayOf, daysBetween } from '../../lib/days.ts'
import { ApiError } from '../errors.ts'
import { forecast } from './forecast.ts'
import type {
  Api,
  Categories,
  Category,
  CategoryInput,
  Disk,
  Entry,
  FileEntry,
  FolderInfo,
  FolderHistory,
  History,
  Overview,
  ParityDisk,
  Period,
  ScanStatus,
  Tree,
  UsedSplit,
} from '../types.ts'

/** One name of a file. Names with the same inode are hardlinks: one set of bytes. */
export interface FakeFile {
  path: string
  inode: number
  size: number
  disk: string
  /** The day the name appeared; days before FakeWorld.firstDay count as that day. */
  added: string
  /** The day the name disappeared, if it did. */
  removed?: string
}

export interface FakeDisk {
  name: string
  kind: Disk['kind']
  filesystem: string
  size: number
  /** Used space no file accounts for (the filesystem's own). */
  overhead: number
}

export interface FakeWorld {
  /** The first Scan's day: History starts here. */
  firstDay: string
  disks: FakeDisk[]
  parity: ParityDisk[]
  files: FakeFile[]
  /** In Rank order, highest first. */
  categories: { id: number; name: string; colour: string }[]
  assignments: { path: string; categoryId: number }[]
  scan: { finishedAt: string; durationMs: number }
}

export interface FakeOptions {
  now?: () => Date
  /** Delay before every answer, to show loading states. */
  latencyMs?: number
  /** How long "Scan now" runs. */
  scanMs?: number
}

const periodDays: Record<Period, number> = { '7d': 7, '30d': 30, '1y': 365 }

/** What "Scan now" does, in turn, so the prototype shows each outcome. */
const scanOutcomes: (string | null)[] = [null, 'Parity check in progress', null, 'Mover is running']

export function createFakeApi(world: FakeWorld, options: FakeOptions = {}): Api {
  const now = options.now ?? (() => new Date())
  const latencyMs = options.latencyMs ?? 0
  const scanMs = options.scanMs ?? 2500

  let categories = world.categories.map((c) => ({ ...c }))
  const assignments = new Map(world.assignments.map((a) => [a.path, a.categoryId]))
  let nextId = Math.max(0, ...categories.map((c) => c.id)) + 1

  let scanId = 1
  const scan: ScanStatus = {
    running: null,
    last: {
      id: scanId,
      startedAt: new Date(Date.parse(world.scan.finishedAt) - world.scan.durationMs).toISOString(),
      finishedAt: world.scan.finishedAt,
      durationMs: world.scan.durationMs,
    },
    skipped: null,
    next: new Date(Date.parse(world.scan.finishedAt) + 3600_000).toISOString(),
  }
  let scanPresses = 0

  const today = () => dayOf(now())

  // -- Who is where ----------------------------------------------------------

  /** The most specific Assignment covering a path, if any. */
  function assignmentOf(path: string): { path: string; categoryId: number } | null {
    let best: { path: string; categoryId: number } | null = null
    for (const [assigned, categoryId] of assignments) {
      if (within(assigned, path) && (!best || assigned.length > best.path.length)) {
        best = { path: assigned, categoryId }
      }
    }
    return best
  }

  function categoryOf(path: string): number | null {
    return assignmentOf(path)?.categoryId ?? null
  }

  /** Rank position; Uncategorized comes after every category. */
  function rankOf(categoryId: number | null): number {
    const index = categoryId === null ? -1 : categories.findIndex((c) => c.id === categoryId)
    return index < 0 ? categories.length : index
  }

  function aliveOn(day: string) {
    return (f: FakeFile) => clampDay(f.added) <= day && (f.removed === undefined || day < f.removed)
  }

  function clampDay(day: string): string {
    return day < world.firstDay ? world.firstDay : day
  }

  function filesOn(day: string): FakeFile[] {
    return world.files.filter(aliveOn(day))
  }

  // -- Series over days ------------------------------------------------------

  function days(): string[] {
    const count = daysBetween(world.firstDay, today()) + 1
    return Array.from({ length: Math.max(1, count) }, (_, i) => addDays(world.firstDay, i))
  }

  /**
   * Bytes per day per key: every inode with a name passing `include` counts
   * its size once a day, under the key `keyOf` gives its names alive that day
   * (null leaves it out).
   */
  function series<K>(
    include: (f: FakeFile) => boolean,
    keyOf: (alive: FakeFile[]) => K | null,
  ): { days: string[]; byKey: Map<K, number[]> } {
    const allDays = days()
    const last = allDays[allDays.length - 1]
    const deltas = new Map<K, number[]>()
    for (const names of byInode(world.files.filter(include)).values()) {
      // Between two events (a name added or removed) the key can't change.
      const events = new Set<string>()
      for (const f of names) {
        events.add(clampDay(f.added))
        if (f.removed !== undefined) events.add(f.removed)
      }
      const sorted = [...events].filter((d) => d <= last).sort()
      sorted.forEach((start, i) => {
        const key = keyOf(names.filter(aliveOn(start)))
        if (key === null) return
        let delta = deltas.get(key)
        if (!delta) {
          delta = new Array<number>(allDays.length + 1).fill(0)
          deltas.set(key, delta)
        }
        const from = daysBetween(world.firstDay, start)
        const to = i + 1 < sorted.length ? daysBetween(world.firstDay, sorted[i + 1]) : allDays.length
        delta[from] += names[0].size
        delta[to] -= names[0].size
      })
    }
    const byKey = new Map<K, number[]>()
    for (const [key, delta] of deltas) {
      let running = 0
      byKey.set(
        key,
        allDays.map((_, i) => (running += delta[i])),
      )
    }
    return { days: allDays, byKey }
  }

  /** The owner of a set of names in any split of Used space: the highest Rank among their categories. */
  function ownerOf(alive: FakeFile[]): number | 'uncategorized' | null {
    if (alive.length === 0) return null
    let best: number | null = null
    let bestRank = Infinity
    for (const f of alive) {
      const id = categoryOf(f.path)
      const rank = rankOf(id)
      if (rank < bestRank) {
        bestRank = rank
        best = id
      }
    }
    return best ?? 'uncategorized'
  }

  // -- Endpoints -------------------------------------------------------------

  function overview(): Overview {
    const day = today()
    const files = filesOn(day)
    const overhead = new Map(world.disks.map((d) => [d.name, d.overhead]))
    const usedOn = new Map(overhead)
    for (const names of byInode(files).values()) {
      usedOn.set(names[0].disk, (usedOn.get(names[0].disk) ?? 0) + names[0].size)
    }
    const disks: Disk[] = world.disks.map((d) => {
      const used = usedOn.get(d.name) ?? 0
      return { name: d.name, kind: d.kind, filesystem: d.filesystem, size: d.size, used, free: d.size - used }
    })
    const totals = {
      size: sumOf(disks, (d) => d.size),
      used: sumOf(disks, (d) => d.used),
      free: sumOf(disks, (d) => d.free),
    }
    const unaccounted = sumOf(world.disks, (d) => d.overhead)

    const split = series(() => true, ownerOf)
    const zero = split.days.map(() => 0)
    const trend = {
      days: split.days,
      categories: categories.map((c) => ({ categoryId: c.id, bytes: split.byKey.get(c.id) ?? zero })),
      uncategorized: split.byKey.get('uncategorized') ?? zero,
      unaccounted: split.days.map(() => unaccounted),
    }
    const lastOf = (values: number[]) => values[values.length - 1] ?? 0
    const donut: UsedSplit = {
      categories: trend.categories.map((c) => ({ categoryId: c.categoryId, bytes: lastOf(c.bytes) })),
      uncategorized: lastOf(trend.uncategorized),
      unaccounted,
    }
    const usedPerDay = split.days.map(
      (_, i) => unaccounted + trend.uncategorized[i] + trend.categories.reduce((sum, c) => sum + c.bytes[i], 0),
    )
    return {
      today: day,
      totals,
      disks,
      parity: world.parity,
      donut,
      trend,
      forecast: forecast(usedPerDay, totals.size, day),
    }
  }

  /**
   * A folder's numbers from the names below it: `under` alive today, `ever`
   * every name it ever held (for the change over the period).
   */
  function folderInfo(
    path: string,
    under: FakeFile[],
    ever: FakeFile[],
    all: Map<number, FakeFile[]>,
    period: Period,
  ): FolderInfo {
    const inodes = byInode(under)
    const size = sumOf([...inodes.values()], (names) => names[0].size)
    let sharedBytes = 0
    const disks = new Map<string, number>()
    for (const [inode, names] of inodes) {
      const everywhere = all.get(inode) ?? names
      if (everywhere.length > names.length) sharedBytes += names[0].size
      disks.set(names[0].disk, (disks.get(names[0].disk) ?? 0) + names[0].size)
    }
    const items = new Set<string>()
    for (const f of under) {
      let p = f.path
      while (p.length > path.length) {
        items.add(p)
        p = parentOf(p)
      }
    }
    const then = addDays(today(), -periodDays[period])
    let change: number | null = null
    if (then >= world.firstDay) {
      const before = byInode(ever.filter(aliveOn(then)))
      change = size - sumOf([...before.values()], (names) => names[0].size)
    }
    const assignment = assignmentOf(path)
    return {
      path,
      name: path.slice(path.lastIndexOf('/') + 1),
      size,
      change,
      items: items.size,
      sharedBytes,
      disks: [...disks].map(([disk, bytes]) => ({ disk, bytes })).sort((a, b) => b.bytes - a.bytes),
      category: assignment && { categoryId: assignment.categoryId, assignedPath: assignment.path },
    }
  }

  function tree(path: string, period: Period): Tree {
    const day = today()
    const files = filesOn(day)
    const ever = world.files.filter((f) => within(path, f.path) && f.path !== path)
    const under = ever.filter(aliveOn(day))
    if (path !== '' && under.length === 0) throw new ApiError(404, `No folder ${path}`)
    const all = byInode(files)

    // Every name below, by the child of `path` it is in.
    const groups = new Map<string, { under: FakeFile[]; ever: FakeFile[] }>()
    for (const f of ever) {
      const rest = path === '' ? f.path : f.path.slice(path.length + 1)
      const slash = rest.indexOf('/')
      const child = slash < 0 ? f.path : `${path === '' ? '' : `${path}/`}${rest.slice(0, slash)}`
      let group = groups.get(child)
      if (!group) {
        group = { under: [], ever: [] }
        groups.set(child, group)
      }
      group.ever.push(f)
      if (aliveOn(day)(f)) group.under.push(f)
    }
    const children: Entry[] = []
    for (const [childPath, group] of groups) {
      if (group.under.length === 0) continue
      if (group.under.length === 1 && group.under[0].path === childPath) {
        children.push(fileEntry(group.under[0], all))
      } else {
        const info = folderInfo(childPath, group.under, group.ever, all, period)
        children.push({
          kind: 'folder',
          name: info.name,
          path: childPath,
          size: info.size,
          change: info.change,
          items: info.items,
          sharedBytes: info.sharedBytes,
          disks: info.disks,
          categoryId: categoryOf(childPath),
        })
      }
    }
    children.sort((a, b) => (a.kind === b.kind ? b.size - a.size : a.kind === 'folder' ? -1 : 1))
    return { path, period, folder: folderInfo(path, under, ever, all, period), children }
  }

  function fileEntry(f: FakeFile, all: Map<number, FakeFile[]>): FileEntry {
    return {
      kind: 'file',
      name: f.path.slice(f.path.lastIndexOf('/') + 1),
      path: f.path,
      size: f.size,
      disk: f.disk,
      links: all.get(f.inode)?.length ?? 1,
      categoryId: categoryOf(f.path),
    }
  }

  function largest(path: string) {
    const files = filesOn(today())
    const all = byInode(files)
    const seen = new Set<number>()
    const result: FileEntry[] = []
    for (const f of files.filter((f) => within(path, f.path)).sort((a, b) => b.size - a.size)) {
      if (seen.has(f.inode)) continue
      seen.add(f.inode)
      result.push(fileEntry(f, all))
      if (result.length === 20) break
    }
    return { path, files: result }
  }

  function history(include: (f: FakeFile) => boolean): History {
    const s = series(include, (alive) => (alive.length > 0 ? 'size' : null))
    const sizes = s.byKey.get('size')
    return { points: s.days.map((day, i) => ({ day, size: sizes?.[i] ?? 0 })) }
  }

  /** A Folder's Size and Shared bytes per day: a name inside it is shared while the file also has one outside. */
  function folderHistory(path: string): FolderHistory {
    const s = series(
      () => true,
      (alive) => {
        const inside = alive.filter((f) => within(path, f.path)).length
        if (inside === 0) return null
        return inside < alive.length ? 'shared' : 'single'
      },
    )
    const shared = s.byKey.get('shared')
    const single = s.byKey.get('single')
    return {
      path,
      points: s.days.map((day, i) => {
        const sharedBytes = shared?.[i] ?? 0
        return { day, size: sharedBytes + (single?.[i] ?? 0), sharedBytes }
      }),
    }
  }

  function categoryList(): Categories {
    const files = filesOn(today())
    const inodes = byInode(files)
    const exists = (path: string) => files.some((f) => within(path, f.path) && f.path !== path)
    const result: Category[] = categories.map((c, i) => {
      let size = 0
      const shared = new Map<number | null, number>()
      for (const names of inodes.values()) {
        const ids = new Set(names.map((f) => categoryOf(f.path)))
        if (!ids.has(c.id)) continue
        size += names[0].size
        for (const other of ids) {
          if (other !== c.id) shared.set(other, (shared.get(other) ?? 0) + names[0].size)
        }
      }
      return {
        ...c,
        rank: i + 1,
        size,
        sharedWith: [...shared].map(([categoryId, bytes]) => ({ categoryId, bytes })).sort((a, b) => b.bytes - a.bytes),
        assignments: [...assignments]
          .filter(([, id]) => id === c.id)
          .map(([path]) => {
            const missing = !exists(path)
            const size = missing
              ? null
              : sumOf([...byInode(files.filter((f) => within(path, f.path))).values()], (n) => n[0].size)
            return { path, size, missing }
          })
          .sort((a, b) => a.path.localeCompare(b.path)),
      }
    })
    let uncategorized = 0
    for (const names of inodes.values()) {
      if (names.some((f) => categoryOf(f.path) === null)) uncategorized += names[0].size
    }
    return { categories: result, uncategorized: { size: uncategorized } }
  }

  function findCategory(id: number) {
    const found = categories.find((c) => c.id === id)
    if (!found) throw new ApiError(404, `No category ${id}`)
    return found
  }

  function checkInput(input: Partial<CategoryInput>, id?: number) {
    if (input.name !== undefined) {
      const name = input.name.trim()
      if (name === '') throw new ApiError(400, 'A category needs a name')
      if (name.toLowerCase() === 'uncategorized') throw new ApiError(400, 'Uncategorized is built in')
      if (categories.some((c) => c.id !== id && c.name.toLowerCase() === name.toLowerCase())) {
        throw new ApiError(409, `There is already a category called ${name}`)
      }
    }
    if (input.colour !== undefined && !/^#[0-9a-f]{6}$/i.test(input.colour)) {
      throw new ApiError(400, 'A colour is "#rrggbb"')
    }
  }

  function categoryById(id: number): Category {
    const found = categoryList().categories.find((c) => c.id === id)
    if (!found) throw new ApiError(404, `No category ${id}`)
    return found
  }

  function scanNow() {
    if (scan.running) return { status: structuredClone(scan), started: true }
    const reason = scanOutcomes[scanPresses++ % scanOutcomes.length]
    const at = now().toISOString()
    if (reason) {
      scan.skipped = { at, reason }
      return { status: structuredClone(scan), started: false }
    }
    const id = ++scanId
    scan.running = { id, startedAt: at }
    setTimeout(() => {
      const finishedAt = now()
      scan.last = {
        id,
        startedAt: at,
        finishedAt: finishedAt.toISOString(),
        durationMs: finishedAt.getTime() - Date.parse(at),
      }
      scan.running = null
      scan.skipped = null
      scan.next = new Date(finishedAt.getTime() + 3600_000).toISOString()
    }, scanMs)
    return { status: structuredClone(scan), started: true }
  }

  // -- The Api ---------------------------------------------------------------

  /** Answers after the latency, as a copy, so callers can't change the fake's state. */
  function answer<T>(work: () => T): Promise<T> {
    return new Promise((resolve, reject) => {
      const run = () => {
        try {
          resolve(structuredClone(work()))
        } catch (err) {
          reject(err)
        }
      }
      if (latencyMs > 0) setTimeout(run, latencyMs)
      else run()
    })
  }

  return {
    health: () => answer(() => ({ status: 'ok' as const })),
    overview: () => answer(overview),
    tree: (path, period) => answer(() => tree(path, period)),
    largest: (path) => answer(() => largest(path)),
    folderHistory: (path) => answer(() => folderHistory(path)),
    categoryHistory: (categoryId) =>
      answer(() => {
        findCategory(categoryId)
        return history((f) => categoryOf(f.path) === categoryId)
      }),
    categories: () => answer(categoryList),
    createCategory: (input) =>
      answer(() => {
        checkInput(input)
        const id = nextId++
        categories.push({ id, name: input.name.trim(), colour: input.colour })
        return categoryById(id)
      }),
    updateCategory: (id, input) =>
      answer(() => {
        const found = findCategory(id)
        checkInput(input, id)
        if (input.name !== undefined) found.name = input.name.trim()
        if (input.colour !== undefined) found.colour = input.colour
        return categoryById(id)
      }),
    deleteCategory: (id) =>
      answer(() => {
        findCategory(id)
        categories = categories.filter((c) => c.id !== id)
        for (const [path, categoryId] of assignments) if (categoryId === id) assignments.delete(path)
      }),
    reorderCategories: (ids) =>
      answer(() => {
        const sorted = [...ids].sort((a, b) => a - b).join()
        if (
          sorted !==
          categories
            .map((c) => c.id)
            .sort((a, b) => a - b)
            .join()
        ) {
          throw new ApiError(400, 'The order must name every category once')
        }
        categories = ids.map(findCategory)
      }),
    assign: (path, categoryId) =>
      answer(() => {
        findCategory(categoryId)
        assignments.set(path, categoryId)
      }),
    unassign: (path) =>
      answer(() => {
        if (!assignments.delete(path)) throw new ApiError(404, `${path} has no Assignment`)
      }),
    repoint: (from, to) =>
      answer(() => {
        const categoryId = assignments.get(from)
        if (categoryId === undefined) throw new ApiError(404, `${from} has no Assignment`)
        if (assignments.has(to)) throw new ApiError(409, `${to} already has an Assignment`)
        assignments.delete(from)
        assignments.set(to, categoryId)
      }),
    scanStatus: () => answer(() => scan),
    scanNow: () => answer(scanNow),
  }
}

// -- Helpers -------------------------------------------------------------------

/** Whether `path` is `folder` or inside it ("" holds everything). */
function within(folder: string, path: string): boolean {
  return folder === '' || path === folder || path.startsWith(`${folder}/`)
}

function parentOf(path: string): string {
  const slash = path.lastIndexOf('/')
  return slash < 0 ? '' : path.slice(0, slash)
}

function byInode(files: FakeFile[]): Map<number, FakeFile[]> {
  const result = new Map<number, FakeFile[]>()
  for (const f of files) {
    const names = result.get(f.inode)
    if (names) names.push(f)
    else result.set(f.inode, [f])
  }
  return result
}

function sumOf<T>(items: T[], value: (item: T) => number): number {
  return items.reduce((sum, item) => sum + value(item), 0)
}
