import { describe, expect, it } from 'vitest'
import { canCheckAccount } from './checkAvailability.js'

describe('account check availability', () => {
  const account = { enabled: true, browser_profile_id: '1', category_ids: [] }
  const context = { isDesktop: true, isOwner: true, account, windowAvailable: true }
  it('allows an unclassified account with an available window', () => {
    expect(canCheckAccount(context)).toBe(true)
  })
  it('requires ownership, desktop and an available bound window', () => {
    expect(canCheckAccount({ ...context, isOwner: false })).toBe(false)
    expect(canCheckAccount({ ...context, isDesktop: false })).toBe(false)
    expect(canCheckAccount({ ...context, windowAvailable: false })).toBe(false)
    expect(canCheckAccount({ ...context, account: { ...account, browser_profile_id: null } })).toBe(false)
  })
})
