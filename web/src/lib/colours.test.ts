import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { displayColour, labelInk, mix, presets, shade, suggestColour, uncategorizedTints } from './colours.ts'

describe('displayColour', () => {
  test('shows a preset in the step chosen for each theme', () => {
    assert.equal(displayColour('#2a78d6', 'light'), '#2a78d6')
    assert.equal(displayColour('#2a78d6', 'dark'), '#3987e5')
    assert.equal(displayColour('#EB6834', 'dark'), '#d95926')
  })

  test('shows a colour the owner picked freely as it is', () => {
    assert.equal(displayColour('#123456', 'dark'), '#123456')
    assert.equal(displayColour('#123456', 'light'), '#123456')
  })
})

describe('suggestColour', () => {
  test('offers the first preset no category uses yet', () => {
    assert.equal(suggestColour([]), '#2a78d6')
    assert.equal(suggestColour(['#2A78D6', '#1baf7a']), '#eb6834')
  })

  test('starts over when every preset is taken', () => {
    const all = ['#2a78d6', '#eb6834', '#1baf7a', '#eda100', '#e87ba4', '#008300', '#6250d6', '#e34948']
    assert.equal(suggestColour(all), '#2a78d6')
  })
})

describe('labelInk', () => {
  test('is white on mid and dark fills, near-black on light ones', () => {
    assert.equal(labelInk('#2a78d6'), '#ffffff')
    assert.equal(labelInk('#e87ba4'), '#ffffff')
    assert.equal(labelInk('#9b9a94'), '#ffffff')
    assert.equal(labelInk('#eda100'), '#0b0b0b')
    assert.equal(labelInk('#c9c7bf'), '#0b0b0b')
  })
})

describe('mix and shade', () => {
  test('mix goes from one colour to the other', () => {
    assert.equal(mix('#000000', '#ffffff', 0), '#000000')
    assert.equal(mix('#000000', '#ffffff', 1), '#ffffff')
    assert.equal(mix('#000000', '#ffffff', 0.5), '#808080')
  })

  test('shade darkens towards black and lightens towards white', () => {
    assert.equal(shade('#2a78d6', 0), '#2a78d6')
    assert.equal(shade('#808080', 0.5), '#404040')
    assert.equal(shade('#808080', -0.5), '#c0c0c0')
  })
})

describe('uncategorizedTints', () => {
  test('are never a preset category colour', () => {
    const preset = new Set(presets.flatMap((p) => [p.light, p.dark]))
    for (const tints of Object.values(uncategorizedTints)) {
      for (const tint of tints) assert.ok(!preset.has(tint), tint)
    }
  })
})
