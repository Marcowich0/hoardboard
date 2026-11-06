import { api } from '../api/index.ts'
import { useApi } from '../api/useApi.ts'

/** The server's health, from the real GET /api/health, and the revision it runs. */
export default function ServerFooter() {
  const { data } = useApi(() => api.health(), [])
  const ok = data?.status === 'ok'
  return (
    <footer className="mt-10 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-line pt-4 text-[12px] text-muted">
      <span className="flex items-center gap-1.5">
        <span
          aria-hidden="true"
          className={`h-2 w-2 rounded-full ${data === undefined ? 'bg-line-strong' : ok ? 'bg-good' : 'bg-bad'}`}
        />
        {data === undefined
          ? 'Checking the server…'
          : ok
            ? 'Server OK'
            : `Server unhealthy${data.error ? `: ${data.error}` : ''}`}
      </span>
      {data?.revision && <span className="font-mono">{data.revision.slice(0, 7)}</span>}
      <span className="ml-auto">Prototype: every number except the server status is made up</span>
    </footer>
  )
}
