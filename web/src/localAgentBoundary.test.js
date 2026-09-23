import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function read(relativePath) {
  return readFileSync(new URL(relativePath, import.meta.url), 'utf8')
}

const source = read('./apps/desktop/features/local-agent/init.js')

describe('Desktop Local Agent boundary', () => {
  it('does not let Vue connect to the Local Agent loopback port', () => {
    expect(source).not.toMatch(/127\.0\.0\.1:8765/)
    expect(source).not.toMatch(/fetch\([^)]*api\/v1\/status/)
  })

  /**
   * AC-05: the Local Agent init path carries no hard-coded Cloud address.
   *
   * `cloudBaseUrl()` here used to answer `http://127.0.0.1:18080` from a literal
   * (and from `window.location.origin`, which is the same address in the only
   * case that ever matched). The address now arrives from
   * `get_public_config`, so the literal has no reason to come back — and this
   * is what says so permanently, rather than a grep recorded once in a change
   * record.
   *
   * Three other files still carry an 18080 literal (`AccountsPage.vue`,
   * `ProfilesPage.vue`, `shared/api/http.js`); they are registered as residual
   * and outside this change, which is why this rule is scoped to `init.js`.
   */
  it('carries no hard-coded Cloud address', () => {
    expect(source).not.toMatch(/18080/)
  })

  /**
   * The same rule, on the one page that was still breaking it.
   *
   * `LocalLogsPage.vue` fetched `http://127.0.0.1:8765/healthz` directly, which
   * the window's CSP blocks -- so "Agent 不可达" was the only thing it could ever
   * show, whether the Agent was up or not. It goes through the Local Agent
   * service now, and this is what keeps it there.
   *
   * Scoped to these two files on purpose: `AccountsPage.vue`,
   * `ProfilesPage.vue` and `shared/api/http.js` still carry the same shape and
   * are registered as residual (they are outside this change's scope), so a
   * whole-tree rule would fail on them today and have to be deleted to pass.
   */
  it('does not let the local-logs page reach the Agent port itself', () => {
    const page = read('./apps/desktop/features/local-logs/LocalLogsPage.vue')
    expect(page).not.toMatch(/127\.0\.0\.1/)
    expect(page).not.toMatch(/\bfetch\(/)
  })
})
