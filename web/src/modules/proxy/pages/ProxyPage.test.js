import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ProxyPage.vue', import.meta.url), 'utf8')

describe('proxy resource page', () => {
  it('uses shared resource layout and preserves proxy operations', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceCard')
    expect(source).toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('wt-resource-table')
    expect(source).toContain('proxyStatItems')
    expect(source).toContain('@click="openCreate"')
    expect(source).toContain('@click="deleteProxy(row)"')
    expect(source).toContain('@click="triggerCheck(row)"')
  })

  it('keeps secondary proxy actions in a compact overflow menu', () => {
    expect(source).toContain('<t-dropdown')
    expect(source).toContain('@click="openEdit(row)"')
    expect(source).toContain('@click="openQuota(row)"')
    expect(source).toContain('@click="deleteProxy(row)"')
  })

  it('uses the browser-window resource table structure', () => {
    expect(source).toContain('class="filter-bar"')
    expect(source).toContain('class="table-scroll-wrap"')
    expect(source).toContain(":scroll=\"{ x: 'max-content' }\"")
    expect(source).toContain('class="pagination-bar"')
  })

  it('keeps proxy filter labels and a six-action-width operation column', () => {
    expect(source).toContain('>代理状态</span>')
    expect(source).toContain('>供应商</span>')
    expect(source).toContain('{ colKey: "op", title: "操作", width: 420, fixed: "right" }')
  })
})
