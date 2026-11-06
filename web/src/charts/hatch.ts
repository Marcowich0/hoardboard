// The hatch that marks Unaccounted (a Look with hatch: true) wherever it is
// drawn, so it differs from Uncategorized without colour.

/** The hatch as an ECharts decal, for the donut and the trend. */
export const hatchDecal = {
  symbol: 'rect',
  dashArrayX: [1, 0],
  dashArrayY: [2, 4],
  rotation: -Math.PI / 4,
  color: 'rgba(0,0,0,0.22)',
}

/** The hatch as a CSS background-image, for swatches and bars. */
export const hatchCss = 'repeating-linear-gradient(135deg, rgba(0,0,0,0.28) 0 1.5px, transparent 1.5px 4px)'
