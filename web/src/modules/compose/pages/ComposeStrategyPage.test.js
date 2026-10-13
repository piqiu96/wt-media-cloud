import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ComposeStrategyPage.vue', import.meta.url), 'utf8')

// 合成策略页：操作列宽随角色可点按钮数自适应；筛选栏按 AdminUI 规范 §2.1
// 「高频优先、主条件更宽」排布。
describe('compose strategy page', () => {
  // 普通运营只有 详情；管理角色（canManage）多 编辑/复制 两个。列宽跟着这份清单走。
  it('derives the op column width from the buttons the current role can see', () => {
    expect(source).toContain("opColumnWidth(canManage.value ? ['详情', '编辑', '复制'] : ['详情'])")
    expect(source).not.toContain("width: 170, fixed: 'right'")
  })

  // 名称搜索是主条件：给 wide 位、排第一；动作组在所有字段之后。
  it('puts the name search first in the wide slot and keeps the actions last', () => {
    const bar = source.slice(source.indexOf('class="filter-bar"'), source.indexOf('table-scroll-wrap'))
    expect(bar).not.toContain('<t-space')
    expect(bar.indexOf('策略名称')).toBeLessThan(bar.indexOf('游戏'))
    expect(bar.indexOf('游戏')).toBeLessThan(bar.indexOf('合成模板'))
    expect(bar.indexOf('合成模板')).toBeLessThan(bar.indexOf('状态'))
    expect(bar).toContain('wt-filter-field--wide')
    const widePos = bar.indexOf('wt-filter-field--wide')
    const nameLabelPos = bar.indexOf('策略名称')
    expect(widePos).toBeLessThan(nameLabelPos)
    expect(bar.indexOf('wt-filter-actions')).toBeGreaterThan(bar.lastIndexOf('wt-filter-field'))
  })
})
