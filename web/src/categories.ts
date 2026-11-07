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
 * everywhere and follows a rename or recolour at once. It also knows the
 * groups: a category may be in one top-level category (Movies in Media).
 */
export interface CategoryBook {
  /** In Rank order, which is group order: a group, then the categories in it. Empty until loaded. */
  categories: Category[]
  uncategorizedSize: number
  loaded: boolean
  /** A category by id; null (or an id that is gone) is Uncategorized. */
  look(categoryId: number | null): Look
  unaccounted: Look
  /** The group a category is in, null at the top level (or for an id that is gone). */
  groupOf(categoryId: number | null): Category | null
  /** The categories in a group, in Rank order; empty for any other category. */
  membersOf(categoryId: number): Category[]
}

export function makeBook(data: Categories | undefined, theme: Theme): CategoryBook {
  const categories = data?.categories ?? []
  const byId = new Map(categories.map((c) => [c.id, c]))
  const members = new Map<number, Category[]>()
  for (const c of categories) {
    if (c.groupId !== null) members.set(c.groupId, [...(members.get(c.groupId) ?? []), c])
  }
  const uncategorized: Look = { name: 'Uncategorized', colour: uncategorizedColour[theme], hatch: false }
  return {
    categories,
    uncategorizedSize: data?.uncategorized.size ?? 0,
    loaded: data !== undefined,
    look(categoryId) {
      const category = categoryId === null ? undefined : byId.get(categoryId)
      return category
        ? { name: category.name, colour: displayColour(category.colour, theme), hatch: false }
        : uncategorized
    },
    unaccounted: { name: 'Unaccounted', colour: unaccountedColour[theme], hatch: true },
    groupOf(categoryId) {
      const groupId = categoryId === null ? null : (byId.get(categoryId)?.groupId ?? null)
      return groupId === null ? null : (byId.get(groupId) ?? null)
    },
    membersOf(categoryId) {
      return members.get(categoryId) ?? []
    },
  }
}

export const CategoryContext = createContext<CategoryBook>(makeBook(undefined, 'dark'))

export function useCategories(): CategoryBook {
  return useContext(CategoryContext)
}
