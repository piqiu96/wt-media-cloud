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

  it('shows all five proxy operations directly when they fit the operation column', () => {
    expect(source).not.toContain('<t-dropdown')
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
    expect(source).toContain('{ colKey: "op", title: "操作", width: 340, fixed: "right" }')
  })

  // AdminUI 规范 §4.1：状态排在属性与关联实体之后。代理的容量/绑定是属性与关联，
  // 状态与检测结果紧随其后；把状态放回供应商后面会让「这台还能不能用」先于
  // 「这台绑了谁」出现，读表顺序与运营的判断顺序相反。
  it('puts the status columns after capacity and bindings', () => {
    const ordered = ['supplier', 'max_profile_count', 'bindings', 'business_status', 'last_check_result', 'expires_at']
    const offsets = ordered.map((key) => source.indexOf(`{ colKey: "${key}"`))
    expect(offsets.every((at) => at > -1), '六列都必须找得到').toBe(true)
    for (let i = 1; i < offsets.length; i += 1) {
      expect(offsets[i], `${ordered[i]} 必须排在 ${ordered[i - 1]} 之后`).toBeGreaterThan(offsets[i - 1])
    }
  })
})
