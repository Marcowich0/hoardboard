import { type ButtonHTMLAttributes, type ReactNode, useEffect, useId, useRef } from 'react'
import { CloseIcon, WarningIcon } from './icons.tsx'
import { hatchCss } from '../charts/hatch.ts'

// The building blocks every page shares.

export function Card({
  title,
  subtitle,
  action,
  children,
  className = '',
  bodyClassName = 'p-4 sm:p-5',
}: {
  title?: ReactNode
  subtitle?: ReactNode
  action?: ReactNode
  children: ReactNode
  className?: string
  bodyClassName?: string
}) {
  return (
    <section className={`rounded-2xl border border-line bg-card shadow-[0_1px_0_rgba(0,0,0,0.04)] ${className}`}>
      {(title || action) && (
        <header className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2 px-4 pt-4 sm:px-5 sm:pt-5">
          <div className="min-w-0">
            {title && <h2 className="text-[15px] font-semibold tracking-tight text-ink">{title}</h2>}
            {subtitle && <p className="mt-0.5 text-[13px] text-muted">{subtitle}</p>}
          </div>
          {action}
        </header>
      )}
      <div className={bodyClassName}>{children}</div>
    </section>
  )
}

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'

const variants: Record<Variant, string> = {
  primary: 'bg-ink text-page hover:opacity-90',
  secondary: 'border border-line-strong bg-card text-ink hover:bg-raised',
  ghost: 'text-ink-2 hover:bg-raised hover:text-ink',
  danger: 'border border-line-strong bg-card text-bad hover:bg-raised',
}

export function Button({
  variant = 'secondary',
  size = 'md',
  className = '',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; size?: 'sm' | 'md' }) {
  return (
    <button
      type="button"
      className={`inline-flex shrink-0 items-center justify-center gap-1.5 rounded-lg font-medium transition-[background-color,opacity] disabled:cursor-not-allowed disabled:opacity-50 ${
        size === 'sm' ? 'h-8 px-2.5 text-[13px]' : 'h-9 px-3.5 text-sm'
      } ${variants[variant]} ${className}`}
      {...props}
    />
  )
}

/** A row of mutually exclusive choices (a period, a range). */
export function Segmented<T extends string>({
  value,
  options,
  onChange,
  label,
}: {
  value: T
  options: readonly { value: T; label: string }[]
  onChange: (value: T) => void
  label: string
}) {
  return (
    <div role="radiogroup" aria-label={label} className="inline-flex rounded-lg border border-line bg-sunken p-0.5">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={o.value === value}
          onClick={() => onChange(o.value)}
          className={`h-7 rounded-md px-2.5 text-[13px] font-medium transition-colors ${
            o.value === value ? 'bg-card text-ink shadow-sm' : 'text-muted hover:text-ink'
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

/** A category's colour mark. Identity rides on this, never on coloured text. */
export function Swatch({
  colour,
  hatch = false,
  size = 10,
  className = '',
}: {
  colour: string
  hatch?: boolean
  size?: number
  className?: string
}) {
  return (
    <span
      aria-hidden="true"
      className={`inline-block shrink-0 rounded-[3px] ${className}`}
      style={{
        width: size,
        height: size,
        backgroundColor: colour,
        backgroundImage: hatch ? hatchCss : undefined,
      }}
    />
  )
}

export function Skeleton({ className = '' }: { className?: string }) {
  return <div className={`animate-pulse rounded-lg bg-raised ${className}`} />
}

export function ErrorNote({ error, onRetry }: { error: Error; onRetry?: () => void }) {
  return (
    <div role="alert" className="flex items-start gap-3 rounded-xl border border-line bg-card p-4 text-sm">
      <WarningIcon className="mt-0.5 text-warn" />
      <div className="min-w-0 flex-1">
        <p className="font-medium">Couldn't load this</p>
        <p className="mt-0.5 break-words text-muted">{error.message}</p>
      </div>
      {onRetry && (
        <Button size="sm" onClick={onRetry}>
          Retry
        </Button>
      )}
    </div>
  )
}

/** A dialog over the page: a sheet from the bottom on a phone, centred above. */
export function Modal({
  title,
  onClose,
  children,
  footer,
  wide = false,
}: {
  title: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  wide?: boolean
}) {
  const dialog = useRef<HTMLDialogElement>(null)
  const titleId = useId()
  useEffect(() => {
    const el = dialog.current
    if (!el) return
    el.showModal()
    return () => el.close()
  }, [])
  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault()
        onClose()
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
      className={`m-0 mt-auto max-h-[92dvh] w-full max-w-none overflow-hidden rounded-t-2xl border border-line bg-card p-0 text-ink shadow-2xl backdrop:bg-black/55 backdrop:backdrop-blur-[2px] sm:m-auto sm:rounded-2xl ${
        wide ? 'sm:max-w-2xl' : 'sm:max-w-md'
      }`}
    >
      <div className="flex max-h-[92dvh] flex-col">
        <header className="flex items-center justify-between gap-4 border-b border-line px-5 py-3.5">
          <h2 id={titleId} className="text-base font-semibold tracking-tight">
            {title}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="-mr-1.5 rounded-md p-1.5 text-muted hover:bg-raised hover:text-ink"
          >
            <CloseIcon />
          </button>
        </header>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">{children}</div>
        {footer && <footer className="flex justify-end gap-2 border-t border-line px-5 py-3">{footer}</footer>}
      </div>
    </dialog>
  )
}

/** A label above a big number. */
export function Stat({
  label,
  value,
  detail,
  icon,
}: {
  label: string
  value: ReactNode
  detail?: ReactNode
  icon?: ReactNode
}) {
  return (
    <div className="min-w-0">
      <p className="flex items-center gap-1.5 text-[13px] font-medium text-muted">
        {icon}
        {label}
      </p>
      <p className="mt-1 text-2xl font-semibold tracking-tight sm:text-[28px]">{value}</p>
      {detail && <p className="mt-0.5 text-[13px] text-muted">{detail}</p>}
    </div>
  )
}
