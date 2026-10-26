import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MaterialLibraryPage.vue', import.meta.url), 'utf8')

/** 取出两段定界文本之间的内容；找不到就抛，别让断言在空串上静默通过。 */
function sliceBetween(text, start, end) {
  const from = text.indexOf(start)
  const to = text.indexOf(end, from)
  if (from === -1 || to === -1) throw new Error(`找不到切片：${start} … ${end}`)
  return text.slice(from, to)
}

// 「素材库不提供下载」这条规则管的是**行操作与即时反馈**：CHG-104 起素材库不再有
// 任何本机下载入口，下载只从「我的素材」发起（服务端也只认那边的关系）。
// 切的终点是 `</t-space>` 而不是第一个 `</template>`：操作格里有嵌套的
// `<template v-else>—</template>`，按 `</template>` 切会把动作条从「—」处剪断。
const opBlock = sliceBetween(source, '#op=', '</t-space>')

describe('material library page', () => {
  /**
   * `?material_id=N` 是一个**真实入口**，不是装饰。
   *
   * 内容池里「已转素材」那一列（`ContentPoolPage.vue`）与挖掘任务的发现结果
   * （`CrawlTasksPage.vue`）都会跳到这个地址。它们的落点要求页面能打开**任何**一条素材，
   * 而不只是恰好在第一页、又没被筛选掉的那一条 —— 所以走 `getMaterial`（单条接口），
   * 不是在已经加载的行里找。
   *
   * 这段契约原先由 `ContentPoolPage.test.js` 的一条用例钉着，钉的是旧的实现
   * （内容池页按 `route.path` 把自己渲染成素材库的只读投影）。投影被真实页面取代后，
   * 那条用例随之删除，契约重建在这里。
   */
  it('opens the material a deep link names, through the single-material endpoint', () => {
    expect(source).toContain('route.query.material_id')
    // 非数字的 id 不请求：`/materials/NaN` 只会换来一个 400，而那个 400 读起来像服务端坏了。
    expect(source).toContain('/^\\d+$/.test(String(raw))')
    expect(source).toContain('await client.get(Number(raw))')
    // 单条接口，而不是在行里找：深链必须能打开分页外或筛选外的那一条。
    expect(source).toContain('async function openFromQuery()')
    // 挂载时会话要先到（默认游戏筛选依赖它），深链的打开跟着走。
    expect(source).toMatch(/onMounted\(async \(\) => \{[\s\S]*openFromQuery\(\)/)
  })

  // 所有筛选都由服务端读 query；页面不再对返回行做二次过滤。
  // 团队范围由服务端按角色决定：普通运营/高级运营固定本团队，只有管理员的 `team_id`
  // 才是「目标团队」。页面必须把团队与游戏都交上去，游戏只是筛选而不是可见边界。
  it('sends every list filter to the server exactly once', () => {
    expect(source).toMatch(/client\.list\(\{[\s\S]*?search: search\.value/)
    for (const param of [
      'team_id: teamFilter.value',
      'game_ids: gameFilters.value',
      'joined: joinedFilter.value',
      'my_usage: myUsageFilter.value',
      'team_usage: teamUsageFilter.value',
      'video_status: statusFilter.value',
      'sort_by: sortBy.value',
      'sort_order: sortOrder.value',
    ]) {
      expect(source, `列表请求必须带上 ${param}`).toContain(param)
    }
    expect(source.match(/client\.list\(/g)).toHaveLength(1)
    expect(source).not.toContain('filteredRows')
    expect(source).not.toContain('row.video_status === statusFilter.value')
    expect(source).not.toContain('row.game_id === gameFilters.value')
  })

  // 普通运营与高级运营进页面时，游戏多选默认本人负责的游戏；管理员默认不筛（读所选团队）。
  // 选项是游戏管理里已启用的全量游戏：本人负责只决定默认勾选，不限制能筛到哪些游戏。
  it('offers every enabled game and defaults to the responsible ones', () => {
    expect(source).toContain('function responsibleGames()')
    expect(source).toContain("currentUser.value?.role === 'admin'")
    expect(source).toContain('return Array.isArray(currentUser.value?.game_ids) ? [...currentUser.value.game_ids] : []')
    expect(source).toContain('function defaultGameFilters()')
    expect(source).toContain('const enabled = new Set(games.value.map((game) => game.id))')
    expect(source).toContain('return responsibleGames().filter((id) => enabled.has(id))')
    expect(source).toContain('gameFilters.value = defaultGameFilters()')
    expect(source).toContain("users.listGames({ scope: 'all', status: 'enabled' })")
    expect(source).toContain('const gameOptions = computed')
    expect(source).toContain('games.value.map((game) => ({ id: game.id, name: game.name }))')
    // 列表要等会话与游戏字典一起到齐再取：默认项不能引用还没加载的选项。
    expect(source).toContain('await Promise.all([loadSession(), loadGames()])')
    expect(source.indexOf('await Promise.all([loadSession(), loadGames()])')).toBeLessThan(source.indexOf('load()\n  loadTeams()'))
    // 选项不再从已加载的行里补。
    expect(source).not.toContain('for (const row of rows.value)')
    expect(source).not.toContain('gameName(games.value, id)')
  })

  // 团队筛选只在管理员手里出现，其余角色连入口都没有：团队是服务端给的边界，
  // 给一颗筛不动的下拉就是「看起来能跨团队」。
  it('shows the team filter to admins only and loads the directory for it', () => {
    expect(source).toContain('const isAdmin = computed(() => currentUser.value?.role === \'admin\')')
    expect(source).toContain('v-if="isAdmin"')
    expect(source).toContain('users.listTeams()')
    expect(source).toMatch(/async function loadTeams\(\) \{\s*if \(!isAdmin\.value\) return/)
  })

  it('defaults to published_at descending and lets ID/time columns toggle the order', () => {
    expect(source).toContain("const sortBy = ref('published_at')")
    expect(source).toContain("const sortOrder = ref('desc')")
    expect(source).toContain("{ colKey: 'id', title: '素材 ID', width: 90, sorter: true }")
    expect(source).toContain("{ colKey: 'published_at', title: '发布时间', width: 150, sorter: true }")
    expect(source).toContain("{ colKey: 'created_at', title: '入库时间', width: 105, sorter: true }")
    expect(source).toContain("sortBy.value = sort?.sortBy || 'published_at'")
    expect(source).toContain("sortOrder.value = sort?.descending === false ? 'asc' : 'desc'")
    expect(source).toContain('@sort-change="changeSort"')
  })

  it('adds to my materials through the one idempotent command', () => {
    expect(source).toContain('await client.addUsage(row.id)')
    // 200 与 201 的响应体相同，客户端分支不了 —— 也就不该假装能分支。
    expect(source).not.toContain('created ===')
  })

  it('renders the four video states through the shared formatters', () => {
    expect(source).toContain('videoStatusLabel(row.video_status)')
    expect(source).toContain('videoStatusTone(row.video_status)')
  })

  // 走查四轮（2026-09-30 用户带设计图 + 裁定）：封面、标题、来源平台合成一个「素材」
  // 单元格，减少列数；素材 ID 仍独立成列且是第一业务列（规范 §5.1，用户明确裁定保留）。
  it('collapses cover, title and platform into one material cell behind a leading id column', () => {
    const idAt = source.indexOf("{ colKey: 'id', title: '素材 ID'")
    const materialAt = source.indexOf("{ colKey: 'material', title: '素材'")
    expect(idAt).toBeGreaterThan(-1)
    expect(materialAt).toBeGreaterThan(-1)
    expect(idAt, '素材 ID 列仍必须是第一业务列').toBeLessThan(materialAt)
    // 三样识别信息不再各自成列。
    for (const gone of ["title: '封面'", "title: '标题'", "title: '来源平台'"]) {
      expect(source, `不该再有 ${gone} 列`).not.toContain(gone)
    }
    expect(source).toContain('<template #material="{ row }">')
    expect(source).toContain('MaterialCover')
    expect(source).toMatch(/<a[^>]*:href="row\.source_url"/)
    expect(source).toContain('{{ row.platform')
    // 走查反馈：ID 不带 # 前缀——运营要把这串数字复制去别处查，# 只会跟着被复制。
    expect(source).toContain('<template #id="{ row }">{{ row.id }}</template>')
    expect(source).not.toContain('#{{ row.id }}')
    // 副行只带来源平台；游戏有自己的一列，同一格里再说一遍就是重复。
    expect(source).not.toContain('· {{ row.author_name')
    expect(source).not.toContain('formatBytes(row.video_size_bytes)')
  })

  // 列名换成了「文件状态」，因为它描述的是源视频文件的准备进度，不是这条素材的业务状态。
  // 取值没换，仍是 video_status 四态。
  it('names the video status column 文件状态 and keeps the four-state labels', () => {
    expect(source).toContain("{ colKey: 'video_status', title: '文件状态'")
    expect(source).not.toContain("title: '视频状态'")
    expect(source).toContain('videoStatusLabel(row.video_status)')
    expect(source).toContain('videoStatusTone(row.video_status)')
  })

  /**
   * CHG-104：素材库一个本机下载入口都不给 —— 行、详情抽屉、加入成功的反馈都没有。
   * 下载只从「我的素材」发起，服务端也只认那边的关系；素材库这边留一颗「下载」，
   * 点下去必然被拒（直接调下载接口要 403），那比没有按钮更糟。
   *
   * 反馈里给的是「去我的素材」这一步：加进去之后，下载在那边的行上做。
   */
  it('offers no local download anywhere on the library page', () => {
    expect(opBlock).not.toContain('下载')
    expect(source).not.toContain('createDownload')
    expect(source).not.toContain('downloadErrors')
    expect(source).not.toContain('@download')
    expect(source).not.toContain('downloadNow')
    const toast = sliceBetween(source, 'function notifyAdded(', '\n}')
    expect(toast).toContain('feedback.toast.success')
    expect(toast).toContain('已加入我的素材')
    expect(toast).toContain('去我的素材')
    expect(toast).not.toContain('下载')
  })

  // 「已加入 → 去我的素材」「已移出 → 重新加入」都读服务端投影的 `usage_status`，
  // 不再拉一次 `/my-materials` 在客户端 join。团队版的关系查询是服务端的事，
  // 页面再拼一份，等于维护第二个真值来源。
  it('reads the viewer relation from the server projection instead of joining my materials', () => {
    expect(source).toContain('row.usage_status')
    expect(source).toContain('<template #joined="{ row }">')
    expect(source).toContain('joinStatus(row).tone')
    expect(source).toContain('joinStatus(row).label')
    expect(source).not.toContain('listMyMaterials')
    expect(source).not.toContain('mineIds')
    expect(source).not.toContain('isMine(')
  })

  // 走查四轮（用户裁定 + 设计图）：已加入的行主操作是「去我的素材」，
  // 未加入的行仍是「加入我的素材」，已移出的行是「重新加入」。三个主操作互斥。
  it('swaps the row primary action with the viewer relation', () => {
    expect(opBlock).toContain('@click="openDetail(row)">详情</t-button>')
    expect(opBlock).not.toContain('>查看</t-button>')
    expect(opBlock).toContain("row.usage_status !== 'active'")
    expect(opBlock).toContain("row.usage_status === 'removed' ? '重新加入' : '加入我的素材'")
    expect(opBlock).toContain('@click="addToMine(row)"')
    expect(opBlock).toContain('@click="goToMyMaterials()">去我的素材</t-button>')
    expect(source).toMatch(/v-else-if="row\.usage_status === 'active'"[\s\S]{0,120}?goToMyMaterials\(\)/)
    // 跳转目标就是「我的素材」页，两个端同名（cloud / desktop router 都是 MyMaterial）。
    expect(source).toContain("{ name: 'MyMaterial' }")
  })

  // 加入成功之后重读列表，让加入状态、我的使用与操作按钮一起翻过来：
  // 只在本行改一个本地标记，会让三列里恰好只有一列说真话。
  it('re-reads the list after joining so every projected field flips together', () => {
    const add = sliceBetween(source, 'async function addToMine(', '\n}')
    expect(add).toContain('await client.addUsage(row.id)')
    expect(add).toContain('await load()')
    expect(add).toContain('notifyAdded()')
  })

  // 详情抽屉是两页共用的一个组件：各写一份的结局是同一个素材在两页显示得不一样，
  // 且没有任何东西会报错。素材库上下文按服务端投影的加入状态决定页脚主操作。
  it('uses the shared material detail drawer in library mode', () => {
    expect(source).toContain("import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'")
    expect(source).toContain('<MaterialDetailDrawer')
    expect(source).toContain('mode="library"')
    expect(source).toContain(':show-usage="canViewUsage"')
    expect(source).toContain('@add="addToMine"')
    expect(source).toContain('@go-mine="goToMyMaterials"')
    expect(source).toContain(':mine="detail?.usage_status === \'active\'"')
    expect(source).not.toContain('material-detail-drawer')
  })

  // 走查五轮（2026-09-30 用户走查 + 设计图）：列表要同时给出两个状态维度。
  // 「文件状态」说的是源视频准备好了没有，「素材状态」说的是这条素材还提供给运营选用吗，
  // 规范 §7.2 明令两者不得合并——同一个素材可以同时是「已暂停 + 可下载」。
  it('puts the material status column next to the file status column', () => {
    expect(source).toContain("{ colKey: 'material_status', title: '素材状态'")
    expect(source.indexOf("{ colKey: 'material_status'")).toBeLessThan(source.indexOf("{ colKey: 'video_status'"))
    expect(source).toContain('materialStatusLabel(row.status)')
    expect(source).toContain('materialStatusTone(row.status)')
  })

  /**
   * CHG-104：三个使用维度各自成列，依次排在文件状态之后、入库时间之前。
   *
   * 加入状态回答「当前这个人有没有把这条素材放进自己的工作集」；我的使用回答
   * 「当前这个人有没有用它做出过成功成片」；使用情况回答「本团队有没有人用它做出过
   * 成功成片、都是谁」。三个问题各有自己的真值来源（关系表 / 成片表按本人 / 成片表按
   * 团队），合成一列就是把三个答案压成一个数，规范 §7.2 明令不得合并。
   */
  it('puts 加入状态, 我的使用 and 使用情况 between the file status and 入库时间', () => {
    const joinedAt = source.indexOf("{ colKey: 'joined'")
    const myUsageAt = source.indexOf("{ colKey: 'my_usage'")
    const usageAt = source.indexOf("{ colKey: 'usage'")
    expect(source).toContain("{ colKey: 'joined', title: '加入状态'")
    expect(source).toContain("{ colKey: 'my_usage', title: '我的使用'")
    expect(source).toContain("{ colKey: 'usage', title: '使用情况'")
    expect(joinedAt).toBeGreaterThan(source.indexOf("{ colKey: 'video_status'"))
    expect(joinedAt).toBeLessThan(myUsageAt)
    expect(myUsageAt).toBeLessThan(usageAt)
    expect(usageAt).toBeLessThan(source.indexOf("{ colKey: 'created_at'"))
    expect(source).toContain('myUsageTone(row.my_usage)')
    expect(source).toContain('myUsageLabel(row.my_usage)')
  })

  // 使用情况对普通运营完全不存在：列、筛选、详情段都不渲染；高级运营/管理员只有
  // 一行可点摘要，明细进弹窗。没有已知使用人时也不写占位文案。
  it('shows usage to seniors and admins only, behind a one-line summary and a dialog', () => {
    expect(source).toContain("const isSenior = computed(() => currentUser.value?.role === 'senior_operator')")
    expect(source).toContain('const canViewUsage = computed(() => isAdmin.value || isSenior.value)')
    expect(source).toContain("canViewUsage.value ? columns : columns.filter((col) => col.colKey !== 'usage')")
    expect(source).toContain('<label v-if="canViewUsage"')
    expect(source).toContain('<template #usage="{ row }">')
    expect(source).toContain('usageSummary(row)')
    expect(source).toContain('@click="openUsage(row)"')
    expect(source).toContain('成功成片 {{ usageOutputCount(usageMaterial) }}')
    expect(source).toContain('v-for="operator in usageOperators(usageMaterial)"')
    expect(source).toContain("{{ operator.name }}")
    expect(source).toContain("成片 {{ operator.output_count }}")
    // 多人时列表自己滚动，第二行不会被挤出弹窗。
    expect(source).toContain('.usage-detail__list { max-height: 280px;')
    expect(source).toContain('overflow-y: auto')
    // 不再有那三句占位文案，也不再有第二行姓名。
    for (const gone of ['暂无成功成片', '负责人未记录', '姓名不可见', 'usageSummaryLines']) {
      expect(source, `不该再有 ${gone}`).not.toContain(gone)
    }
    expect(source).toContain(':show-usage="canViewUsage"')
  })

  /**
   * 服务端还没有 `status` 字段（第五章「素材状态和生命周期」未落，change.md §3 明确不做）。
   * 用户 2026-09-30 裁定：先把样式与协议立起来，数据打通后再同步过来。所以这一列此刻
   * 渲染的是**占位**，不是编造的状态。
   *
   * 「编造」的具体形状就是 `row.status || 'available'` 这类兜底——它看起来像容错，
   * 实际是把一条读不到状态的素材静默画成「可用」，而「可用」正是能触发「加入我的素材」
   * 的那一个取值。
   */
  it('renders placeholders instead of guessing a state the server never sent', () => {
    // 断言只量这一格：整份源码里 `row.status || 'available'` 会出现在**讲解这条规则的
    // 注释里**，拿整份源码做否证会把说明当证据，读数是注释的函数。
    const cell = sliceBetween(source, '#material_status="{ row }"', '</template>')
    expect(cell).toContain('v-if="row.status"')
    expect(cell).toContain('v-else>—')
    expect(cell, '读不到状态就得是 —，不是被兜底成某个取值').not.toContain('available')
  })

  /**
   * 规范 §16.2 / 设计图要点④：暂停与下架**禁止新增领取**。
   *
   * 禁止的是**新增**，不是导航：已加入的行无论什么状态都还能去「我的素材」——
   * §16.2 那句「历史关系和结果继续保留」说的正是这个。
   */
  it('refuses a new add on a paused or delisted material but keeps the navigation', () => {
    expect(source).toContain('function canAdd(row)')
    expect(source).toMatch(/canAdd\(row\)[\s\S]{0,140}?row\.status === 'available'/)
    expect(source).toMatch(/v-else-if="row\.usage_status === 'active'"/)
    // 没有可做主操作时画「—」，而不是留一颗必然被拒绝的按钮。
    expect(opBlock).toContain('<template v-else>—</template>')
  })

  // 走查三轮（交互对齐 §2.3/§5.3/§5.5）：行操作收敛为「详情 | 单一主操作」；
  // 「查看」是全系统要消灭的同义词。
  it('narrows each row to 详情 followed by one primary action', () => {
    expect(opBlock).toContain('@click="openDetail(row)">详情</t-button>')
    expect(opBlock).not.toContain('>查看</t-button>')
    // 行里只有这两颗：详情 + 主操作（加入或去我的素材），没有第二个业务动作。
    expect(opBlock.match(/<t-button/g)).toHaveLength(3)
  })
})
