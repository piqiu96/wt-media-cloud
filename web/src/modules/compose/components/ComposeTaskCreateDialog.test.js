import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ComposeTaskCreateDialog.vue', import.meta.url), 'utf8')
const caller = readFileSync(new URL('../pages/ComposeTasksPage.vue', import.meta.url), 'utf8')

// 创建合成任务的**唯一**入口在合成任务页（我的素材里的创建入口与「加入合成」已撤）。
// 素材因此只能在这个抽屉里选，候选由抽屉自己去「我的素材」取。这里钉住会静默失效的
// 接线与几句不能写错的文案。
describe('compose task create drawer', () => {
  it('keeps the caller contract: visible in, created out, nothing else', () => {
    expect(source).toContain("visible: { type: Boolean, default: false }")
    expect(source).toContain("defineEmits(['update:visible', 'created'])")
    expect(source).toContain('@update:visible="emit(\'update:visible\', $event)"')
    // 宿主只剩合成任务页；素材范围不再由调用方注入（那个 prop 已随旧入口一起删掉）。
    expect(source).not.toContain('materials: { type: Array')
    expect(caller).toContain('v-model:visible="createVisible"')
    expect(caller).toContain('@created="onCreated"')
    expect(caller).not.toContain(':materials="')
    expect(source).not.toContain('<t-dialog')
  })

  // 候选就是「我的素材」里使用中的关系：已放弃的不允许新建任务（PRD §5.4.5），过滤落在
  // 候选集上。下载状态不参与过滤 —— 下载原素材是任务自己工作的一部分。
  it('takes its candidates from my materials, in-use only', () => {
    expect(source).toContain('materialsClient.listMyMaterials()')
    expect(source).toContain("usage.status === 'active' && usage.material")
    expect(source).not.toContain('download_status')
    expect(source).toContain('只列出「我的素材」里使用中的素材')
  })

  // 停用游戏不能再建策略，也不该出现在素材的游戏筛选里。候选本身不缩小：
  // 这里只决定「能筛哪些游戏」，已经选中的素材与全量候选都不受影响。
  it('offers only enabled games in the candidate filter', () => {
    expect(source).toContain("import { enabledGames } from '../../../shared/utils/games.js'")
    expect(source).toContain('const enabledIds = new Set(enabledGames(games.value).map((game) => game.id))')
    expect(source).toContain('.filter((id) => enabledIds.has(id))')
    expect(source).toContain('games.value.find((game) => game.id === id)?.name || id')
  })

  // 负责人必须来自策略（M4-I）：提交时把它一起发出去，策略随后被改负责人时服务端
  // 会拒掉这次创建而不是悄悄换人；未指派负责人的策略在这里就拦下并说明去哪里补。
  it('carries the strategy operator into the task and refuses an unassigned strategy', () => {
    expect(source).toContain('operator_id: selectedStrategy.value.operator_id')
    expect(source).toContain("if (!selectedStrategy.value?.operator_id)")
    expect(source).toContain('该策略尚未指定负责人，请先在合成策略页补选后再创建')
    expect(source).toContain('compose-create__confirm-label">负责人')
    expect(source).toContain("selectedStrategy.value?.operator_name || '未记录'")
  })

  // 策略排在第 1 段：素材是否已经用过是按策略算的，先选它这块才有意义。
  it('lays the form out in the four fixed sections, strategy first', () => {
    const sections = [...source.matchAll(/compose-create__title"><span>(\d)<\/span>\s*([^<]+)</g)]
      .map((match) => [match[1], match[2].trim()])
    expect(sections).toEqual([['1', '合成策略'], ['2', '已选素材'], ['3', '可覆盖参数'], ['4', '提交确认']])
  })

  // 候选是一张内联列表，不再是一个多选下拉：长标题在下拉里变成标签墙，与下面的已选列表
  // 说的是同一件事，还把搜索框挤没了。筛选行（游戏 + 使用情况）+ 搜索框 + 行内按钮取代它。
  it('lists candidates inline, filtered by game, usage and a keyword', () => {
    expect(source).toContain('v-model="candidateUsage"')
    expect(source).toContain(':options="usageOptions"')
    expect(source).toContain('v-model="candidateKeyword"')
    expect(source).toContain('搜索素材标题或素材 ID')
    expect(source).toContain('@click="addMaterial(row.id)"')
    // 文案与可用性都来自 materialStates（那里逐条钉着），这里只钉渲染接线：标签按 tone 上色，
    // 按钮的文字与禁用态一起取。
    expect(source).toContain('{{ row.tag.label }}')
    expect(source).toContain('`compose-create__tag--${row.tag.tone}`')
    expect(source).toContain('{{ rowAction(row).label }}')
    expect(source).toContain(':disabled="rowAction(row).disabled"')
    // 多选下拉整块撤掉：留着它就会出现「搜索区」与「已选区」各说一遍同一批素材。
    expect(source).not.toContain('materialOptions')
    expect(source).not.toContain('filterMaterial')
    expect(source).not.toContain('multiple')
    expect(source).not.toContain('filterable')
  })

  // 需求（第五轮走查）：候选默认按发布时间倒序，并给一个排序字段。排序值传给 materialStates
  // 的 candidateRows —— 只换下拉不传参的话，切了排序列表一动不动。
  it('sorts the candidates by the field the operator picked, newest published by default', () => {
    expect(source).toContain('v-model="candidateSort"')
    expect(source).toContain(':options="CANDIDATE_SORTS"')
    expect(source).toContain('aria-label="排序"')
    expect(source).toContain("const candidateSort = ref('published_desc')")
    expect(source).toContain('candidateRows(visibleCandidates.value, statesById.value, candidateUsage.value, candidateKeyword.value, candidateSort.value)')
    expect(source).toContain("candidateSort.value = 'published_desc'")
  })

  // 需求（第五轮走查）：点赞与收藏随行展示，运营在这块挑素材时不必再点开详情。
  // 数据本来就在素材接口里，是候选映射没接。
  it('shows the likes and favourites it carries on the row', () => {
    expect(source).toContain('like_count: usage.material.like_count')
    expect(source).toContain('favorite_count: usage.material.favorite_count')
    expect(source).toContain('published_at: usage.material.published_at')
    expect(source).toContain('点赞 {{ countLabel(row.like_count) }} · 收藏 {{ countLabel(row.favorite_count) }}')
    expect(source).toContain('function countLabel(value) { return Number(value || 0).toLocaleString() }')
  })

  // 一条 (策略, 素材) 只跑一次：有过任务记录的行整行只读，连按钮都不画（画一个禁用按钮
  // 会让人以为是权限或状态问题），重跑在任务页对那条任务点「重试」。
  it('offers no button at all for a pair that already has a task record', () => {
    expect(source).toContain('v-if="rowAction(row)"')
    expect(source).toContain('这条策略下已有任务记录的素材是只读的')
    expect(source).toContain('这条策略下还没有已使用过的候选素材。')
    // 「再生成一版」随新规则撤掉：它是抽屉里的第二个重跑入口。
    expect(source).not.toContain('再生成一版')
  })

  // 使用情况是一个字段（两个取值），与「游戏」并排；状态没读回来之前它不可用，行上的按钮
  // 也不给 —— 此刻「没用过」和「不知道」长得一样。
  it('offers 未使用过/已使用过 as one field with counts', () => {
    expect(source).toContain("const candidateUsage = ref('unused')")
    expect(source).toContain('未使用过（${counts.value.unused}）')
    expect(source).toContain('已使用过（${counts.value.used}）')
    expect(source).toContain(':disabled="!statesReady"')
    expect(source).toContain("if (!statesReady.value) return { label: '状态读取中', disabled: true }")
    // 成功过的行要说出「已生成 N 版」：运营看得出这条素材在这条策略下出了几版成片，
    // 而不是只知道「用过」。
    expect(source).toContain('已生成 {{ row.state.success_count }} 版')
  })

  // 读状态失败就整块降级：宁可让人看到「读不到」+ 重试，也不静默把一切当成未使用过 ——
  // 那正是会诱导重复创建的那个答案。
  it('reads the usage states per strategy and degrades loudly when it cannot', () => {
    expect(source).toContain('client.listStrategyMaterialStates(strategyId.value, candidates.value.map((item) => item.id))')
    expect(source).toContain('watch(strategyId, () => {')
    expect(source).toContain('statesReady.value = false')
    expect(source).toContain('statesError.value = e?.message || \'读取素材使用情况失败\'')
    expect(source).toContain('<FeedbackAlert v-if="statesError"')
    // 没选策略时只给一句提示，不去问服务端。
    expect(source).toContain('先在上一步选择合成策略：素材是否已经用过，是按策略算的。')
    expect(source).toContain('if (!strategyId.value || candidates.value.length === 0) return')
  })

  // 渲染上限只作用于未选中的行，且被省略的条数要说出来。
  it('caps the rendered candidates without hiding a selected row', () => {
    expect(source).toContain('cappedRows(rows.value, selectedIds.value, CANDIDATE_ROW_LIMIT)')
    expect(source).toContain('还有 {{ visibleRows.omitted }} 条未列出，用上面的搜索缩小范围。')
  })

  it('puts both actions in the footer and nothing in the header but the close button', () => {
    const footer = source.slice(source.indexOf('<template #footer>'))
    expect(footer).toContain('>取消</t-button>')
    expect(footer).toContain('>创建任务</t-button>')
    expect(footer).toContain('theme="primary" :disabled="selected.length === 0" :loading="saving"')
    expect(footer).not.toContain('>关闭</t-button>')
    expect(source).toContain('header="创建合成任务"')
    expect(source).toContain(':close-btn="true"')
    expect(source).toContain('size="min(62vw, 880px)"')
    expect(source).toContain(':close-on-overlay-click="false"')
  })

  // 提交期间主按钮进 Loading，重复点击与空选择都不发请求。
  it('guards against a double submit and against an empty selection', () => {
    expect(source).toContain('if (saving.value || selected.value.length === 0) return')
    expect(source).toContain("strategyError.value = '请选择合成策略'")
  })

  // 每条素材独立一条任务、一个幂等键：键在素材进入选择时生成，抽屉存续期间不变 ——
  // 保存按钮的双击/重试命中的是服务端唯一键，而不是造出两批任务。
  it('gives every selected material its own stable request key', () => {
    expect(source).toContain('crypto.randomUUID()')
    expect(source).toContain('watch(selectedIds, (ids) => ensureKeys(ids))')
    expect(source).toContain('for (const id of ids) if (!next[id]) next[id] = crypto.randomUUID()')
    expect(source).toContain('items: selected.value.map((material) => ({ material_id: material.id, request_key: requestKeys.value[material.id] }))')
    expect(source).toContain('每条素材独立生成一条合成任务')
  })

  // 留空 = 用策略值：空串发上去会被 schema 当成显式的空值拒绝。口径只有 taskOverrides 一份。
  it('only sends the parameters the operator actually filled in', () => {
    expect(source).toContain('const overridePayload = computed(() => buildOverridePayload(fields.value, strategyConfig.value, overrides.value))')
    expect(source).toContain('if (Object.keys(overridePayload.value).length > 0) body.config_override = overridePayload.value')
    expect(source).toContain('参数留空即使用策略里保存的值')
  })

  // 需求：参数默认显示策略里保存的值。空着不等于「没设置」，只会让人以为策略值没生效。
  it('prefills the overridable parameters from the chosen strategy', () => {
    expect(source).toContain("import { buildOverridePayload, prefillOverrides, validateOverrides } from '../taskOverrides.js'")
    expect(source).toContain('overrides.value = prefillOverrides(fields.value, strategy.config)')
    // 策略与模板并发到达，字段清单要等模板到才算得出来；这个值保证只按一条策略填一次，
    // 换策略才整体重填 —— 上一条策略的覆盖值对新策略没有意义。
    expect(source).toContain('watch([strategyId, fields], () => {')
    expect(source).toContain('if (prefilledFor.value === strategy.id) return')
    expect(source).toContain('下面的值已按策略里保存的参数预填')
  })

  // 需求：创建时报的英文参数错误不该再出现。本地拦住就是服务端会拒的那几种情况，
  // 拦住了就不发请求 —— 而且要在按钮进 Loading 之前拦。
  it('blocks an invalid override locally, in Chinese, before any request', () => {
    const submit = source.slice(source.indexOf('async function submit()'), source.indexOf('</script>'))
    const validateAt = submit.indexOf('validateOverrides(fields.value, strategyConfig.value, overrides.value)')
    const requestAt = submit.indexOf('client.createComposeTasks(body)')
    expect(validateAt).toBeGreaterThan(-1)
    expect(requestAt).toBeGreaterThan(validateAt)
    expect(submit).toContain('if (Object.keys(overrideErrors.value).length > 0)')
    expect(submit.indexOf("feedback.toast.error('可覆盖参数有误")).toBeLessThan(submit.indexOf('saving.value = true'))
    expect(submit).toContain("e?.type === 'compose_task_invalid'")
    expect(submit).toContain('任务参数未通过校验，请检查「可覆盖参数」后重试')
    expect(submit).not.toContain('basic_cut')
  })

  it('marks the offending field and prints the Chinese reason under it', () => {
    expect(source).toContain(":status=\"overrideErrors[field.key] ? 'error' : undefined\"")
    expect(source).toContain('<FormError :message="overrideErrors[field.key]" />')
  })

  // 需求：选素材要能按游戏筛、按名称搜。切游戏只缩候选，已选素材不能跟着消失。
  it('filters the candidates by game without losing the selection', () => {
    expect(source).toContain('client.listGames()')
    expect(source).toContain('v-model="gameFilter"')
    expect(source).toContain('const visibleCandidates = computed')
    expect(source).toContain('game_id: usage.material.game_id')
    expect(source).toContain('按游戏筛选只缩小候选，不会取消已选中的素材')
    // 已选取自**全量**候选：改成 visibleCandidates 会让切游戏时已选素材凭空消失。
    const selectedLine = source.slice(source.indexOf('const selected = computed'), source.indexOf('\n', source.indexOf('const selected = computed')))
    expect(selectedLine).toContain('candidates.value')
    expect(selectedLine).not.toContain('visibleCandidates')
  })

  // 需求：创建时可填任务名称；多条素材的序号由**服务端**加，前端只把名字原样带上。
  it('carries an optional task name and previews the numbering', () => {
    const submit = source.slice(source.indexOf('async function submit()'), source.indexOf('</script>'))
    expect(source).toContain('v-model="taskName"')
    expect(source).toContain(':maxlength="60"')
    expect(submit).toContain('if (taskName.value.trim()) body.name = taskName.value.trim()')
    expect(submit).not.toContain('body.name = `')
    expect(source).toContain("taskName.value = ''")
    const confirm = source.slice(source.indexOf('4</span> 提交确认'), source.indexOf('</section>', source.indexOf('4</span> 提交确认')))
    expect(confirm).toContain('{{ nameSummary }}')
    expect(source).toContain('未填写，按素材与策略自动生成')
  })

  // 只有启用中的策略可选、覆盖参数只列模板 schema 允许的字段（含引导视频这类地址）。
  it('offers only enabled strategies and only schema-allowed parameters', () => {
    expect(source).toContain("item.status === 'enabled'")
    expect(source).toContain("schemaFields(template?.schema).filter((field) => field.key !== 'guide_video_url')")
    expect(source).toContain('只有启用中的策略可选')
  })

  // 提交成功只能说「已创建 / 已提交」——合成成片是异步作业，这里说不出「合成成功」。
  it('never claims the composite is finished', () => {
    expect(source).not.toContain('合成成功')
    expect(source).toContain('提交后任务进入队列')
    expect(caller).toContain('已创建 ${created.length} 个合成任务')
  })

  // 第四段是纯展示：它复述这一次提交真正会发生什么，不产生任何新请求。
  it('summarises the pending submission without inventing fields', () => {
    const confirm = source.slice(source.indexOf('4</span> 提交确认'), source.indexOf('</section>', source.indexOf('4</span> 提交确认')))
    expect(confirm).toContain('{{ selected.length }} 条')
    expect(confirm).toContain('{{ strategyName }}')
    expect(confirm).toContain("overrideSummary.length ? overrideSummary.join('，') : '未覆盖，使用策略里保存的值'")
    expect(confirm).not.toContain('client.')
  })
})
