import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import type { Theme } from './lib/colours.ts'

export type { Theme }

const STORAGE_KEY = 'hoardboard.theme'

function readTheme(): Theme {
  try {
    if (localStorage.getItem(STORAGE_KEY) === 'light') return 'light'
  } catch {
    // Storage blocked: use the default dark theme.
  }
  return 'dark'
}

/**
 * The theme, applied as data-theme on <html> and remembered per browser.
 * public/theme-init.js applies the saved choice before the first paint; this
 * keeps it in sync after.
 */
export function useTheme() {
  const [theme, setTheme] = useState<Theme>(readTheme)

  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  const toggle = useCallback(() => {
    setTheme((current) => {
      const next: Theme = current === 'dark' ? 'light' : 'dark'
      try {
        localStorage.setItem(STORAGE_KEY, next)
      } catch {
        // Storage blocked: the choice lasts until the page is reloaded.
      }
      return next
    })
  }, [])

  return { theme, toggle }
}

/** The theme in effect, for what CSS can't switch by itself (chart colours). */
export const ThemeContext = createContext<Theme>('dark')

export function useCurrentTheme(): Theme {
  return useContext(ThemeContext)
}
