import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ProfilesPage.vue', import.meta.url), 'utf8')

describe('browser window resource page', () => {
  it('composes the shared resource presentation primitives', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceCard')
    expect(source).toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('wt-resource-table')
    expect(source).toContain('profileStatItems')
  })

  it('keeps every existing window operation reachable through the action menu', () => {
    expect(source).toContain('<t-dropdown')
    expect(source).toContain('@click="openProfile(row)"')
    expect(source).toContain('@click="closeProfile(row)"')
    expect(source).toContain('@click="openProxyBinding(row)"')
    expect(source).toContain('@click="openEdit(row)"')
    expect(source).toContain('@click="toggleBusinessStatus(row)"')
  })
})
