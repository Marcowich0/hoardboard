import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { api, type ScanStatus as Status } from '../api/index.ts'
import { invalidate } from '../api/useApi.ts'
import { formatAgo, formatDuration } from '../lib/format.ts'
import { ClockIcon, RefreshIcon, WarningIcon } from './icons.tsx'

/** Polls GET /api/scan: every second while a Scan runs, every 30 s otherwise. */
function useScan() {
  const [status, setStatus] = useState<Status | null>(null)
  const [now, setNow] = useState(() => new Date())
  const wasRunning = useRef(false)
  const running = status !== null && status.running !== null

  const apply = useCallback((next: Status) => {
    setStatus(next)
    setNow(new Date())
    // A Scan just finished: everything on screen is out of date.
    if (wasRunning.current && !next.running) invalidate()
    wasRunning.current = next.running !== null
  }, [])

  useEffect(() => {
    let alive = true
    const poll = () =>
      api.scanStatus().then(
        (next) => alive && apply(next),
        () => {
          // Keep the last status; the next poll tries again.
        },
      )
    void poll()
    const timer = setInterval(() => void poll(), running ? 1000 : 30_000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [apply, running])

  const scanNow = useCallback(async () => apply((await api.scanNow()).status), [apply])

  return { status, now, scanNow }
}

export default function ScanStatus() {
  const { status, now, scanNow } = useScan()
  const [pressing, setPressing] = useState(false)
  const running = status?.running ?? null

  const press = async () => {
    setPressing(true)
    try {
      await scanNow()
    } finally {
      setPressing(false)
    }
  }

  let line: ReactNode = <span className="text-muted">Loading scan status…</span>
  if (running) {
    line = (
      <span className="text-ink-2">
        Scanning…{' '}
        <span className="tabular text-muted">{formatDuration(now.getTime() - Date.parse(running.startedAt))}</span>
      </span>
    )
  } else if (status?.last) {
    line = (
      <span className="text-ink-2">
        Last scan {formatAgo(new Date(status.last.finishedAt), now)}
        <span className="text-muted"> · took {formatDuration(status.last.durationMs)}</span>
      </span>
    )
  } else if (status) {
    line = <span className="text-ink-2">No scan yet</span>
  }

  return (
    // display: contents, so on a phone the status text can wrap to its own
    // row under the header while the button stays in it.
    <div className="contents">
      <div
        className="order-last min-w-0 basis-full pb-0.5 text-[13px] leading-tight md:order-none md:ml-auto md:basis-auto md:pb-0 md:text-right"
        aria-live="polite"
      >
        <p className="flex items-center gap-1.5 truncate md:justify-end">
          <ClockIcon size={14} className="shrink-0 text-muted" />
          {line}
        </p>
        {status?.skipped && !running && (
          <p
            className="mt-0.5 flex items-center gap-1.5 truncate text-ink-2 md:justify-end"
            title={status.skipped.reason}
          >
            <WarningIcon size={14} className="shrink-0 text-warn" />
            <span className="truncate">
              Skipped {formatAgo(new Date(status.skipped.at), now)}: {status.skipped.reason}
            </span>
          </p>
        )}
      </div>
      <button
        type="button"
        onClick={() => void press()}
        disabled={pressing || running !== null}
        className="ml-auto inline-flex h-9 shrink-0 items-center gap-1.5 md:ml-0 rounded-lg border border-line-strong bg-card px-3 text-sm font-medium text-ink transition-colors hover:bg-raised disabled:cursor-default disabled:opacity-70"
      >
        <RefreshIcon size={16} className={running ? 'spin' : ''} />
        <span>{running ? 'Scanning' : 'Scan now'}</span>
      </button>
    </div>
  )
}
