import type { Period } from '../api/types.ts'

export const periods: readonly Period[] = ['7d', '30d', '1y']
export const defaultPeriod: Period = '30d'

/** The Explorer's URL for a folder and period; names are escaped, the default period left out. */
export function explorerHref(path: string, period: Period): string {
  const encoded = path === '' ? '' : `/${path.split('/').map(encodeURIComponent).join('/')}`
  return `/explorer${encoded}${period === defaultPeriod ? '' : `?period=${period}`}`
}

/** The period in a query string, or the default. */
export function periodFrom(query: URLSearchParams): Period {
  const value = query.get('period')
  return periods.find((p) => p === value) ?? defaultPeriod
}

/** Every folder from the root down to `path`, for a breadcrumb trail. */
export function breadcrumbs(path: string): { name: string; path: string }[] {
  const names = path === '' ? [] : path.split('/')
  return [
    { name: 'All shares', path: '' },
    ...names.map((name, i) => ({ name, path: names.slice(0, i + 1).join('/') })),
  ]
}

/** The folder above `path`; the root's is the root. */
export function parentPath(path: string): string {
  const slash = path.lastIndexOf('/')
  return slash < 0 ? '' : path.slice(0, slash)
}
