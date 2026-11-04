import { describe, expect, it } from 'vitest'
import { draggedKey, reorderItems } from './reorder.js'
describe('compose draft reorder', () => {
  it('moves in either direction without resetting drafts, keys or object identity', () => {
    const source = [{ key: 'a', params: { draft: 'unfinished' } }, { key: 'b' }, { key: 'c' }]
    const moved = reorderItems(source, 'a', 'c')
    expect(moved.map((item) => item.key)).toEqual(['b', 'c', 'a'])
    expect(moved[2]).toBe(source[0])
    expect(moved[2].params.draft).toBe('unfinished')
    expect(source.map((item) => item.key)).toEqual(['a', 'b', 'c'])
    expect(reorderItems(moved, 'a', 'b')).toEqual(source)
  })
  it('accepts rule draft keys and ignores invalid or same-row drops', () => {
    const rules = [{ _key: 'a' }, { _key: 'b' }]
    expect(reorderItems(rules, 'a', 'b', '_key')).toEqual([rules[1], rules[0]])
    expect(reorderItems(rules, 'missing', 'b', '_key')).toBe(rules)
    expect(reorderItems(rules, 'a', 'a', '_key')).toBe(rules)
  })
  it('rejects foreign-list drag payloads and malformed drops', () => {
    const transfer = (value) => ({ getData: () => value })
    expect(draggedKey(transfer('{"scope":"settings","key":"a"}'), 'settings')).toBe('a')
    expect(draggedKey(transfer('{"scope":"rules","key":"a"}'), 'settings')).toBeNull()
    expect(draggedKey(transfer('broken'), 'settings')).toBeNull()
  })
})
