import { useId } from 'react'
import type { CategoryInput } from '../api/index.ts'
import { presets } from '../lib/colours.ts'
import { useCurrentTheme } from '../theme.ts'
import { CheckIcon } from './icons.tsx'

/** A category's name and colour: the presets, or any colour. */
export function CategoryFields({
  value,
  onChange,
  autoFocus = false,
}: {
  value: CategoryInput
  onChange: (value: CategoryInput) => void
  autoFocus?: boolean
}) {
  const theme = useCurrentTheme()
  const nameId = useId()
  const custom = !presets.some((p) => p.light === value.colour.toLowerCase())
  return (
    <div className="space-y-4">
      <div>
        <label htmlFor={nameId} className="mb-1.5 block text-[13px] font-medium text-ink-2">
          Name
        </label>
        <input
          id={nameId}
          value={value.name}
          autoFocus={autoFocus}
          maxLength={40}
          placeholder="Movies"
          onChange={(e) => onChange({ ...value, name: e.target.value })}
          className="h-10 w-full rounded-lg border border-line-strong bg-sunken px-3 text-sm text-ink placeholder:text-muted focus:border-focus focus:outline-none"
        />
      </div>
      <fieldset>
        <legend className="mb-1.5 text-[13px] font-medium text-ink-2">Colour</legend>
        <div className="flex flex-wrap items-center gap-2">
          {presets.map((p) => {
            const selected = p.light === value.colour.toLowerCase()
            return (
              <button
                key={p.light}
                type="button"
                aria-label={p.name}
                aria-pressed={selected}
                title={p.name}
                onClick={() => onChange({ ...value, colour: p.light })}
                className={`flex h-8 w-8 items-center justify-center rounded-full ring-offset-2 ring-offset-card transition-shadow ${
                  selected ? 'ring-2 ring-ink' : 'hover:ring-2 hover:ring-line-strong'
                }`}
                style={{ backgroundColor: p[theme] }}
              >
                {selected && <CheckIcon size={16} className="text-white" />}
              </button>
            )
          })}
          <label
            title="Any colour"
            className={`relative flex h-8 cursor-pointer items-center gap-1.5 rounded-full border border-line-strong pr-3 pl-1 text-[13px] text-ink-2 ring-offset-2 ring-offset-card ${
              custom ? 'ring-2 ring-ink' : ''
            }`}
          >
            <span
              className="h-6 w-6 rounded-full"
              style={{
                background: custom
                  ? value.colour
                  : 'conic-gradient(#e34948, #eda100, #1baf7a, #2a78d6, #6250d6, #e87ba4, #e34948)',
              }}
            />
            Custom
            <input
              type="color"
              value={value.colour}
              onChange={(e) => onChange({ ...value, colour: e.target.value })}
              className="absolute inset-0 cursor-pointer opacity-0"
            />
          </label>
        </div>
      </fieldset>
    </div>
  )
}
