import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ComposeTasksPage.vue', import.meta.url), 'utf8')

// CHG-20261013-082 T-05：合成任务页（列表/筛选/详情/排队中取消）。
// 与本仓其余列表页同一量法：钉住**会静默失效**的那几处接线。
describe('compose tasks page', () => {
  // 三个下拉/输入里最容易腐烂的是「筛了但参数没上车」：load 必须把两个筛选项
  // 都拼进服务端参数，缺一个那条筛选就是摆设（MyMaterialsPage 有过这个 bug）。
  it('loads the list through the server-side filters it renders', () => {
    expect(source).toContain('client.listComposeTasks(params)')
    expect(source).toContain('if (filters.value.status) params.status = filters.value.status')
    expect(source).toContain('if (filters.value.compose_strategy_id) params.compose_strategy_id = filters.value.compose_strategy_id')
    expect(source).toContain('v-model="filters.status"')
    expect(source).toContain('v-model="filters.compose_strategy_id"')
  })

  it('renders the five task statuses and the pending stage as 等待源素材', () => {
    expect(source).toContain("pending: '排队中'")
    expect(source).toContain("running: '执行中'")
    expect(source).toContain("success: '已完成'")
    expect(source).toContain("failed: '失败'")
    expect(source).toContain("cancelled: '已取消'")
    expect(source).toContain('等待源素材')
  })

  // 取消只对排队中的行出现——非 pending 的行连按钮都没有，而不是点了再被拒。
  it('offers cancel only to pending tasks and asks before sending it', () => {
    expect(source).toContain("row.status === 'pending'")
    expect(source).toContain('client.cancelComposeTask(row.id)')
    expect(source).toContain('DialogPlugin.confirm')
  })

  it('opens detail from the server so the snapshot is the stored one', () => {
    expect(source).toContain('client.getComposeTask(row.id)')
    expect(source).toContain('config_snapshot')
  })

  // 操作列宽按行里最多会出现的按钮清单推导（排队中的行才有 取消），不再写死 130。
  it('derives the op column width from the buttons a pending row can show', () => {
    expect(source).toContain("opColumnWidth(['详情', '取消'])")
    expect(source).not.toContain("width: 130, fixed: 'right'")
  })

  // 高频条件在前：状态是排查任务时最先动的那个；策略名长，给它宽位。
  it('puts the status filter first and gives the strategy select the wide slot', () => {
    const bar = source.slice(source.indexOf('class="filter-bar"'), source.indexOf('table-scroll-wrap'))
    expect(bar).not.toContain('<t-space')
    expect(bar.indexOf('状态')).toBeGreaterThanOrEqual(0)
    expect(bar.indexOf('状态')).toBeLessThan(bar.indexOf('合成策略'))
    const fields = bar.match(/wt-filter-field[^"]*"/g) || []
    expect(fields.filter((entry) => entry.includes('wt-filter-field--wide'))).toHaveLength(1)
    expect(bar.indexOf('wt-filter-actions')).toBeGreaterThan(bar.lastIndexOf('wt-filter-field'))
  })
})
