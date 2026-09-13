import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./AppLayout.vue', import.meta.url), 'utf8')

describe('application navigation', () => {
  it('hides account opening while retaining the resource navigation entries', () => {
    expect(source).not.toContain("title: '账号开户'")
    expect(source).toContain("title: '运营资源'")
    expect(source).toContain("path: '/browser-windows'")
    expect(source).toContain("path: '/proxies'")
    expect(source).toContain("path: '/accounts'")
  })

  it('renders functional menu groups as collapsible submenus', () => {
    expect(source).toContain('<t-submenu')
    expect(source).toContain('v-model:expanded="expandedGroups"')
  })
})
