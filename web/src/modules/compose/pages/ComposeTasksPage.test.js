import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ComposeTasksPage.vue', import.meta.url), 'utf8')
const createDialog = readFileSync(new URL('../components/ComposeTaskCreateDialog.vue', import.meta.url), 'utf8')

// CHG-20261013-082 T-05 / M4-M5 合成任务页定点整改：列表 9 列、详情 3 Tab 4 分区、
// 关联实体只读抽屉、活动任务轮询。
// 与本仓其余列表页同一量法：钉住**会静默失效**的那几处接线。
describe('compose tasks page', () => {
  it('opens the existing task detail for a run-history task link', () => {
    expect(source).toContain("const taskID = String(route.query.task_id || '')")
    expect(source).toContain('openDetail({ id: Number(taskID) })')
  })
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

  it('uses the shared trigger labels for the filter, list and detail', () => {
    expect(source).toContain("import { TRIGGER_TYPE_OPTIONS, triggerLabel } from '../labels.js'")
    expect(source).toContain(':options="TRIGGER_TYPE_OPTIONS"')
    expect(source).toContain('triggerLabel(row.trigger_type)')
    expect(source).toContain('triggerLabel(detail.trigger_type)')
    expect(source).not.toContain('triggerText')
  })

  // 负责人筛选（CHG-104 T-08）：团队成员本身就是候选项，未选择即本团队全部；
  // 不再有「我的／团队全部」伪选项，页面也不再发送 operator_scope。
  it('lists team members as the only operator options and narrows by operator_id', () => {
    expect(source).toContain('v-model="filters.operator"')
    expect(source).toContain('v-if="canFilterOperators"')
    expect(source).toContain('operatorOptions.value.map((option) => ({ value: `op:${option.id}`, label: option.display_name }))')
    expect(source).not.toContain("value: 'mine'")
    expect(source).not.toContain("value: 'team'")
    expect(source).not.toContain("filters.value.operator = 'mine'")
    expect(source).not.toContain('params.operator_scope')
    expect(source).toContain('if ((isAdmin.value || isSenior.value) && namedOperator) params.operator_id = namedOperator')
    expect(source).toContain('const namedOperator = operatorIdFromOption(filters.value.operator)')
    // 清空下拉给的是 undefined，不是空串：解析和“选项是否还在”都必须走守卫。
    expect(source).toContain('missingOperatorOption(filters.value.operator, operatorFilterOptions.value)')
    expect(source).not.toContain('filters.value.operator.startsWith')
    expect(source).toContain('params.team_id = Number(filters.value.team_id)')
    expect(source).not.toContain(':clearable="!isSenior"')
    expect(source).toContain('client.listComposeOperators({ team_id: isAdmin.value ? Number(filters.value.team_id) : undefined })')
    expect(source).toContain("row.operator_name || '未记录'")
    expect(source).toContain('<span class="compose-detail__label">负责人</span>')
  })

  // 执行阶段是过程信息，只在详情里；列表放决策信息，进度百分比一律不进列表。
  it('renders the five formal task statuses and keeps the stage out of the list', () => {
    expect(source).toContain("pending: '排队中'")
    expect(source).toContain("running: '处理中'")
    expect(source).toContain("success: '成功'")
    expect(source).toContain("failed: '失败'")
    expect(source).toContain("cancelled: '已取消'")
    expect(source).not.toContain("title: '执行阶段'")
    expect(source).not.toContain('<t-progress')
    expect(source).not.toContain('row.progress')
  })

  // 等待素材时底层仍是「运行中」，但用户看到的事实是「还在等文件」。列表与详情两处
  // 徽标都必须走同一个 statusText：只改一处，另一处就退回「处理中」，用户会以为
  // 合成正在跑，而实际上云端连源文件都还没拿到。
  it('labels a waiting task as waiting for its material, not as processing', () => {
    expect(source).toContain("waiting_input: '等待素材准备'")
    expect(source).toContain('function statusText(row)')
    expect(source).toContain("if (row.status === 'running' && row.stage === 'waiting_input') return stageLabels.waiting_input")
    expect(source).toContain(':label="statusText(row)"')
    expect(source).toContain(':label="statusText(detail)"')
  })

  // 列序固定：识别 → 来源（含负责人）→ 结果 → 状态 → 时间（创建、完成）→ 操作。
  it('declares the eleven columns in the fixed order', () => {
    const columns = source.slice(source.indexOf('const columns'), source.indexOf('const tableScroll'))
    const order = ['id', 'name', 'trigger_type', 'material_id', 'strategy', 'operator', 'output', 'status', 'created_at', 'finished_at', 'op']
    // 分母可见：列数对不上说明列结构被改过，立刻失败而不是让后面的顺序断言空转。
    expect((columns.match(/colKey:/g) || [])).toHaveLength(order.length)
    for (let index = 1; index < order.length; index += 1) {
      const previous = columns.indexOf(`colKey: '${order[index - 1]}'`)
      expect(previous, order[index - 1]).toBeGreaterThanOrEqual(0)
      expect(previous).toBeLessThan(columns.indexOf(`colKey: '${order[index]}'`))
    }
  })

  it('shows the task finish time in the list and names the detail field the same', () => {
    expect(source).toContain("{ colKey: 'finished_at', title: '完成时间', width: 150 }")
    expect(source).toContain('<template #finished_at="{ row }">{{ formatDateTime(row.finished_at) }}</template>')
    expect(source).toContain('<span class="compose-detail__label">完成时间</span><span>{{ formatDateTime(detail.finished_at) }}</span>')
    expect(source).not.toContain('结束时间')
  })

  // 任务 ID 一律裸数字：`#252` 这类内码前缀在本页与创建抽屉里都不出现。
  it('never prefixes an id with a hash', () => {
    for (const [file, text] of [['ComposeTasksPage.vue', source], ['ComposeTaskCreateDialog.vue', createDialog]]) {
      expect(text, file).not.toMatch(/#(?:\{|\$|\d)/)
    }
  })

  // 任务名称是创建时形成的事实，只在历史行为空时才退回「合成任务 252」。
  it('keeps the stored task name and falls back to a stable one', () => {
    expect(source).toContain('display_name?.trim()')
    expect(source).toContain('`合成任务 ${row.id}`')
  })

  /**
   * CHG-20261015-098（用户裁定）：创建合成任务的入口**只**在这里 —— 我的素材里的工具条
   * 批量与行内「加入合成」整条撤掉，素材改由创建抽屉自己去「我的素材」取。
   *
   * 三处接线都会静默失效：页头那颗按钮没挂 `createVisible`、页面尾部没挂抽屉实例、
   * `@created` 没接上 `load()` —— 任何一处断掉，用户看到的都是「点了没反应」或者
   * 「创建成功但列表里没有」。
   */
  it('carries the only create entry: a header button opening the shared dialog', () => {
    expect(source).toContain('新建合成任务')
    expect(source).toContain('@click="createVisible = true"')
    expect(source).toContain('const createVisible = ref(false)')
    expect(source).toContain('<ComposeTaskCreateDialog v-model:visible="createVisible" @created="onCreated" />')
    expect(source).toContain("import ComposeTaskCreateDialog from '../components/ComposeTaskCreateDialog.vue'")
    // 素材范围不再由宿主注入：入口只有一个，抽屉自己取候选。
    expect(source).not.toContain(':materials="')
  })

  // 创建之后必须重读列表：新任务是 pending，列表的轮询正是靠这一行读到的数据挂上去的。
  // 成功与部分失败分开说，失败的原因逐条带上 —— 只说「创建失败」等于让运营再试一次同样的错。
  it('re-reads the list after creating and splits the created from the failed', () => {
    const handler = source.slice(source.indexOf('async function onCreated'), source.indexOf('\n}', source.indexOf('async function onCreated')))
    expect(handler).toContain('await load()')
    expect(handler).toContain('result.created')
    expect(handler).toContain('result.failed')
    expect(handler).toContain('feedback.toast.success(`已创建 ${created.length} 个合成任务`)')
    expect(handler).toMatch(/feedback\.toast\.error\(/)
    // 成片是异步作业：这里说不出「合成成功」。
    expect(source).not.toContain('合成成功')
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
  it('derives the op column width from the buttons a row can show', () => {
    expect(source).toContain("opColumnWidth(['详情', '取消', '重试'])")
    expect(source).not.toContain("width: 130, fixed: 'right'")
  })

  // 顺序即查找顺序：先认团队/策略，再认负责人，然后才是触发方式、状态与时间；
  // 策略名长，仍占宽位。逐项按 label 文本定位（不是裸词 indexOf），顺序写错时这里一定红。
  it('orders the filter bar 团队 → 合成策略 → 负责人 → 触发方式 → 状态 → 创建时间', () => {
    const bar = source.slice(source.indexOf('class="filter-bar"'), source.indexOf('table-scroll-wrap'))
    expect(bar).not.toContain('<t-space')
    const labels = [...bar.matchAll(/wt-filter-field__label">([^<]+)</g)].map((match) => match[1])
    expect(labels).toEqual(['团队', '合成策略', '负责人', '触发方式', '状态', '创建时间'])
    const fields = bar.match(/wt-filter-field[^"]*"/g) || []
    expect(fields.filter((entry) => entry.includes('wt-filter-field--wide'))).toHaveLength(1)
    // 宽位只归策略（团队、负责人都是窄字段）：宽类之后先出现策略标题，再出现负责人标题。
    const wideAt = bar.indexOf('wt-filter-field--wide')
    const strategyAt = bar.indexOf('合成策略', wideAt)
    expect(strategyAt).toBeGreaterThan(wideAt)
    expect(bar.indexOf('负责人', wideAt)).toBeGreaterThan(strategyAt)
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

  // 重试受理范围钉在页面与服务端同一个判据上（retryPolicy）：已取消的、以及除
  // execution_uncertain 以外的一切失败，高级运营及以上才有按钮 —— 藏的是按钮，
  // 不是让人点了再被 409。
  it('gates the retry button by role and the single exclusion the server makes', () => {
    expect(source).toContain("import { UNCERTAIN_CODE, isRetryableTask, retryHint } from '../retryPolicy.js'")
    expect(source).toContain('canRetry.value && isRetryableTask(row)')
    expect(source).toContain("attempt.error_code === UNCERTAIN_CODE ? '不可直接重试' : '可由高级运营提交重试'")
    expect(source).toContain('client.retryComposeTask(item.id)')
    expect(source).toContain("currentUser.value?.role")
    // 旧的错误码白名单整条撤掉：它会把 input_missing 这类失败判成「不可重试」，
    // 而那条任务在新规则下没有第二个重跑入口。
    expect(source).not.toContain('NEW_TASK_CODES')
    expect(source).not.toContain('RETRYABLE_CODES')
  })

  // 「为什么没有重试按钮」的理由由 retryPolicy 一处给出（那里逐条钉着），页面只负责
  // 在概览与执行记录里用同一句 —— 同一件事不并存两种说法。
  it('explains in the detail why the retry button is not there', () => {
    expect(source).toContain('<p v-if="retryHint(detail, canRetry)" class="compose-detail__hint">{{ retryHint(detail, canRetry) }}</p>')
    expect(source).not.toContain('修正后新建任务')
  })

  // 人工重试是「重新获得完整容错配额」而不是「次数清零」：执行编号不回退
  // （compose_task_attempts 的 UNIQUE(task_id, attempt) 也不允许回退），文案里就不许出现清零。
  it('asks before retrying and says the retry budget is recalculated, not zeroed', () => {
    const dialog = source.slice(source.indexOf('function retry('), source.indexOf('function cancel('))
    expect(dialog).toContain("header: '重试任务'")
    expect(dialog).toContain('自动容错重试机会重新计算')
    expect(dialog).toContain('DialogPlugin.confirm')
    expect(dialog).not.toContain('清零')
  })

  it('lets seniors and admins review an uncertain execution with explicit verification', () => {
    expect(source).toContain("detail.error_code === 'execution_uncertain'")
    expect(source).toContain('旧 FFmpeg 已停止')
    expect(source).toContain('没有已提交成片')
    expect(source).toContain('reviewReady(')
    expect(source).toContain('client.resolveComposeUncertain(')
    expect(source).toContain('client.listComposeUncertainReviews(')
    expect(source).toContain('失败阶段：')
    // 同团队高级运营也能核对，不再是管理员专属。
    expect(source).toContain("const canReviewUncertain = computed(() => ['senior_operator', 'admin'].includes(currentUser.value?.role))")
    expect(source).toContain('<h4>待人工核对</h4>')
    expect(source).toContain('核对人 {{ review.actor_user_id }}')
    expect(source).not.toContain('待管理员核对')
    expect(source).not.toContain('仅管理员')
    expect(source).not.toContain('管理员才能核对')
  })

  // 已成功的任务可能保留上一次失败的 error_code（例如 329 第 1 次不明确、第 2 次成功）：
  // 详情里不能再挂一条「待核对」告警，执行记录照常保留那次失败。
  it('does not show a stale failure block on a successful task', () => {
    expect(source).toContain("v-if=\"detail.status === 'failed' && (detail.error_code || detail.error_message)\"")
    expect(source).toContain("v-if=\"detail.status === 'failed' && detail.error_code === 'execution_uncertain'\"")
  })

  // 详情头只有「任务详情 + ×」：不放 Hero 大卡，也不放任何业务按钮。
  it('drops the hero card and keeps business actions out of the header', () => {
    expect(source).not.toContain('compose-detail__hero')
    expect(source).toContain('header="任务详情"')
    expect(source).toContain('size="min(62vw, 880px)"')
    expect(source).toContain(':close-btn="true"')
    expect(source).not.toContain('>关闭<')
  })

  // 页脚只在真有业务动作时才渲染；三 Tab 与概览四分区不得增删改名。
  it('renders the footer only for an actionable task and keeps the four overview sections', () => {
    expect(source).toContain(':footer="detailFooterOps.length > 0"')
    expect(source).toContain("detailFooterOps.includes('cancel')")
    expect(source).toContain("detailFooterOps.includes('retry')")
    const overview = source.slice(source.indexOf('label="概览"'), source.indexOf('label="执行记录"'))
    expect((overview.match(/<h4>/g) || [])).toHaveLength(4)
    expect((source.match(/<t-tab-panel/g) || [])).toHaveLength(3)
    for (const label of ['概览', '执行记录', '配置快照']) expect(source).toContain(`label="${label}"`)
  })

  // 失败任务的成片区只陈述事实：未生成成片 + 为什么，不编一个占位成片。
  it('states that a failed task produced no output', () => {
    expect(source).toContain('任务执行失败，因此没有生成成片。')
    expect(source).toContain('未生成成片')
    expect(source).toContain("row.status === 'success' ? row.composite_output_id : null")
    // 当前阶段是纯文字，且阶段名带「中」的（视频合成中）不能直接接「失败」。
    expect(source).toContain(".replace(/中$/, '')")
  })

  // 成片只在「生成结果」里出现一次：它既是关联（点得进去）也是结果（长什么样），
  // 但两处都写就成了两个事实来源；生成时间也不再藏在成片后面那半句拼出来的话里。
  it('keeps the output link out of the related-info section and gives the time its own cell', () => {
    const related = source.slice(source.indexOf('<h4>关联信息</h4>'), source.indexOf('<h4>执行信息</h4>'))
    expect(related).toContain('关联素材')
    expect(related).toContain('合成策略')
    expect(related).not.toContain('生成成片')
    const result = source.slice(source.indexOf('<h4>生成结果</h4>'), source.indexOf('label="执行记录"'))
    expect(result).toContain('生成成片')
    expect(result).toContain('生成时间')
    expect(result).toContain("{{ outputSummary ? formatDateTime(outputSummary.created_at) : '—' }}")
    expect(source).not.toContain('生成于')
  })

  // 失败原因写进**详情**的任务信息并高亮：来查「为什么失败」的人先看的就是这一段，
  // 不必再翻到执行记录去对错误码。列表页不加列（列数见上面那条九列断言）。
  it('writes the failure reason into the detail task info, highlighted', () => {
    const info = source.slice(source.indexOf('<h4>任务信息</h4>'), source.indexOf('<h4>关联信息</h4>'))
    expect(info).toContain("v-if=\"detail.status === 'failed'\"")
    expect(info).toContain('失败原因')
    expect(info).toContain('safeComposeError(detail.error_code)')
    expect(source).toContain('.compose-detail__failure-reason { color: var(--wt-danger); font-weight: 500; }')
    expect(source).not.toContain("title: '失败原因'")
  })

  // 上面那条只钉得住字符串，钉不住「这支变量在页面上真能取到值」——本轮就踩过：规则写着
  // var(--wt-status-error) 而字符串断言全绿，可那支语义表从未被任何入口引入，页面上是一片继承色。
  // 所以这里从**应用入口真正 import 的样式表**取定义，只放行 TDesign 自带的 --td-*。
  it('only uses CSS variables the loaded stylesheets define', () => {
    const entries = ['../../../apps/cloud/main.ts', '../../../apps/desktop/main.ts']
      .map((path) => new URL(path, import.meta.url))
    const sheets = entries.flatMap((entry) => {
      const code = readFileSync(entry, 'utf8')
      return [...code.matchAll(/import\s+"(\.[^"]+\.css)"/g)].map((m) => new URL(m[1], entry))
    })
    // 反向对照：解析不到样式表就说明这条测试在空转，不是「没问题」。
    expect(sheets.length).toBeGreaterThan(0)
    const defined = new Set()
    for (const sheet of sheets) {
      const css = readFileSync(sheet, 'utf8')
      for (const m of css.matchAll(/(--[a-z0-9-]+)\s*:/g)) defined.add(m[1])
    }
    const used = [...new Set([...source.matchAll(/var\((--[a-z0-9-]+)/g)].map((m) => m[1]))]
    expect(used.length).toBeGreaterThan(0)
    const unresolved = used.filter((name) => !name.startsWith('--td-') && !defined.has(name))
    expect(unresolved).toEqual([])
  })

  // 执行记录要看得见关键过程：每走完一步云端追加一条，页面把它列出来。步骤名是**服务端**
  // 下发的中文，前端不另建一份 key→中文 的映射（两份就会各错一份）。
  it('renders the recorded steps of every attempt', () => {
    expect(source).toContain("import { attemptSteps, stepMetricsText } from '../attemptSteps.js'")
    expect(source).toContain("import { formatDurationMs } from '../../../shared/utils/units.js'")
    expect(source).toContain('v-for="step in attemptSteps(attempt)"')
    expect(source).toContain("{{ step.label || '未记录' }}")
    expect(source).toContain('用时 {{ formatDurationMs(step.duration_ms) }}')
    expect(source).toContain('{{ stepMetricsText(step) }}')
    expect(source).toContain("step.outcome === 'failed'")
    expect(source).toContain("{{ step.error_label || '原因未记录' }}")
    expect(source).toContain('该次执行没有步骤记录。')
    const attemptsTab = source.slice(source.indexOf('label="执行记录"'), source.indexOf('label="配置快照"'))
    expect(attemptsTab).toContain('compose-detail__steps')
    expect(attemptsTab).not.toContain('step.error_message')
    expect(source).not.toContain("cut: '视频合成'")
  })

  // 分页只保留一套总数（TDesign 自带），页面不再自报一个。
  it('shows a single total count', () => {
    expect(source).not.toContain('共 {{ rows.length }} 条')
    expect(source).toContain(':total="rows.length"')
  })

  // 「前往…页面」是关联抽屉里唯一的路由入口：实体链接与表格行都不再换页。
  it('opens related entities in read-only drawers instead of navigating away', () => {
    expect(source).toContain('@click="openMaterial(row.material_id)"')
    expect(source).toContain('@click="openStrategy(row.compose_strategy_id)"')
    expect(source).toContain('@click="openOutput(outputID(row))"')
    expect(source).not.toContain('@click="goMaterial(')
    expect(source).not.toContain('@click="goStrategy(')
    expect(source).not.toContain('@click="goOutput(')
    for (const [fn, name] of [['goMaterialPage', 'MaterialLibrary'], ['goStrategyPage', 'ComposeStrategy'], ['goOutputPage', 'FinishedMedia']]) {
      const body = source.match(new RegExp(`function ${fn}\\([^)]*\\) \\{([^}]*)\\}`))?.[1] || ''
      expect(body, fn).toContain('router.push')
      expect(body, fn).toContain(`name: '${name}'`)
    }
    // 三层关联各用一份只读详情：素材复用现成组件，策略/成片复用抽出来的共享组件。
    expect(source).toContain('mode="readonly"')
    expect(source).toContain('<ComposeStrategySummary')
    expect(source).toContain('<ComposeOutputDetail')
    // 任务上下文里的成片抽屉不开「配置快照」：那一页会露出带签名的引导视频地址。
    expect(source).toContain(":tabs=\"['basic', 'source']\"")
    // 来源追踪里的编号全站都不带 `#`（用户裁定：`#` 只管关联实体 ID 的显示前缀），
    // 那个只为「我的成片页保留 #」而存在的开关连同它的默认值一起删了 ——
    // 留着就是一个「有的页面带、有的不带」的口子。
    expect(source).not.toContain('with-id-prefix')
    const outputDetail = readFileSync(new URL('../components/ComposeOutputDetail.vue', import.meta.url), 'utf8')
    expect(outputDetail).not.toContain('withIdPrefix')
    expect(outputDetail).not.toContain('#${')
  })

  // 「前往…页面」那一跳要真能落到对象上：目标页得认这个 query（深链是唯一的路由出口）。
  it('navigates to pages that still consume the deep-link query', () => {
    const strategies = readFileSync(new URL('./ComposeStrategyPage.vue', import.meta.url), 'utf8')
    const outputs = readFileSync(new URL('./MyOutputsPage.vue', import.meta.url), 'utf8')
    const materials = readFileSync(new URL('../../materials/pages/MaterialLibraryPage.vue', import.meta.url), 'utf8')
    expect(materials).toContain('client.get(Number(raw))')
    expect(strategies).toContain('client.getStrategy(Number(id)).then(openDetail)')
    expect(outputs).toContain('openDetail({ id: Number(id) })')
  })

  // 活动任务自动刷新：只复用模块内的最小轮询，页面自己不起 timer。
  it('auto-refreshes the list every ten seconds and polls an open detail only while a task is active', () => {
    // 列表：页面开着就定时重读（T-25 用户裁定），不再靠刷新按钮或「有活任务才轮询」。
    expect(source).toContain("import { useAutoRefresh } from '../../../shared/utils/useAutoRefresh.js'")
    expect(source).toContain('useAutoRefresh(() => load({ quiet: true }),')
    expect(source).not.toContain('hasActiveTask(rows.value)')
    expect(source).not.toContain('function syncListPolling()')
    // 详情：抽屉开着且任务还在跑时才 5 秒轮询，这一份保留。
    expect(source).toContain('useTaskPolling(')
    expect(source).toContain('function syncDetailPolling()')
    expect(source).toContain('detailPolling.start()')
    expect(source).toContain('detailPolling.stop()')
    // 定时那一遍不掀整页 Loading、不把用户正在看的页码拨回去。
    expect(source).toContain('load({ quiet: true })')
    expect(source).toContain('if (!quiet) pagination.value.current = 1')
    // 刷新按钮已移除；页面自己不建 timer（交给 useAutoRefresh）。
    expect(source).not.toContain('@click="load()">刷新')
    expect(source).not.toContain('setInterval')
  })

  // 关联实体仍按各自的详情端点取数，不拿列表行的副本充当详情。
  it('reads the related entity details through their own endpoints', () => {
    expect(source).toContain('materialsClient.get(id)')
    expect(source).toContain('client.getStrategy(id)')
    expect(source).toContain('client.getComposeOutput(id)')
    expect(source).toContain('client.getComposeOutputVideoUrl(id)')
    // 每次打开都从原接口取准确对象地址，再转换为可播放地址。
    expect(source).toContain('client.getComposeOutputVideoUrl(id).catch(() => null)')
    expect(source.match(/publicObjectReadUrl\(link\?\.url\)/g)).toHaveLength(2)
  })
})
