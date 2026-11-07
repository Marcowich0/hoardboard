// The one place category colours come from. A category stores one colour,
// "#rrggbb"; every chart, legend and the Explorer shows it through
// displayColour, so a category looks the same everywhere.
//
// The presets are a categorical palette validated for colour-blind
// separation in this order (adjacent pairs), with a step per theme: a preset
// is stored as its light step and drawn in the dark step on the dark theme.
// A colour the owner picks freely is drawn as it is in both.

export type Theme = 'dark' | 'light'

export interface Preset {
  name: string
  light: string
  dark: string
}

export const presets: readonly Preset[] = [
  { name: 'Blue', light: '#2a78d6', dark: '#3987e5' },
  { name: 'Orange', light: '#eb6834', dark: '#d95926' },
  { name: 'Aqua', light: '#1baf7a', dark: '#199e70' },
  { name: 'Yellow', light: '#eda100', dark: '#c98500' },
  { name: 'Magenta', light: '#e87ba4', dark: '#d55181' },
  { name: 'Green', light: '#008300', dark: '#008300' },
  { name: 'Violet', light: '#6250d6', dark: '#9085e9' },
  { name: 'Red', light: '#e34948', dark: '#e66767' },
]

/** Uncategorized: a neutral grey, so it never reads as one of the owner's categories. */
export const uncategorizedColour: Record<Theme, string> = { light: '#9b9a94', dark: '#7d7c76' }

/**
 * Muted tints for what no category claims, where telling folders apart helps
 * (the Explorer's treemap): low-chroma slate, sage, sand, mauve, teal and
 * clay, so neighbouring uncategorized folders differ while staying well
 * clear of the owner's saturated category colours. They mean "uncategorized"
 * together; none of them is ever a category's.
 */
export const uncategorizedTints: Record<Theme, readonly string[]> = {
  dark: ['#5d6674', '#5f6b5c', '#766a5a', '#6c6070', '#596b6b', '#735f58'],
  light: ['#a9b2bf', '#adb9a7', '#c3b6a2', '#baacbe', '#a6bbbb', '#c4aba3'],
}

/** Unaccounted: a quieter grey, always drawn with a hatch so it differs from Uncategorized without colour. */
export const unaccountedColour: Record<Theme, string> = { light: '#c9c7bf', dark: '#4a4945' }

/** The colour to draw a category's stored colour in, on this theme. */
export function displayColour(colour: string, theme: Theme): string {
  const preset = presets.find((p) => p.light === colour.toLowerCase())
  return preset ? preset[theme] : colour
}

/** A colour for a new category: the first preset nobody uses, or the first one. */
export function suggestColour(used: string[]): string {
  const taken = new Set(used.map((c) => c.toLowerCase()))
  return (presets.find((p) => !taken.has(p.light)) ?? presets[0]).light
}

/** The ink for a label set inside a fill: white, or near-black on light fills (yellow, light grey). */
export function labelInk(fill: string): string {
  const channel = (i: number) => {
    const v = parseInt(fill.slice(i, i + 2), 16) / 255
    return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
  }
  const luminance = 0.2126 * channel(1) + 0.7152 * channel(3) + 0.0722 * channel(5)
  return luminance > 0.4 ? '#0b0b0b' : '#ffffff'
}

/** a and b mixed: t 0 is a, 1 is b. Both "#rrggbb". */
export function mix(a: string, b: string, t: number): string {
  const ch = (hex: string, i: number) => parseInt(hex.slice(i, i + 2), 16)
  const out = [1, 3, 5].map((i) => Math.round(ch(a, i) + (ch(b, i) - ch(a, i)) * t))
  return `#${out.map((v) => v.toString(16).padStart(2, '0')).join('')}`
}

/** A colour a little darker (amount > 0) or lighter (amount < 0): mixed with black or white. */
export function shade(colour: string, amount: number): string {
  return amount >= 0 ? mix(colour, '#000000', amount) : mix(colour, '#ffffff', -amount)
}
