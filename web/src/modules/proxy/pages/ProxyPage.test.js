import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ProxyPage.vue', import.meta.url), 'utf8')

describe('proxy resource page', () => {
  it('uses shared resource layout and preserves proxy operations', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceCard')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('OperationProgressDialog')
    expect(source).toContain('wt-resource-table')
    expect(source).toContain('@click="openCreate"')
    expect(source).toContain('@click="deleteProxy(row)"')
    expect(source).toContain('@click="checkOne(row)"')
  })

  it('shows every permitted proxy operation directly with a measured operation column', () => {
    expect(source).not.toContain('<t-dropdown')
    expect(source).toContain('const proxyActionLabels = computed(() => isGovernor.value')
    expect(source).toContain('? ["详情", "编辑", "检测", "设置配额", "删除"]')
    expect(source).toContain(': ["详情", "编辑", "检测"]')
    expect(source).toContain('width: opColumnWidth(proxyActionLabels.value, { variant: "text", loading: true, primary: true }), fixed: "right"')
    expect(source).toContain('@click="openEdit(row)"')
    expect(source).toContain('@click="openQuota(row)"')
  })

  it('uses the browser-window resource table structure', () => {
    expect(source).toContain('class="filter-bar"')
    expect(source).toContain('class="table-scroll-wrap"')
    expect(source).toContain(':scroll="tableScroll"')
    expect(source).toContain('class="pagination-bar"')
    expect(source).toContain('<t-pagination')
  })

  it('keeps proxy filter labels visible', () => {
    expect(source).toContain('>资源启停</span>')
    expect(source).toContain('>检测结果</span>')
    expect(source).toContain('>到期</span>')
    expect(source).toContain('>供应商</span>')
    expect(source).toContain('>搜索</span>')
  })

  // AdminUI 规范 §4.1：属性与关联在前，状态与检测结果紧随其后。把状态放回供应商
  // 后面会让「这台还能不能用」先于「这台绑了谁」出现，读表顺序与运营的判断顺序相反。
  it('puts the status columns after capacity and bindings', () => {
    const ordered = ['region_supplier', 'source_type', 'usage', 'business_status', 'last_check_result', 'expires_at']
    const offsets = ordered.map((key) => source.indexOf(`{ colKey: "${key}"`))
    expect(offsets.every((at) => at > -1), '六列都必须找得到').toBe(true)
    for (let i = 1; i < offsets.length; i += 1) {
      expect(offsets[i], `${ordered[i]} 必须排在 ${ordered[i - 1]} 之后`).toBeGreaterThan(offsets[i - 1])
    }
  })

  it('keeps batch health checks in the selection toolbar', () => {
    expect(source).toContain('>批量检测（{{ selectedRowKeys.length }}）</t-button>')
    expect(source).toContain('async function runBatchCheck(')
  })
})
