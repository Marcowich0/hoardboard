import type { Theme } from '../lib/colours.ts'

/** Everything in a chart that isn't data: ink, grid, axes, tooltip. Recessive by design. */
export interface Chrome {
  surface: string
  ink: string
  ink2: string
  muted: string
  grid: string
  axis: string
  tooltip: {
    backgroundColor: string
    borderColor: string
    borderWidth: number
    padding: number[]
    textStyle: { color: string; fontSize: number; fontFamily: string }
    extraCssText: string
  }
}

const font = 'system-ui, -apple-system, "Segoe UI", sans-serif'

const tokens: Record<Theme, Omit<Chrome, 'tooltip'>> = {
  light: { surface: '#fcfcfb', ink: '#0b0b0b', ink2: '#52514e', muted: '#898781', grid: '#e9e8e2', axis: '#c3c2b7' },
  dark: { surface: '#191918', ink: '#f0efec', ink2: '#c3c2b7', muted: '#8f8d85', grid: '#2a2a28', axis: '#3a3a37' },
}

export function chrome(theme: Theme): Chrome {
  const t = tokens[theme]
  return {
    ...t,
    tooltip: {
      backgroundColor: theme === 'dark' ? '#232321' : '#ffffff',
      borderColor: theme === 'dark' ? 'rgba(255,255,255,0.10)' : 'rgba(11,11,11,0.10)',
      borderWidth: 1,
      padding: [8, 10],
      textStyle: { color: t.ink, fontSize: 12, fontFamily: font },
      extraCssText: `border-radius: 10px; box-shadow: 0 8px 24px rgba(0,0,0,${theme === 'dark' ? 0.45 : 0.12});`,
    },
  }
}

export const chartFont = font
