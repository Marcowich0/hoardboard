import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import type { TreemapNode } from '../api/types.ts'
import { type FitNode, type FitRules, fitTreemap } from './treemap.ts'

const rules: FitRules = {
  minTile: 100,
  minDenseTile: 30,
  denseShare: 0.5,
  minOpen: 400,
  maxDepth: 3,
  frame: () => ({ border: 0, header: 0 }),
}

const folder = (name: string, size: number, children?: TreemapNode[]): TreemapNode => ({
  kind: 'folder',
  name,
  path: name,
  size,
  categoryId: null,
  children,
})
const file = (name: string, size: number): TreemapNode => ({ kind: 'file', name, path: name, size, categoryId: null })

/** nodes as "name(px)[children]", the rest as "+items(px)". */
const outline = (nodes: FitNode[]): string =>
  nodes
    .map((n) => {
      const s = n.kind === 'rest' ? `+${n.items}(${Math.round(n.px)})` : `${n.name}(${Math.round(n.px)})`
      return n.children ? `${s}[${outline(n.children)}]` : s
    })
    .join(' ')

describe('fitTreemap', () => {
  test('keeps tiles big enough to see and folds the others into one rest', () => {
    const got = fitTreemap([file('a', 600), file('b', 300), file('c', 50), file('d', 50)], 1000, rules)
    assert.equal(outline(got), 'a(600) b(300) +2(100)')
  })

  test('adds a rest the server sent to the one it makes', () => {
    const rest: TreemapNode = { kind: 'rest', name: '', path: '', size: 40, categoryId: null, items: 7 }
    const got = fitTreemap([file('a', 900), file('b', 60), rest], 1000, rules)
    assert.equal(outline(got), 'a(900) +8(100)')
  })

  test('opens a folder only with room inside for more than a rest', () => {
    const got = fitTreemap(
      [
        folder('big', 800, [file('x', 500), file('y', 300)]),
        folder('crumbs', 200, [file('p', 10), file('q', 10)]),
      ],
      1000,
      { ...rules, minTile: 120, minDenseTile: 120, minOpen: 150 },
    )
    assert.equal(outline(got), 'big(800)[x(500) y(300)] crumbs(200)')
  })

  test("takes the frame and header off a folder's inside", () => {
    // 900 px² is 30×30; less a 2px border and a 10px header, 26×18 = 468 px².
    const framed = { ...rules, minOpen: 450, frame: () => ({ border: 2, header: 10 }) }
    const got = fitTreemap([folder('f', 1, [file('x', 1), file('y', 1)])], 900, framed)
    assert.equal(outline(got), 'f(900)[x(234) y(234)]')
    assert.equal(outline(fitTreemap([folder('f', 1, [file('x', 1)])], 900, { ...framed, minOpen: 500 })), 'f(900)')
  })

  test('keeps smaller tiles where the rest would take most of a folder', () => {
    const many = Array.from({ length: 20 }, (_, i) => file(`m${i}`, 1))
    const got = fitTreemap([...many, file('tiny', 0.5)], 1000, rules)
    assert.equal(got.length, 21)
    assert.equal(outline(got.slice(-2)), 'm19(49) +1(24)')
  })

  test('stops at the deepest level', () => {
    const deep = folder('a', 1, [folder('b', 1, [folder('c', 1, [file('d', 1)])])])
    assert.equal(outline(fitTreemap([deep], 10000, rules)), 'a(10000)[b(10000)[c(10000)]]')
  })
})
