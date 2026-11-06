import assert from 'node:assert/strict'
import { test } from 'node:test'
import { escapeHtml } from './html.ts'

test('escapeHtml leaves no markup in a folder name', () => {
  assert.equal(
    escapeHtml(`<img src=x onerror="a('b')"> & co`),
    '&#60;img src=x onerror=&#34;a(&#39;b&#39;)&#34;&#62; &#38; co',
  )
})
