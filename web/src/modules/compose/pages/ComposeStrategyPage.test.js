import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ComposeStrategyPage.vue', import.meta.url), 'utf8')

// 合成策略页：操作列宽随角色可点按钮数自适应；筛选栏按 AdminUI 规范 §2.1
// 「高频优先、主条件更宽」排布。
describe('compose strategy page', () => {
  it('keeps immediate execution separate from saving the next planned time', () => {
    expect(source).toContain('client.runStrategyNow(row.id, crypto.randomUUID())')
    expect(source).toContain('不改变')
  })
  // 普通运营只有详情；管理角色可详情、编辑、执行、复制。列宽跟着这份清单走。
  it('derives the op column width from the buttons the current role can see', () => {
    expect(source).toContain("opColumnWidth(canManage.value ? ['详情', '编辑', '执行', '复制'] : ['详情'])")
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

  // 运算符词表跟着字段走（发布时间一套、计数一套）。这两套只能走 :options 传，
  // 用 <t-option> 子节点时下拉会把它缓存进自己的 computed：编辑器整体换掉 form 对象后
  // 缓存追的还是旧对象，第一个条件的运算符列表会一直停在「发布时间」那套——收藏量
  // 那里只能选到「早于」这类服务端会拒的运算符（400 operator is not allowed）。
  it('feeds the operator options as a prop so they follow the field', () => {
    expect(source).toContain(':options="conditionOperators(condition.field)"')
    expect(source).not.toMatch(/<t-option v-for="operator in conditionOperators/)
  })

  // 批次结果归集（CHG-20261015-095）：统计由服务端 GROUP BY 给出，页面不二次
  // 推算；批次按运行记录的计划时间懒加载，部分失败要能逐素材看到。
  it('reads run batches lazily and renders server-side counts per material', () => {
    expect(source).toContain('client.listStrategyRunTasks(detailRow.value.id, run.schedule_slot)')
    expect(source).toContain('batchCountText(batchDetail[run.run_key].view.counts)')
    expect(source).toContain('素材 {{ task.material_id }}')
    expect(source).toContain('task.error_message')
  })
})
