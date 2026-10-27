import { describe, expect, it } from 'vitest'
import { enabledGames } from './games.js'

describe('enabledGames', () => {
  it('keeps only games that can still be chosen', () => {
    const rows = [
      { id: 'sjz', status: 'enabled' },
      { id: 'other', status: 'disabled' },
      { id: 'legacy' },
    ]
    expect(enabledGames(rows).map((game) => game.id)).toEqual(['sjz'])
    expect(rows).toHaveLength(3)
  })

  it('returns an empty list instead of failing on a missing catalog', () => {
    expect(enabledGames([])).toEqual([])
    expect(enabledGames(null)).toEqual([])
    expect(enabledGames(undefined)).toEqual([])
    expect(enabledGames({ id: 'sjz', status: 'enabled' })).toEqual([])
  })
})
