import {
  closestCenter,
  DndContext,
  type DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { type ReactNode, useEffect, useId, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { api, type Category, type CategoryInput } from '../api/index.ts'
import { change, useApi } from '../api/useApi.ts'
import { useCategories } from '../categories.ts'
import HistoryChart from '../charts/HistoryChart.tsx'
import { CategoryFields } from '../components/CategoryForm.tsx'
import FolderPicker from '../components/FolderPicker.tsx'
import {
  CategoriesIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CloseIcon,
  FolderIcon,
  GripIcon,
  LinkIcon,
  PencilIcon,
  PlusIcon,
  TrashIcon,
  WarningIcon,
} from '../components/icons.tsx'
import { Button, Card, Modal, Skeleton, Swatch } from '../components/ui.tsx'
import { suggestColour } from '../lib/colours.ts'
import { formatBytes, formatCount } from '../lib/format.ts'
import { explorerHref, parentPath } from '../lib/paths.ts'
import Page from './Page.tsx'

export default function Categories() {
  const book = useCategories()
  const params = useParams()
  const navigate = useNavigate()
  const [creating, setCreating] = useState(false)
  const selectedId = params.id === undefined ? null : Number(params.id)
  const selected = book.categories.find((c) => c.id === selectedId) ?? null

  // On a wide screen with nothing chosen, show the first category.
  useEffect(() => {
    if (selectedId === null && book.categories.length > 0 && window.matchMedia('(min-width: 1024px)').matches) {
      navigate(`/categories/${book.categories[0].id}`, { replace: true })
    }
  }, [book.categories, navigate, selectedId])

  const newButton = (
    <Button variant="primary" onClick={() => setCreating(true)}>
      <PlusIcon size={16} />
      New category
    </Button>
  )

  if (!book.loaded) {
    return (
      <Page title="Categories">
        <div className="grid gap-5 lg:grid-cols-[22rem_1fr]">
          <Skeleton className="h-96 rounded-2xl" />
          <Skeleton className="hidden h-96 rounded-2xl lg:block" />
        </div>
      </Page>
    )
  }

  return (
    <Page
      title={<span className={selectedId !== null ? 'max-lg:hidden' : ''}>Categories</span>}
      actions={<span className={selectedId !== null ? 'max-lg:hidden' : ''}>{newButton}</span>}
    >
      {book.categories.length === 0 ? (
        <EmptyState onCreate={() => setCreating(true)} uncategorized={book.uncategorizedSize} />
      ) : (
        <div className="grid items-start gap-4 sm:gap-5 lg:grid-cols-[22rem_1fr]">
          <div className={selectedId !== null ? 'max-lg:hidden' : ''}>
            <RankList selectedId={selectedId} />
          </div>
          <div className={selectedId === null ? 'max-lg:hidden' : ''}>
            {selected ? (
              <CategoryDetail key={selected.id} category={selected} />
            ) : selectedId !== null ? (
              <Card>
                <p className="text-sm text-muted">This category no longer exists.</p>
                <Link to="/categories" className="mt-2 inline-block text-sm text-focus underline">
                  All categories
                </Link>
              </Card>
            ) : null}
          </div>
        </div>
      )}
      {creating && (
        <CategoryEditor
          title="New category"
          initial={{ name: '', colour: suggestColour(book.categories.map((c) => c.colour)), groupId: null }}
          confirm="Create"
          onSave={async (input) => {
            const created = await change(api.createCategory(input))
            navigate(`/categories/${created.id}`)
          }}
          onClose={() => setCreating(false)}
        />
      )}
    </Page>
  )
}

function EmptyState({ onCreate, uncategorized }: { onCreate: () => void; uncategorized: number }) {
  return (
    <Card bodyClassName="px-6 py-14 text-center">
      <span className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-raised text-muted">
        <CategoriesIcon size={22} />
      </span>
      <h2 className="text-lg font-semibold tracking-tight">No categories yet</h2>
      <p className="mx-auto mt-1.5 max-w-md text-sm text-muted">
        Group folders from any share into categories such as Movies, TV or Photos, to see what your space goes to and
        how each one grows. Until then, all {formatBytes(uncategorized)} is Uncategorized.
      </p>
      <Button variant="primary" className="mt-5" onClick={onCreate}>
        <PlusIcon size={16} />
        Create your first category
      </Button>
    </Card>
  )
}

// -- The Rank list ---------------------------------------------------------------

function RankList({ selectedId }: { selectedId: number | null }) {
  const book = useCategories()
  // A dragged order shows at once, until the server's order catches up; a
  // category added, deleted or regrouped meanwhile makes it stale, so the
  // server's wins.
  const [pending, setPending] = useState<{ order: number[]; server: string } | null>(null)
  const serverOrder = book.categories.map((c) => c.id)
  const serverKey = book.categories.map((c) => `${c.id}<${c.groupId}`).join(',')
  const order = pending && pending.server === serverKey ? pending.order : serverOrder
  const byId = useMemo(() => new Map(book.categories.map((c) => [c.id, c])), [book.categories])
  const largest = Math.max(1, ...book.categories.map((c) => c.size), book.uncategorizedSize)
  const tops = order.filter((id) => byId.get(id)?.groupId === null)
  const membersOf = (id: number) => order.filter((m) => byId.get(m)?.groupId === id)

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  // A top-level category moves with its group; a category in a group moves
  // within it. Moving into or out of a group is in the category's Edit.
  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) return
    const from = byId.get(Number(active.id))
    let to = byId.get(Number(over.id))
    // A group dragged over another group's category: over that group.
    if (from?.groupId === null && to?.groupId != null) to = byId.get(to.groupId)
    if (!from || !to || from.id === to.id || from.groupId !== to.groupId) return
    let next: number[]
    if (from.groupId === null) {
      const moved = arrayMove(tops, tops.indexOf(from.id), tops.indexOf(to.id))
      next = moved.flatMap((id) => [id, ...membersOf(id)])
    } else {
      const members = membersOf(from.groupId)
      const moved = arrayMove(members, members.indexOf(from.id), members.indexOf(to.id))
      next = tops.flatMap((id) => [id, ...(id === from.groupId ? moved : membersOf(id))])
    }
    setPending({ order: next, server: serverKey })
    change(api.reorderCategories(next)).catch(() => setPending(null))
  }

  return (
    <Card
      title="Rank"
      subtitle="Drag to reorder. Where categories share bytes, the higher one owns them in the overview. A group keeps its categories under it."
      bodyClassName="p-2 sm:p-3"
    >
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
        <SortableContext items={tops} strategy={verticalListSortingStrategy}>
          <ol className="space-y-1">
            {tops.map((id) => {
              const category = byId.get(id)
              if (!category) return null
              const members = membersOf(id)
              return (
                <RankRow
                  key={id}
                  category={category}
                  rank={order.indexOf(id) + 1}
                  selected={id === selectedId}
                  largest={largest}
                  members={members.length}
                >
                  {members.length > 0 && (
                    <SortableContext items={members} strategy={verticalListSortingStrategy}>
                      <ol className="mt-1 ml-5 space-y-1 border-l border-line pl-1.5">
                        {members.map((m) => {
                          const member = byId.get(m)
                          return member ? (
                            <RankRow
                              key={m}
                              category={member}
                              rank={order.indexOf(m) + 1}
                              selected={m === selectedId}
                              largest={largest}
                            />
                          ) : null
                        })}
                      </ol>
                    </SortableContext>
                  )}
                </RankRow>
              )
            })}
          </ol>
        </SortableContext>
      </DndContext>
      <div className="mt-2 border-t border-line pt-2">
        <div className="grid grid-cols-[1.5rem_1.25rem_auto_1fr_auto] items-center gap-2 px-2 py-2 text-sm">
          <span />
          <span />
          <Swatch colour={book.look(null).colour} size={12} />
          <span className="min-w-0">
            <span className="block truncate text-ink-2">Uncategorized</span>
            <span className="block text-[12px] text-muted">Everything else, automatically</span>
          </span>
          <span className="tabular text-[13px] text-ink-2">{formatBytes(book.uncategorizedSize)}</span>
        </div>
      </div>
    </Card>
  )
}

function RankRow({
  category,
  rank,
  selected,
  largest,
  members = 0,
  children,
}: {
  category: Category
  rank: number
  selected: boolean
  largest: number
  /** How many categories are in it, if it is a group. */
  members?: number
  /** The group's own list, under its row. */
  children?: ReactNode
}) {
  const book = useCategories()
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id: category.id,
  })
  const look = book.look(category.id)
  const missing = category.assignments.filter((a) => a.missing).length

  return (
    <li
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={`rounded-xl ${isDragging ? 'relative z-10 bg-card shadow-xl ring-1 ring-line-strong' : ''}`}
    >
      <div
        className={`relative grid grid-cols-[1.5rem_1.25rem_auto_1fr_auto] items-center gap-2 rounded-xl px-2 py-2 transition-colors ${
          selected ? 'bg-raised' : isDragging ? 'bg-card' : 'hover:bg-raised/60'
        }`}
      >
        <button
          ref={setActivatorNodeRef}
          type="button"
          aria-label={`Move ${category.name}, rank ${rank}`}
          className="relative z-[1] flex h-8 w-6 cursor-grab touch-none items-center justify-center rounded-md text-muted hover:bg-card hover:text-ink active:cursor-grabbing"
          {...attributes}
          {...listeners}
        >
          <GripIcon size={16} />
        </button>
        <span className="tabular text-center text-[12px] text-muted">{rank}</span>
        <Swatch colour={look.colour} size={12} />
        <Link
          to={`/categories/${category.id}`}
          className="min-w-0 after:absolute after:inset-0 after:left-14 after:content-['']"
        >
          <span className="flex items-center gap-1.5">
            <span className={`truncate text-sm ${members > 0 ? 'font-semibold' : 'font-medium'}`}>{category.name}</span>
            {members > 0 && (
              <span className="shrink-0 rounded-md bg-raised px-1.5 text-[11px] font-medium text-muted">
                group · {members}
              </span>
            )}
            {missing > 0 && (
              <span title={`${missing} missing folder${missing > 1 ? 's' : ''}`} className="text-warn">
                <WarningIcon size={14} />
              </span>
            )}
          </span>
          <span className="mt-1 block h-1 overflow-hidden rounded-full bg-track" aria-hidden="true">
            <span
              className="block h-full rounded-full"
              style={{ width: `${Math.max(1.5, (category.size / largest) * 100)}%`, backgroundColor: look.colour }}
            />
          </span>
        </Link>
        <span className="tabular flex items-center gap-1 text-[13px] text-ink-2">
          {formatBytes(category.size)}
          <ChevronRightIcon size={14} className="text-muted lg:hidden" />
        </span>
      </div>
      {children}
    </li>
  )
}

// -- One category ------------------------------------------------------------------

function CategoryDetail({ category }: { category: Category }) {
  const book = useCategories()
  const navigate = useNavigate()
  const look = book.look(category.id)
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [picking, setPicking] = useState<{ repoint: string | null } | null>(null)
  const { data: history } = useApi(() => api.categoryHistory(category.id), [category.id])
  const missing = category.assignments.filter((a) => a.missing)
  const sharedTotal = category.sharedWith.reduce((s, x) => s + x.bytes, 0)
  const group = book.groupOf(category.id)
  const members = book.membersOf(category.id)
  const [adding, setAdding] = useState(false)

  return (
    <div className="space-y-4 sm:space-y-5">
      <Link to="/categories" className="inline-flex items-center gap-1 text-sm text-muted hover:text-ink lg:hidden">
        <ChevronLeftIcon size={16} />
        All categories
      </Link>

      <Card bodyClassName="p-4 sm:p-6">
        <div className="flex flex-wrap items-start gap-4">
          <span
            className="mt-1 h-10 w-10 shrink-0 rounded-xl"
            style={{ backgroundColor: look.colour }}
            aria-hidden="true"
          />
          <div className="min-w-0 flex-1">
            <h2 className="truncate text-2xl font-semibold tracking-tight">{category.name}</h2>
            <p className="text-[13px] text-muted">
              Rank {category.rank} of {book.categories.length} · {formatCount(category.assignments.length)} folder
              {category.assignments.length === 1 ? '' : 's'}
              {members.length > 0 && ` · a group of ${members.length} ${members.length === 1 ? 'category' : 'categories'}`}
              {group && (
                <>
                  {' · in '}
                  <Link to={`/categories/${group.id}`} className="inline-flex items-center gap-1 text-ink-2 hover:underline">
                    <Swatch colour={book.look(group.id).colour} size={8} />
                    {group.name}
                  </Link>
                </>
              )}
            </p>
          </div>
          <div className="flex gap-2">
            <Button onClick={() => setEditing(true)}>
              <PencilIcon size={15} />
              Edit
            </Button>
            <Button variant="danger" aria-label={`Delete ${category.name}`} onClick={() => setDeleting(true)}>
              <TrashIcon size={15} />
              <span className="max-sm:hidden">Delete</span>
            </Button>
          </div>
        </div>

        <div className="mt-5 grid gap-5 sm:grid-cols-[auto_1fr] sm:gap-10">
          <div>
            <p className="text-[13px] font-medium text-muted">Size</p>
            <p className="mt-1 text-4xl font-semibold tracking-tight">{formatBytes(category.size)}</p>
            <p className="mt-1 text-[13px] text-muted">
              {members.length > 0 ? 'With the categories in it; ' : ''}
              {sharedTotal > 0
                ? 'shared bytes included, each file once'
                : `nothing shared with ${members.length > 0 ? 'categories outside it' : 'other categories'}`}
            </p>
          </div>
          {category.sharedWith.length > 0 && (
            <div className="min-w-0">
              <p className="flex items-center gap-1.5 text-[13px] font-medium text-muted">
                <LinkIcon size={14} />
                Also in, through hardlinks
              </p>
              <ul className="mt-2 space-y-1.5">
                {category.sharedWith.map((s) => {
                  const other = book.look(s.categoryId)
                  return (
                    <li key={String(s.categoryId)} className="flex items-center gap-2 text-sm">
                      <span className="tabular font-medium">{formatBytes(s.bytes)}</span>
                      <span className="text-muted">also in</span>
                      <Swatch colour={other.colour} size={10} />
                      {s.categoryId === null ? (
                        <span>{other.name}</span>
                      ) : (
                        <Link to={`/categories/${s.categoryId}`} className="hover:underline">
                          {other.name}
                        </Link>
                      )}
                    </li>
                  )
                })}
              </ul>
              <p className="mt-2 text-[12px] text-muted">
                {category.sharedWith.some(
                  (s) =>
                    s.categoryId !== null &&
                    book.categories.findIndex((c) => c.id === s.categoryId) < category.rank - 1,
                )
                  ? 'In the overview, shared bytes go to whichever category ranks higher.'
                  : members.length > 0
                    ? `In the overview, these bytes stay in ${category.name}: it ranks higher.`
                    : `In the overview, ${category.name} owns these bytes: it ranks higher.`}
              </p>
            </div>
          )}
        </div>
      </Card>

      {missing.length > 0 && (
        <div role="status" className="flex items-start gap-3 rounded-2xl border border-warn/40 bg-warn/10 p-4 text-sm">
          <WarningIcon className="mt-0.5 shrink-0 text-warn" />
          <div>
            <p className="font-medium">
              {missing.length === 1 ? 'A folder is missing' : `${missing.length} folders are missing`}
            </p>
            <p className="mt-0.5 text-ink-2">
              It was renamed or deleted since you assigned it. Re-point the assignment to the folder's new name; its
              history is kept.
            </p>
          </div>
        </div>
      )}

      {members.length > 0 && (
        <Card
          title="In this group"
          subtitle={`Each category alone; ${category.name}'s own folders are the rest`}
          action={
            <Button size="sm" onClick={() => setAdding(true)}>
              <PlusIcon size={15} />
              New category in it
            </Button>
          }
          bodyClassName="pt-3 pb-2"
        >
          <ul className="divide-y divide-line border-t border-line">
            {members.map((m) => (
              <li key={m.id}>
                <Link
                  to={`/categories/${m.id}`}
                  className="flex items-center gap-3 px-4 py-2.5 hover:bg-raised/60 sm:px-5"
                >
                  <Swatch colour={book.look(m.id).colour} size={12} />
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">{m.name}</span>
                  <span className="hidden h-1.5 w-24 overflow-hidden rounded-full bg-track sm:block" aria-hidden="true">
                    <span
                      className="block h-full rounded-full"
                      style={{
                        width: `${Math.max(2, (m.size / Math.max(1, category.size)) * 100)}%`,
                        backgroundColor: book.look(m.id).colour,
                      }}
                    />
                  </span>
                  <span className="tabular w-20 text-right text-[13px] text-ink-2">{formatBytes(m.size)}</span>
                </Link>
              </li>
            ))}
          </ul>
        </Card>
      )}

      <Card
        title="Trend"
        subtitle={
          members.length > 0
            ? 'Its Size per day, the categories in it included, with today\'s folders applied to the past too'
            : "Its Size per day, with today's folders applied to the past too"
        }
      >
        {history ? (
          <HistoryChart history={history} colour={look.colour} name={category.name} height={240} />
        ) : (
          <Skeleton className="h-60" />
        )}
      </Card>

      <Card
        title="Folders"
        subtitle="Everything below each folder belongs here, unless a subfolder is assigned elsewhere"
        action={
          <Button size="sm" onClick={() => setPicking({ repoint: null })}>
            <PlusIcon size={15} />
            Add folder
          </Button>
        }
        bodyClassName="pt-3 pb-2"
      >
        {category.assignments.length === 0 ? (
          <p className="px-5 py-6 text-center text-sm text-muted">
            No folders yet. Add one, or use "Add to category…" in the Explorer.
          </p>
        ) : (
          <ul className="divide-y divide-line border-t border-line">
            {category.assignments.map((a) => (
              <li key={a.path} className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-2.5 sm:px-5">
                {a.missing ? (
                  <WarningIcon size={18} className="shrink-0 text-warn" />
                ) : (
                  <FolderIcon size={18} className="shrink-0 text-muted" />
                )}
                <div className="min-w-0 flex-1">
                  {a.missing ? (
                    <span className="block truncate font-mono text-[13px] text-ink-2 line-through decoration-muted">
                      {a.path}
                    </span>
                  ) : (
                    <Link
                      to={explorerHref(a.path, '30d')}
                      className="block truncate font-mono text-[13px] hover:underline"
                    >
                      {a.path}
                    </Link>
                  )}
                  {a.missing && <span className="text-[12px] font-medium text-warn">Missing</span>}
                </div>
                {a.missing ? (
                  <Button size="sm" onClick={() => setPicking({ repoint: a.path })}>
                    Re-point to…
                  </Button>
                ) : (
                  <span className="tabular text-[13px] text-ink-2">{a.size === null ? '' : formatBytes(a.size)}</span>
                )}
                <button
                  type="button"
                  aria-label={`Remove ${a.path} from ${category.name}`}
                  title="Remove from this category"
                  onClick={() => void change(api.unassign(a.path))}
                  className="rounded-md p-1.5 text-muted hover:bg-raised hover:text-ink"
                >
                  <CloseIcon size={16} />
                </button>
              </li>
            ))}
          </ul>
        )}
      </Card>

      {editing && (
        <CategoryEditor
          title={`Edit ${category.name}`}
          editing={category}
          initial={{ name: category.name, colour: category.colour, groupId: category.groupId }}
          confirm="Save"
          onSave={async (input) => {
            await change(api.updateCategory(category.id, input))
          }}
          onClose={() => setEditing(false)}
        />
      )}
      {adding && (
        <CategoryEditor
          title={`New category in ${category.name}`}
          initial={{
            name: '',
            colour: suggestColour(book.categories.map((c) => c.colour)),
            groupId: category.id,
          }}
          confirm="Create"
          onSave={async (input) => {
            const created = await change(api.createCategory(input))
            navigate(`/categories/${created.id}`)
          }}
          onClose={() => setAdding(false)}
        />
      )}
      {deleting && (
        <Modal
          title={`Delete ${category.name}?`}
          onClose={() => setDeleting(false)}
          footer={
            <>
              <Button variant="ghost" onClick={() => setDeleting(false)}>
                Cancel
              </Button>
              <Button
                variant="primary"
                className="!bg-bad !text-white"
                onClick={async () => {
                  await change(api.deleteCategory(category.id))
                  setDeleting(false)
                  navigate('/categories', { replace: true })
                }}
              >
                Delete category
              </Button>
            </>
          }
        >
          <p className="text-sm text-ink-2">
            Its {category.assignments.length === 1 ? 'folder goes' : `${category.assignments.length} folders go`} back
            to Uncategorized, or to a category assigned higher up the tree. No files are touched, and the folders'
            history is kept.
            {members.length > 0 &&
              ` The ${members.length === 1 ? 'category' : `${members.length} categories`} in it move${members.length === 1 ? 's' : ''} to the top level.`}
          </p>
        </Modal>
      )}
      {picking && (
        <FolderPicker
          title={picking.repoint ? 'Re-point to…' : `Add a folder to ${category.name}`}
          start={picking.repoint ? existingAncestor(picking.repoint) : ''}
          confirm={picking.repoint ? 'Re-point here' : 'Add this folder'}
          note={
            picking.repoint
              ? `Was ${picking.repoint}. Choose the folder it is called now.`
              : 'A folder already in another category moves here.'
          }
          onPick={async (path) => {
            await change(picking.repoint ? api.repoint(picking.repoint, path) : api.assign(path, category.id))
            setPicking(null)
          }}
          onClose={() => setPicking(null)}
        />
      )}
    </div>
  )
}

/** Where to start looking for a Missing folder's new name: its parent. */
function existingAncestor(path: string): string {
  return parentPath(path)
}

function CategoryEditor({
  title,
  editing,
  initial,
  confirm,
  onSave,
  onClose,
}: {
  title: string
  /** The category being edited; none for a new one. */
  editing?: Category
  initial: CategoryInput
  confirm: string
  onSave: (input: CategoryInput) => Promise<void>
  onClose: () => void
}) {
  const book = useCategories()
  const [value, setValue] = useState(initial)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const groupId = useId()
  // Groups are one level deep: a group can't go into another, and only a
  // top-level category can hold others.
  const isGroup = editing !== undefined && book.membersOf(editing.id).length > 0
  const groups = book.categories.filter((c) => c.groupId === null && c.id !== editing?.id)
  const save = async () => {
    setBusy(true)
    setError(null)
    try {
      await onSave({ name: value.name.trim(), colour: value.colour, groupId: value.groupId ?? null })
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal
      title={title}
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy || value.name.trim() === ''} onClick={() => void save()}>
            {confirm}
          </Button>
        </>
      }
    >
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (value.name.trim() !== '') void save()
        }}
      >
        <CategoryFields value={value} onChange={setValue} autoFocus />
        <div className="mt-4">
          <label htmlFor={groupId} className="mb-1.5 block text-[13px] font-medium text-ink-2">
            Group
          </label>
          <select
            id={groupId}
            value={value.groupId ?? ''}
            disabled={isGroup || groups.length === 0}
            onChange={(e) => setValue({ ...value, groupId: e.target.value === '' ? null : Number(e.target.value) })}
            className="h-10 w-full rounded-lg border border-line-strong bg-sunken px-3 text-sm text-ink focus:border-focus focus:outline-none disabled:opacity-60"
          >
            <option value="">None: a top-level category</option>
            {groups.map((g) => (
              <option key={g.id} value={g.id}>
                In {g.name}
              </option>
            ))}
          </select>
          <p className="mt-1.5 text-[12px] text-muted">
            {isGroup
              ? 'It holds other categories, so it stays at the top level.'
              : 'Put related categories together, such as Movies and TV in Media. The group shows their total.'}
          </p>
        </div>
        {error && (
          <p role="alert" className="mt-3 text-[13px] text-bad">
            {error}
          </p>
        )}
        <button type="submit" hidden />
      </form>
    </Modal>
  )
}
