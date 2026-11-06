import { type ReactNode, useMemo } from 'react'
import { api } from './api/index.ts'
import { useApi } from './api/useApi.ts'
import { CategoryContext, makeBook } from './categories.ts'
import { useCurrentTheme } from './theme.ts'

/** Loads the categories once for every page (and again after any change). */
export default function CategoryProvider({ children }: { children: ReactNode }) {
  const theme = useCurrentTheme()
  const { data } = useApi(() => api.categories(), [])
  const book = useMemo(() => makeBook(data, theme), [data, theme])
  return <CategoryContext value={book}>{children}</CategoryContext>
}
