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

  it('provides the same query and reset controls as the other resource pages', () => {
    expect(source).toContain('@click="applyFilters"')
    expect(source).toContain('@click="resetFilters"')
    expect(source).toContain('>查询</t-button>')
    expect(source).toContain('>重置</t-button>')
  })

  // 扫描抽屉的页脚只剩业务动作（2026-09-30 裁定：关闭走抽屉右上角的 ×）。没有可接受的
  // 变化时页脚整条不出现——`:footer` 为 false 时 TDesign 连页脚容器一起不渲染，否则会
  // 留下一道空条。
  it('keeps the scan drawer footer on the accept actions and drops it when there is nothing to accept', () => {
    expect(source).toContain(':footer="scanAcceptsChanges"')
    expect(source).toContain('const scanAcceptsChanges = computed(')
    const footer = source.slice(source.indexOf('<template #footer>', source.indexOf('scanDetailVisible')))
    expect(footer).toContain('接受本地变化')
    expect(footer).toContain('恢复Cloud配置并读回验证')
    expect(footer).not.toContain('>关闭</t-button>')
  })
})
