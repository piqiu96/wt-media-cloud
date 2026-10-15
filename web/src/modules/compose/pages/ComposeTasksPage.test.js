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
    expect(source).toContain('if (filters.value.trigger_type) params.trigger_type = filters.value.trigger_type')
    expect(source).toContain('if (filters.value.compose_strategy_id) params.compose_strategy_id = filters.value.compose_strategy_id')
    expect(source).toContain('v-model="filters.status"')
    expect(source).toContain('v-model="filters.trigger_type"')
    expect(source).toContain('v-model="filters.compose_strategy_id"')
    expect(source).toContain('createdRangeParams(filters.value.created_range)')
    expect(source).toContain('v-model="filters.created_range"')
  })

  it('renders the five formal task statuses and separates stage from status', () => {
    expect(source).toContain("pending: '排队中'")
    expect(source).toContain("running: '处理中'")
    expect(source).toContain("success: '成功'")
    expect(source).toContain("failed: '失败'")
    expect(source).toContain("cancelled: '已取消'")
    expect(source).toContain("title: '执行阶段'")
    expect(source).not.toContain('<t-progress')
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
    expect(source).toContain("opColumnWidth(['详情', '取消', '重试'])")
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

  // CHG-20261013-089 T-03：详情按「任务状态 → 快照 → 执行记录 → 成片」组织，逐次
  // 记录来自独立端点，不拿任务摘要冒充执行日志。
  it('loads attempts through the attempts endpoint and renders them per attempt', () => {
    expect(source).toContain('client.listComposeTaskAttempts(row.id)')
    expect(source).toContain('client.listComposeTaskAttempts(id)')
    expect(source).toContain('第 {{ attempt.attempt }} 次执行')
    expect(source).not.toContain('自动发起')
    expect(source).toContain("outcomeLabels[attempt.outcome]")
    expect(source).toContain('执行记录')
    expect(source).toContain('还没有执行记录')
  })

  // 重试分类钉在页面与服务端同一张表上：环境类与人工判断类有按钮（高级运营及以上），
  // 输入/模板类不给入口只引导新建任务——藏的是按钮，不是点了再被 409。
  it('gates the retry button by role and the error-code classes', () => {
    expect(source).toContain("new Set(['storage_error', 'task_timeout', 'execution_failed'])")
    expect(source).toContain("new Set(['input_missing', 'input_invalid', 'template_unavailable'])")
    expect(source).toContain("canRetry && retryableCodes.has(detail.error_code)")
    expect(source).toContain("newTaskCodes.has(detail.error_code)")
    expect(source).toContain('该失败不能重试，请修正后新建任务')
    expect(source).toContain('client.retryComposeTask(item.id)')
    expect(source).toContain("currentUser.value?.role")
  })

  // 成片关联只给 ID（服务端也不给对象地址），跳转去我的成片页。
  it('links a success task to its output id and the finished-media page', () => {
    expect(source).toContain("row.status === 'success' ? row.composite_output_id : null")
    expect(source).toContain('成片 #{{ outputID(detail) }}')
    expect(source).toContain("name: 'FinishedMedia', query: { output_id:")
  })

  it('shows uncertain execution as pending administrator review and records explicit verification', () => {
    expect(source).toContain("detail.error_code === 'execution_uncertain'")
    expect(source).toContain('旧 FFmpeg 已停止')
    expect(source).toContain('没有已提交成片')
    expect(source).toContain('reviewReady(')
    expect(source).toContain('client.resolveComposeUncertain(')
    expect(source).toContain('client.listComposeUncertainReviews(')
    expect(source).toContain('失败阶段：')
  })

  it('places the task and linked entities before status, then removes duplicate summary and output tab', () => {
    const columns = source.slice(source.indexOf('const columns'), source.indexOf('const tableScroll'))
    for (const [first, second] of [['task', 'material_id'], ['material_id', 'strategy'], ['strategy', 'output'], ['output', 'status'], ['status', 'stage'], ['stage', 'op']]) {
      expect(columns.indexOf(`colKey: '${first}'`)).toBeLessThan(columns.indexOf(`colKey: '${second}'`))
    }
    expect(source).not.toContain('compose-list-summary')
    expect(source).not.toContain('label="成片结果"')
    expect(source).toContain('label="配置快照"')
    expect(source).toContain('display_name?.trim()')
    expect(source).toContain('goMaterial(row)')
    expect(source).toContain('goStrategy(row)')
  })

  it('opens the existing entity detail surfaces by scoped API reads', () => {
    const strategies = readFileSync(new URL('./ComposeStrategyPage.vue', import.meta.url), 'utf8')
    const outputs = readFileSync(new URL('./MyOutputsPage.vue', import.meta.url), 'utf8')
    const materials = readFileSync(new URL('../../materials/pages/MaterialLibraryPage.vue', import.meta.url), 'utf8')
    expect(source).toContain("name: 'MaterialLibrary', query: { material_id:")
    expect(source).toContain("name: 'ComposeStrategy', query: { strategy_id:")
    expect(source).toContain("name: 'FinishedMedia', query: { output_id:")
    expect(materials).toContain('client.get(Number(raw))')
    expect(strategies).toContain('client.getStrategy(Number(id)).then(openDetail)')
    expect(outputs).toContain('openDetail({ id: Number(id) })')
  })
})
