import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { displayColour, labelInk, suggestColour } from './colours.ts'

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
