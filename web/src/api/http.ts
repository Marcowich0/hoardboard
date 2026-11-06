// The real hoardboard server. Each endpoint here replaces its fake one in
// ./index.ts once the server has it.

import { ApiError } from './errors.ts'
import type { HealthReport } from './types.ts'

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const response = await fetch(url, {
    method,
    headers:
      body === undefined
        ? { Accept: 'application/json' }
        : { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await response.text()
  let data: unknown = undefined
  try {
    data = text === '' ? undefined : JSON.parse(text)
  } catch {
    // Not JSON (a proxy's error page, say): the status says enough.
  }
  if (!response.ok) {
    const message =
      typeof data === 'object' && data !== null && 'error' in data && typeof data.error === 'string'
        ? data.error
        : `${method} ${url}: ${response.status} ${response.statusText}`
    throw new ApiError(response.status, message)
  }
  return data as T
}

export const getJSON = <T>(url: string) => request<T>('GET', url)
export const sendJSON = <T>(method: 'POST' | 'PUT' | 'PATCH' | 'DELETE', url: string, body?: unknown) =>
  request<T>(method, url, body)

/** GET /api/health. It answers 503 with a report when unhealthy, which is still a report. */
export async function health(): Promise<HealthReport> {
  try {
    return await getJSON<HealthReport>('/api/health')
  } catch (err) {
    if (err instanceof ApiError) return { status: 'unhealthy', error: err.message }
    throw err
  }
}
