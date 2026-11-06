import { createContext, useContext } from 'react'
import type { Categories, Category } from './api/index.ts'
import { displayColour, type Theme, unaccountedColour, uncategorizedColour } from './lib/colours.ts'

/** How a category, Uncategorized or Unaccounted is drawn. */
export interface Look {
  name: string
  /** The colour for this theme. */
  colour: string
  /** Drawn with a hatch on top (Unaccounted), so it differs without colour. */
  hatch: boolean
}

/**
 * The single source of category names and colours: the donut, the trends,
 * the Explorer and every legend ask it, so each category looks the same
 * everywhere and follows a rename or recolour at once.
 */
export interface CategoryBook {
  /** In Rank order. Empty until loaded. */
  categories: Category[]
  uncategorizedSize: number
  loaded: boolean
  /** A category by id; null (or an id that is gone) is Uncategorized. */
  look(categoryId: number | null): Look
  unaccounted: Look
}

export function makeBook(data: Categories | undefined, theme: Theme): CategoryBook {
  const byId = new Map((data?.categories ?? []).map((c) => [c.id, c]))
  const uncategorized: Look = { name: 'Uncategorized', colour: uncategorizedColour[theme], hatch: false }
  return {
    categories: data?.categories ?? [],
    uncategorizedSize: data?.uncategorized.size ?? 0,
    loaded: data !== undefined,
    look(categoryId) {
      const category = categoryId === null ? undefined : byId.get(categoryId)
      return category
        ? { name: category.name, colour: displayColour(category.colour, theme), hatch: false }
        : uncategorized
    },
    unaccounted: { name: 'Unaccounted', colour: unaccountedColour[theme], hatch: true },
  }
}

export const CategoryContext = createContext<CategoryBook>(makeBook(undefined, 'dark'))

export function useCategories(): CategoryBook {
  return useContext(CategoryContext)
}
