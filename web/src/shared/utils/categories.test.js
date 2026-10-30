import { describe, expect, it } from 'vitest'
import { CATEGORY_TYPE_OPTIONS, categoryTypeLabel, enabledCategories } from './categories.js'

describe('categories', () => {
  it('keeps only enabled categories', () => {
    const rows = [
      { id: 1, enabled: true },
      { id: 2, enabled: false },
      { id: 3 },
    ]
    expect(enabledCategories(rows).map((category) => category.id)).toEqual([1])
    expect(rows).toHaveLength(3)
  })

  it('returns an empty list for a missing or malformed catalog', () => {
    expect(enabledCategories([])).toEqual([])
    expect(enabledCategories(null)).toEqual([])
    expect(enabledCategories(undefined)).toEqual([])
    expect(enabledCategories({ id: 1, enabled: true })).toEqual([])
  })

  it('maps the code-owned category types to user-facing labels', () => {
    expect(CATEGORY_TYPE_OPTIONS).toEqual([
      { value: 'game', label: '游戏' },
      { value: 'ai', label: 'AI' },
      { value: 'anime', label: '二次元' },
    ])
    expect(categoryTypeLabel('game')).toBe('游戏')
    expect(categoryTypeLabel('unknown')).toBe('—')
  })
})
