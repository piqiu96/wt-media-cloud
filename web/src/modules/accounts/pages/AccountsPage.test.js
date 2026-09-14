import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./AccountsPage.vue', import.meta.url), 'utf8')

describe('media account resource page', () => {
  it('uses shared resource layout and preserves account operations', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceCard')
    expect(source).toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('wt-resource-table')
    expect(source).toContain('resourceAccountStats')
    expect(source).toContain('@click="openCreate"')
    expect(source).toContain('@click="checkFromRow(row)"')
    expect(source).toContain('@click="toggleBizStatus(row)"')
  })

  it('shows the routine account operations directly without a separate cookie column', () => {
    expect(source).not.toContain('<t-dropdown')
    expect(source).not.toContain('{ colKey: "cookie", title: "Cookie"')
    expect(source).toContain('@click="openCookieDialog(row)"')
    expect(source).toContain('@click="openEdit(row)"')
  })

  it('uses the browser-window resource table structure', () => {
    expect(source).toContain('class="filter-bar"')
    expect(source).toContain('class="table-scroll-wrap"')
    expect(source).toContain(":scroll=\"{ x: 'max-content' }\"")
    expect(source).toContain('class="pagination-bar"')
  })

  it('keeps account filter labels and a six-action-width operation column', () => {
    expect(source).toContain('>综合搜索</span>')
    expect(source).toContain('>账号状态</span>')
    expect(source).toContain('{ colKey: "op", title: "操作", width: 400, fixed: "right" }')
  })
})
