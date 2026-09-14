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

  it('shows all five routine window operations directly instead of hiding them in an overflow menu', () => {
    expect(source).not.toContain('>更多</t-button>')
    expect(source).toContain('@click="openProfile(row)"')
    expect(source).toContain('@click="closeProfile(row)"')
    expect(source).toContain('@click="openProxyBinding(row)"')
    expect(source).toContain('@click="openEdit(row)"')
    expect(source).toContain('@click="toggleBusinessStatus(row)"')
  })

  it('keeps the browser window operation column pinned during horizontal scrolling', () => {
    expect(source).toContain('{ colKey: "op", title: "操作", width: 320, fixed: "right" }')
  })

  it('keeps the filter field labels visible', () => {
    expect(source).toContain('>窗口 ID</span>')
    expect(source).toContain('>授权用户</span>')
    expect(source).toContain('>Cloud 状态</span>')
  })
})
