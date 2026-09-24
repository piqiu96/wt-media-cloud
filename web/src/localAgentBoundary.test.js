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

  /**
   * The same rule over the whole 本机设置 surface, module by module.
   *
   * The rewritten viewer reads two log trees and can open either one's folder;
   * the settings page writes a file under the data root. Every one of those is a
   * Rust command away, so there is no reason for a port or a `fetch` to appear
   * anywhere in this set — and if one appeared, it would be in a module the
   * page-scoped assertions above cannot see.
   *
   * The list is spelled out rather than globbed: a new file under these features
   * is then a deliberate addition to this list, which is the moment to check it.
   */
  it('keeps the whole local-settings surface off the network', () => {
    const modules = [
      './apps/desktop/features/local-settings/LocalSettingsPage.vue',
      './apps/desktop/features/local-settings/service.js',
      './apps/desktop/features/local-settings/local-settings-view.js',
      './apps/desktop/features/local-logs/local-logs-view.js',
    ]

    for (const path of modules) {
      const text = read(path)
      expect(text, path).not.toMatch(/127\.0\.0\.1/)
      expect(text, path).not.toMatch(/\blocalhost\b/)
      expect(text, path).not.toMatch(/\bfetch\(/)
      // No second door to the network, either: a page that reached the Agent
      // through XHR or a WebSocket would be the same CSP failure with a
      // different spelling.
      expect(text, path).not.toMatch(/XMLHttpRequest|new WebSocket/)
    }
  })
})
