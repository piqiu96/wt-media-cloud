import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const page = readFileSync(new URL('./LoginPage.vue', import.meta.url), 'utf8')

describe('login feedback keeps the submit button reachable', () => {
  it('uses a growing card without the old feedback height and scroll patches', () => {
    expect(page).toContain('.login-card{position:absolute')
    expect(page).toContain('min-height:69.61%')
    expect(page).not.toContain('.login-card.has-feedback')
    expect(page).not.toContain('scrollIntoView')
  })

  it('shows field errors near inputs and moves session replacement to a dialog', () => {
    expect(page).toContain('<FormError id="login-username-error"')
    expect(page).toContain('<FormError id="login-password-error"')
    expect(page).toContain('<FeedbackDialog')
    expect(page).not.toContain('<t-alert')
  })
})
