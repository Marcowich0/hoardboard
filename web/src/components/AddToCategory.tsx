import { useState } from 'react'
import { api, ApiError, type CategoryInput } from '../api/index.ts'
import { change } from '../api/useApi.ts'
import { useCategories } from '../categories.ts'
import { suggestColour } from '../lib/colours.ts'
import { CategoryFields } from './CategoryForm.tsx'
import { CheckIcon, PlusIcon } from './icons.tsx'
import { Button, Modal, Swatch } from './ui.tsx'

/**
 * "Add to category…" for one Folder: pick a category (or make one). The
 * folder's own Assignment is replaced; subfolders assigned elsewhere keep theirs.
 */
export default function AddToCategory({
  path,
  current,
  onClose,
}: {
  path: string
  /** The category the folder is in now, and the Assignment that decides it. */
  current: { categoryId: number; assignedPath: string } | null
  onClose: () => void
}) {
  const book = useCategories()
  const own = current?.assignedPath === path ? current.categoryId : null
  const [choice, setChoice] = useState<number | 'new' | null>(own ?? (book.categories.length === 0 ? 'new' : null))
  const [draft, setDraft] = useState<CategoryInput>({
    name: '',
    colour: suggestColour(book.categories.map((c) => c.colour)),
  })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const save = async () => {
    setBusy(true)
    setError(null)
    try {
      if (choice === 'new') {
        // Pick the new category at once, so that if assigning fails, Add
        // retries the assign instead of making the category twice.
        const created = await change(api.createCategory(draft))
        setChoice(created.id)
        await change(api.assign(path, created.id))
      } else if (choice !== null) {
        await change(api.assign(path, choice))
      }
      onClose()
    } catch (err) {
      setError(err instanceof ApiError || err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const remove = async () => {
    setBusy(true)
    setError(null)
    try {
      await change(api.unassign(path))
      onClose()
    } catch (err) {
      setError(err instanceof ApiError || err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const inherited = current && current.assignedPath !== path ? book.look(current.categoryId) : null

  return (
    <Modal
      title="Add to category"
      onClose={onClose}
      footer={
        <>
          {own !== null && (
            <Button variant="ghost" className="mr-auto text-bad" disabled={busy} onClick={() => void remove()}>
              Remove from {book.look(own).name}
            </Button>
          )}
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            disabled={busy || choice === null || choice === own || (choice === 'new' && draft.name.trim() === '')}
            onClick={() => void save()}
          >
            {choice === 'new' ? 'Create and add' : 'Add'}
          </Button>
        </>
      }
    >
      <p className="mb-1 text-[13px] text-muted">Folder</p>
      <code className="mb-4 block truncate rounded-lg border border-line bg-sunken px-3 py-2 font-mono text-[13px]">
        {path}
      </code>
      {inherited && (
        <p className="mb-3 text-[13px] text-ink-2">
          Now in <strong className="font-semibold">{inherited.name}</strong> through{' '}
          <code className="font-mono text-[12px]">{current?.assignedPath}</code>. Adding it here wins over that: the
          most specific Assignment counts.
        </p>
      )}
      <ul role="radiogroup" aria-label="Category" className="space-y-1">
        {book.categories.map((c) => {
          const look = book.look(c.id)
          const selected = choice === c.id
          return (
            <li key={c.id}>
              <button
                type="button"
                role="radio"
                aria-checked={selected}
                onClick={() => setChoice(c.id)}
                className={`flex w-full items-center gap-3 rounded-lg border px-3 py-2.5 text-left text-sm transition-colors ${
                  selected ? 'border-ink bg-raised' : 'border-line hover:bg-raised'
                }`}
              >
                <Swatch colour={look.colour} size={12} />
                <span className="flex-1">{c.name}</span>
                {own === c.id && <span className="text-[12px] text-muted">Current</span>}
                {selected && <CheckIcon size={16} />}
              </button>
            </li>
          )
        })}
        <li>
          <button
            type="button"
            role="radio"
            aria-checked={choice === 'new'}
            onClick={() => setChoice('new')}
            className={`flex w-full items-center gap-3 rounded-lg border border-dashed px-3 py-2.5 text-left text-sm transition-colors ${
              choice === 'new' ? 'border-ink bg-raised' : 'border-line-strong text-ink-2 hover:bg-raised'
            }`}
          >
            <PlusIcon size={14} />
            New category…
          </button>
        </li>
      </ul>
      {choice === 'new' && (
        <div className="mt-4 rounded-xl border border-line p-4">
          <CategoryFields value={draft} onChange={setDraft} autoFocus />
        </div>
      )}
      {error && (
        <p role="alert" className="mt-3 text-[13px] text-bad">
          {error}
        </p>
      )}
    </Modal>
  )
}
