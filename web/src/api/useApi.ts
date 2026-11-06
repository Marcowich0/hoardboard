import { useCallback, useEffect, useRef, useState } from 'react'

// Every mounted useApi reloads after a change (a category saved, a Scan
// finished), so the pages never show stale numbers next to fresh ones.
const listeners = new Set<() => void>()

/** Tells every mounted useApi to load again. */
export function invalidate(): void {
  for (const listener of listeners) listener()
}

/** Runs a change, then reloads everything that shows data. */
export async function change<T>(work: Promise<T>): Promise<T> {
  const result = await work
  invalidate()
  return result
}

export interface Loaded<T> {
  /** The latest answer; it stays while a newer request loads. */
  data: T | undefined
  error: Error | undefined
  /** True until the answer for the current inputs arrives. */
  loading: boolean
  reload: () => void
}

/**
 * Loads data through `api`, again whenever a dependency changes or anything
 * calls invalidate(). Dependencies are plain values (strings, numbers). An
 * answer that arrives after a newer request started is dropped.
 */
export function useApi<T>(load: () => Promise<T>, deps: readonly (string | number | boolean | null)[]): Loaded<T> {
  const [version, setVersion] = useState(0)
  const key = `${JSON.stringify(deps)}#${version}`
  const [state, setState] = useState<{ key?: string; data?: T; error?: Error }>({})
  const reload = useCallback(() => setVersion((v) => v + 1), [])
  const latestLoad = useRef(load)

  useEffect(() => {
    latestLoad.current = load
  })

  useEffect(() => {
    listeners.add(reload)
    return () => {
      listeners.delete(reload)
    }
  }, [reload])

  useEffect(() => {
    let current = true
    latestLoad.current().then(
      (data) => {
        if (current) setState({ key, data })
      },
      (err: unknown) => {
        if (current) {
          setState((s) => ({ key, data: s.data, error: err instanceof Error ? err : new Error(String(err)) }))
        }
      },
    )
    return () => {
      current = false
    }
  }, [key])

  return { data: state.data, error: state.error, loading: state.key !== key, reload }
}
