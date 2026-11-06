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
