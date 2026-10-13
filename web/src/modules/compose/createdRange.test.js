import { describe, expect, it } from 'vitest'
import { createdRangeParams } from './createdRange.js'

describe('compose page created range', () => {
  it('includes both selected local calendar days with an exclusive next-day end', () => {
    const params = createdRangeParams(['2026-10-12', '2026-10-13'])
    expect(new Date(params.created_from).getDate()).toBe(12)
    expect(new Date(params.created_before).getDate()).toBe(14)
  })

  it('omits a cleared range and refuses reversed or impossible days', () => {
    expect(createdRangeParams([])).toEqual({})
    expect(() => createdRangeParams(['2026-10-14', '2026-10-13'])).toThrow()
    expect(() => createdRangeParams(['2026-02-30', '2026-03-01'])).toThrow()
  })
})
