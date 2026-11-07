import { useState } from 'react'
import { api } from '../api/index.ts'
import { useApi } from '../api/useApi.ts'
import { useCategories } from '../categories.ts'
import { formatBytes } from '../lib/format.ts'
import { breadcrumbs } from '../lib/paths.ts'
import { ChevronRightIcon, FolderIcon } from './icons.tsx'
import { Button, ErrorNote, Modal, Skeleton, Swatch } from './ui.tsx'

/**
 * Picks a Folder from the tree, so nobody types a path: browse down from the
 * shares, then choose the folder you are in.
 */
export default function FolderPicker({
  title,
  start = '',
  confirm,
  note,
  onPick,
  onClose,
}: {
  title: string
  start?: string
  confirm: string
  note?: string
  onPick: (path: string) => void | Promise<void>
  onClose: () => void
}) {
  const [path, setPath] = useState(start)
  const [busy, setBusy] = useState(false)
  const book = useCategories()
  const { data, error, loading, reload } = useApi(() => api.tree(path, '30d'), [path])
  const folders = data?.path === path ? data.children.filter((c) => c.kind === 'folder') : []

  const pick = async () => {
    setBusy(true)
    try {
      await onPick(path)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={title}
      onClose={onClose}
      wide
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={path === '' || busy} onClick={() => void pick()}>
            {confirm}
          </Button>
        </>
      }
    >
      {note && <p className="mb-3 text-[13px] text-muted">{note}</p>}
      <nav aria-label="Folder" className="mb-3 flex flex-wrap items-center gap-1 text-[13px]">
        {breadcrumbs(path).map((crumb, i, all) => (
          <span key={crumb.path} className="flex items-center gap-1">
            {i > 0 && <ChevronRightIcon size={14} className="text-muted" />}
            <button
              type="button"
              onClick={() => setPath(crumb.path)}
              className={`rounded px-1 py-0.5 ${i === all.length - 1 ? 'font-semibold text-ink' : 'text-muted hover:text-ink'}`}
            >
              {crumb.name}
            </button>
          </span>
        ))}
      </nav>
      {path !== '' && (
        <div className="mb-3 flex items-center gap-2 rounded-lg border border-line bg-sunken px-3 py-2 text-[13px]">
          <span className="text-muted">Selected</span>
          <code className="min-w-0 truncate font-mono text-ink">{path}</code>
        </div>
      )}
      {error && <ErrorNote error={error} onRetry={reload} />}
      {!data && loading && (
        <div className="space-y-1.5">
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-10" />
          ))}
        </div>
      )}
      {data && (
        <ul
          className={`divide-y divide-line rounded-xl border border-line ${loading ? 'opacity-60' : ''} loading-fade`}
        >
          {folders.length === 0 && (
            <li className="px-3 py-6 text-center text-[13px] text-muted">No folders below this one</li>
          )}
          {folders.map((f) => {
            const look = book.look(f.kind === 'folder' ? f.categoryId : null)
            return (
              <li key={f.path}>
                <button
                  type="button"
                  onClick={() => setPath(f.path)}
                  className="flex w-full items-center gap-3 px-3 py-2.5 text-left text-sm hover:bg-raised"
                >
                  <FolderIcon size={18} className="shrink-0 text-muted" />
                  <span className="min-w-0 flex-1 truncate">{f.name}</span>
                  <span className="hidden items-center gap-1.5 text-[12px] text-muted sm:flex">
                    <Swatch colour={look.colour} size={8} />
                    {look.name}
                  </span>
                  <span className="tabular w-20 text-right text-[13px] text-ink-2">{formatBytes(f.size)}</span>
                  <ChevronRightIcon size={16} className="shrink-0 text-muted" />
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </Modal>
  )
}
