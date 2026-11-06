import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { api, type Entry, type Period, type Tree } from '../api/index.ts'
import { useApi } from '../api/useApi.ts'
import { useCategories } from '../categories.ts'
import DiskSplitChart from '../charts/DiskSplitChart.tsx'
import HistoryChart from '../charts/HistoryChart.tsx'
import TreemapChart from '../charts/TreemapChart.tsx'
import AddToCategory from '../components/AddToCategory.tsx'
import {
  ArrowDownIcon,
  ArrowUpIcon,
  ChevronRightIcon,
  FileIcon,
  FolderIcon,
  LinkIcon,
  TagIcon,
} from '../components/icons.tsx'
import { Button, Card, ErrorNote, Segmented, Skeleton, Swatch } from '../components/ui.tsx'
import { formatBytes, formatChange, formatCount, formatShare } from '../lib/format.ts'
import { breadcrumbs, explorerHref, parentPath, periodFrom } from '../lib/paths.ts'
import { type SortDir, type SortKey, sortEntries } from '../lib/sort.ts'

const periodOptions: { value: Period; label: string }[] = [
  { value: '7d', label: '7 days' },
  { value: '30d', label: '30 days' },
  { value: '1y', label: '1 year' },
]

const periodWords: Record<Period, string> = { '7d': '7 days', '30d': '30 days', '1y': 'year' }

export default function Explorer() {
  const path = useParams()['*']?.replace(/\/+$/, '') ?? ''
  const [query] = useSearchParams()
  const period = periodFrom(query)
  const { data, error, loading, reload } = useApi(() => api.tree(path, period), [path, period])

  return (
    <div className="space-y-4 sm:space-y-5">
      <Breadcrumbs path={path} period={period} />
      {error && (!data || data.path !== path) ? (
        <ErrorNote error={error} onRetry={reload} />
      ) : !data || data.path !== path ? (
        <ExplorerSkeleton />
      ) : (
        <ExplorerBody tree={data} period={period} loading={loading} />
      )}
    </div>
  )
}

function Breadcrumbs({ path, period }: { path: string; period: Period }) {
  const crumbs = breadcrumbs(path)
  return (
    <nav aria-label="Folder" className="-mx-1 overflow-x-auto">
      <ol className="flex items-center gap-0.5 px-1 text-[13px] whitespace-nowrap">
        {crumbs.map((crumb, i) => (
          <li key={crumb.path} className="flex items-center gap-0.5">
            {i > 0 && <ChevronRightIcon size={14} className="text-muted" />}
            {i === crumbs.length - 1 ? (
              <span aria-current="page" className="rounded-md px-1.5 py-1 font-medium text-ink">
                {crumb.name}
              </span>
            ) : (
              <Link
                to={explorerHref(crumb.path, period)}
                className="rounded-md px-1.5 py-1 text-muted hover:bg-raised hover:text-ink"
              >
                {crumb.name}
              </Link>
            )}
          </li>
        ))}
      </ol>
    </nav>
  )
}

function ExplorerBody({ tree, period, loading }: { tree: Tree; period: Period; loading: boolean }) {
  const navigate = useNavigate()
  const [, setQuery] = useSearchParams()
  const book = useCategories()
  const [adding, setAdding] = useState<{ path: string; current: Tree['folder']['category'] } | null>(null)
  const folder = tree.folder
  const look = folder.category ? book.look(folder.category.categoryId) : null
  const isRoot = tree.path === ''

  const open = (entry: Entry) => {
    if (entry.kind === 'folder') navigate(explorerHref(entry.path, period))
  }
  const setPeriod = (p: Period) => {
    setQuery(p === '30d' ? {} : { period: p }, { replace: true })
  }

  return (
    <div className={`space-y-4 sm:space-y-5 loading-fade ${loading ? 'opacity-70' : ''}`}>
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div className="min-w-0">
          <h1 className="flex items-center gap-2.5 text-2xl font-semibold tracking-tight break-all sm:text-[28px]">
            {!isRoot && (
              <Link
                to={explorerHref(parentPath(tree.path), period)}
                aria-label="Up one folder"
                className="hidden rounded-lg p-1 text-muted hover:bg-raised hover:text-ink sm:block"
              >
                <ArrowUpIcon size={20} />
              </Link>
            )}
            {isRoot ? 'All shares' : folder.name}
          </h1>
          <p className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[13px] text-muted">
            {isRoot ? (
              'Every share, merged across all disks'
            ) : look && folder.category ? (
              <>
                <span className="inline-flex items-center gap-1.5 rounded-full border border-line bg-card px-2 py-0.5 text-ink">
                  <Swatch colour={look.colour} size={8} />
                  {look.name}
                </span>
                {folder.category.assignedPath !== tree.path && (
                  <span>
                    through <code className="font-mono text-[12px]">{folder.category.assignedPath}</code>
                  </span>
                )}
              </>
            ) : (
              <span className="inline-flex items-center gap-1.5 rounded-full border border-line bg-card px-2 py-0.5 text-ink-2">
                <Swatch colour={book.look(null).colour} size={8} />
                Uncategorized
              </span>
            )}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Segmented label="Change over" value={period} options={periodOptions} onChange={setPeriod} />
          {!isRoot && (
            <Button onClick={() => setAdding({ path: tree.path, current: folder.category })}>
              <TagIcon size={16} />
              Add to category…
            </Button>
          )}
        </div>
      </header>

      <div className="grid grid-cols-2 gap-px overflow-hidden rounded-2xl border border-line bg-line sm:grid-cols-4">
        <Fact label="Size" value={formatBytes(folder.size)} />
        <Fact
          label={`Change, last ${periodWords[period]}`}
          value={folder.change === null ? '—' : formatChange(folder.change)}
          detail={folder.change === null ? 'History is shorter than that' : undefined}
          tone={folder.change === null ? undefined : folder.change > 0 ? 'up' : folder.change < 0 ? 'down' : undefined}
        />
        <Fact label="Items" value={formatCount(folder.items)} detail="folders and files below" />
        <Fact
          label="Shared bytes"
          value={formatBytes(folder.sharedBytes)}
          detail={folder.sharedBytes > 0 ? 'also hardlinked outside it' : 'nothing hardlinked elsewhere'}
        />
      </div>

      <div className="grid gap-4 sm:gap-5 lg:grid-cols-3">
        <Card
          className="lg:col-span-2"
          title="What it holds"
          subtitle="Sized by Size, coloured by category; files are lighter. Click a folder to open it."
          bodyClassName="p-3 sm:p-4"
        >
          {tree.children.length > 0 ? (
            <div className="h-[300px] sm:h-[410px]">
              <TreemapChart entries={tree.children} total={folder.size} onOpen={open} />
            </div>
          ) : (
            <p className="py-16 text-center text-sm text-muted">This folder is empty</p>
          )}
        </Card>
        <div className="grid gap-4 sm:gap-5 sm:grid-cols-2 lg:grid-cols-1">
          <Card title="On disks" subtitle="Where Unraid put its bytes">
            {folder.disks.length > 0 ? (
              <DiskSplitChart disks={folder.disks} colour={look?.colour ?? book.look(null).colour} />
            ) : (
              <p className="text-sm text-muted">Nothing on any disk</p>
            )}
          </Card>
          <FolderHistory
            path={tree.path}
            name={isRoot ? 'All shares' : folder.name}
            colour={look?.colour ?? book.look(null).colour}
          />
        </div>
      </div>

      <Contents tree={tree} period={period} onAdd={(entry) => setAdding({ path: entry.path, current: null })} />

      <LargestFiles path={tree.path} />

      {adding && <AddToCategory path={adding.path} current={adding.current} onClose={() => setAdding(null)} />}
    </div>
  )
}

function Fact({ label, value, detail, tone }: { label: string; value: string; detail?: string; tone?: 'up' | 'down' }) {
  return (
    <div className="min-w-0 bg-card px-4 py-3.5">
      <p className="truncate text-[12px] font-medium text-muted">{label}</p>
      <p className="mt-1 flex items-center gap-1 text-xl font-semibold tracking-tight">
        {tone === 'up' && <ArrowUpIcon size={16} className="text-muted" />}
        {tone === 'down' && <ArrowDownIcon size={16} className="text-muted" />}
        {value}
      </p>
      {detail && <p className="mt-0.5 truncate text-[12px] text-muted">{detail}</p>}
    </div>
  )
}

function FolderHistory({ path, name, colour }: { path: string; name: string; colour: string }) {
  const { data } = useApi(() => api.folderHistory(path), [path])
  return (
    <Card title="History" subtitle="Its Size, one point a day since the first scan">
      {data ? (
        <HistoryChart history={data} colour={colour} name={name} height={170} />
      ) : (
        <Skeleton className="h-[170px]" />
      )}
    </Card>
  )
}

const columns: { key: SortKey; label: string; className: string }[] = [
  { key: 'name', label: 'Name', className: 'text-left' },
  { key: 'size', label: 'Size', className: 'text-right' },
  { key: 'items', label: 'Items', className: 'hidden text-right md:table-cell' },
  { key: 'change', label: 'Change', className: 'text-right' },
]

function Contents({ tree, period, onAdd }: { tree: Tree; period: Period; onAdd: (entry: Entry) => void }) {
  const book = useCategories()
  const [sort, setSort] = useState<{ key: SortKey; dir: SortDir }>({ key: 'size', dir: 'desc' })
  const [limit, setLimit] = useState(100)
  const rows = useMemo(() => sortEntries(tree.children, sort.key, sort.dir), [sort, tree.children])
  const folders = tree.children.filter((c) => c.kind === 'folder').length
  const files = tree.children.length - folders

  const toggle = (key: SortKey) =>
    setSort((s) =>
      s.key === key ? { key, dir: s.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: key === 'name' ? 'asc' : 'desc' },
    )

  return (
    <Card
      title="Contents"
      subtitle={`${formatCount(folders)} folders, ${formatCount(files)} files · change over the last ${periodWords[period]}`}
      bodyClassName="pt-3"
    >
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-y border-line text-[12px] text-muted">
              {columns.map((c) => (
                <th
                  key={c.key}
                  scope="col"
                  aria-sort={sort.key === c.key ? (sort.dir === 'asc' ? 'ascending' : 'descending') : undefined}
                  className={`px-2 py-2 font-medium first:pl-4 sm:first:pl-5 ${c.className}`}
                >
                  <button
                    type="button"
                    onClick={() => toggle(c.key)}
                    className={`inline-flex items-center gap-1 hover:text-ink ${sort.key === c.key ? 'text-ink' : ''}`}
                  >
                    {c.label}
                    {sort.key === c.key &&
                      (sort.dir === 'asc' ? <ArrowUpIcon size={12} /> : <ArrowDownIcon size={12} />)}
                  </button>
                </th>
              ))}
              <th scope="col" className="w-12 pr-4 sm:pr-5">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {rows.slice(0, limit).map((entry) => {
              const look = book.look(entry.categoryId)
              return (
                <tr key={entry.path} className="group hover:bg-raised/60">
                  <td className="max-w-0 py-2 pr-2 pl-4 sm:pl-5">
                    <div className="flex min-w-0 items-center gap-2.5">
                      <span className="relative shrink-0">
                        {entry.kind === 'folder' ? (
                          <FolderIcon size={18} className="text-muted" />
                        ) : (
                          <FileIcon size={18} className="text-muted" />
                        )}
                        <Swatch
                          colour={look.colour}
                          size={7}
                          className="absolute -right-0.5 -bottom-0.5 ring-2 ring-card"
                        />
                      </span>
                      {entry.kind === 'folder' ? (
                        <Link
                          to={explorerHref(entry.path, period)}
                          className="min-w-0 truncate font-medium hover:underline"
                          title={entry.name}
                        >
                          {entry.name}
                        </Link>
                      ) : (
                        <span className="min-w-0 truncate text-ink-2" title={entry.name}>
                          {entry.name}
                        </span>
                      )}
                      {entry.kind === 'file' && entry.links > 1 && (
                        <span
                          title={`${entry.links} names (hardlinked)`}
                          className="flex shrink-0 items-center gap-0.5 text-[11px] text-muted"
                        >
                          <LinkIcon size={13} />
                          {entry.links}
                        </span>
                      )}
                      {entry.kind === 'folder' && entry.sharedBytes > 0 && (
                        <span
                          title={`${formatBytes(entry.sharedBytes)} also hardlinked outside it`}
                          className="hidden shrink-0 items-center gap-0.5 text-[11px] text-muted sm:flex"
                        >
                          <LinkIcon size={13} />
                          {formatShare(entry.sharedBytes, entry.size)}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="w-28 px-2 py-2 text-right sm:w-40">
                    <div className="flex items-center justify-end gap-2.5">
                      <span
                        className="hidden h-1.5 w-14 overflow-hidden rounded-full bg-track sm:block"
                        aria-hidden="true"
                      >
                        <span
                          className="block h-full rounded-full"
                          style={{
                            width: `${Math.max(2, (entry.size / Math.max(1, tree.folder.size)) * 100)}%`,
                            backgroundColor: look.colour,
                          }}
                        />
                      </span>
                      <span className="tabular text-ink-2">{formatBytes(entry.size)}</span>
                    </div>
                  </td>
                  <td className="tabular hidden w-24 px-2 py-2 text-right text-muted md:table-cell">
                    {entry.kind === 'folder' ? formatCount(entry.items) : '—'}
                  </td>
                  <td className="tabular w-24 px-2 py-2 text-right sm:w-28">
                    <ChangeCell entry={entry} />
                  </td>
                  <td className="w-12 py-1 pr-3 text-right sm:pr-4">
                    {entry.kind === 'folder' && (
                      <button
                        type="button"
                        onClick={() => onAdd(entry)}
                        aria-label={`Add ${entry.name} to a category`}
                        title="Add to category…"
                        className="rounded-md p-1.5 text-muted opacity-100 hover:bg-card hover:text-ink sm:opacity-0 sm:group-hover:opacity-100 sm:focus:opacity-100"
                      >
                        <TagIcon size={16} />
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      {rows.length > limit && (
        <div className="border-t border-line px-4 py-3 text-center">
          <Button size="sm" variant="ghost" onClick={() => setLimit((l) => l + 200)}>
            Show more ({formatCount(rows.length - limit)} left)
          </Button>
        </div>
      )}
    </Card>
  )
}

function ChangeCell({ entry }: { entry: Entry }) {
  if (entry.kind === 'file') return <span className="text-muted">—</span>
  if (entry.change === null)
    return (
      <span className="text-muted" title="History is shorter than the period">
        —
      </span>
    )
  if (entry.change === 0) return <span className="text-muted">0</span>
  return <span className={entry.change > 0 ? 'text-ink' : 'text-ink-2'}>{formatChange(entry.change)}</span>
}

function LargestFiles({ path }: { path: string }) {
  const { data } = useApi(() => api.largest(path), [path])
  const book = useCategories()
  const [all, setAll] = useState(false)
  const files = data ? (all ? data.files : data.files.slice(0, 10)) : []
  return (
    <Card
      title="Largest files"
      subtitle="Anywhere below this folder, each physical file once"
      bodyClassName="pt-2 pb-2"
    >
      {!data ? (
        <div className="space-y-2 px-5 py-2">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-9" />
          ))}
        </div>
      ) : data.files.length === 0 ? (
        <p className="px-5 py-6 text-center text-sm text-muted">No files</p>
      ) : (
        <ol className="divide-y divide-line">
          {files.map((f, i) => {
            const rest = path === '' ? f.path : f.path.slice(path.length + 1)
            const folder = parentPath(rest)
            return (
              <li key={f.path} className="flex items-center gap-3 px-4 py-2 sm:px-5">
                <span className="tabular w-5 shrink-0 text-right text-[12px] text-muted">{i + 1}</span>
                <Swatch colour={book.look(f.categoryId).colour} size={8} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm" title={f.name}>
                    {f.name}
                  </p>
                  {folder && (
                    <Link
                      to={explorerHref(parentPath(f.path), '30d')}
                      className="block truncate text-[12px] text-muted hover:text-ink hover:underline"
                    >
                      {folder}
                    </Link>
                  )}
                </div>
                {f.links > 1 && (
                  <span
                    title={`${f.links} names (hardlinked)`}
                    className="hidden items-center gap-0.5 text-[11px] text-muted sm:flex"
                  >
                    <LinkIcon size={13} />
                    {f.links}
                  </span>
                )}
                <span className="hidden w-12 text-right text-[12px] text-muted sm:block">{f.disk}</span>
                <span className="tabular w-20 text-right text-sm text-ink-2">{formatBytes(f.size)}</span>
              </li>
            )
          })}
        </ol>
      )}
      {data && data.files.length > 10 && (
        <div className="border-t border-line px-4 pt-2 text-center">
          <Button size="sm" variant="ghost" onClick={() => setAll((a) => !a)}>
            {all ? 'Show the top 10' : `Show all ${data.files.length}`}
          </Button>
        </div>
      )}
    </Card>
  )
}

function ExplorerSkeleton() {
  return (
    <div className="space-y-5" aria-busy="true" aria-label="Loading">
      <Skeleton className="h-16 w-72" />
      <Skeleton className="h-24 rounded-2xl" />
      <div className="grid gap-5 lg:grid-cols-3">
        <Skeleton className="h-[440px] rounded-2xl lg:col-span-2" />
        <Skeleton className="h-[440px] rounded-2xl" />
      </div>
    </div>
  )
}
