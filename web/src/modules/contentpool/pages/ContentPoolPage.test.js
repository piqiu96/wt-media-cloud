import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ContentPoolPage.vue', import.meta.url), 'utf8')

describe('content pool page', () => {
  it('uses the shared resource layout and exposes the M3-A actions', () => {
    expect(source).toContain('ResourcePageHeader')
    // 概览指标与分页总数重复，已移除（AdminUI 规范 §3.3）。
    expect(source).not.toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('发现内容')
    expect(source).toContain('ID/链接发现')
    expect(source).toContain('关键词发现')
    expect(source).toContain('转素材')
    expect(source).toContain('批量转素材')
    expect(source).toContain('审核模式')
    expect(source).toContain('转素材并下一条')
    expect(source).toContain('忽略并下一条')
    expect(source).toContain('client.batchMaterialize')
    expect(source).toContain('strategy_id')
    expect(source).toContain('crawl_task_id')
    expect(source).toContain('ID/链接')
    expect(source).toContain('KeywordTags')
    expect(source).toContain('开始发现')
    expect(source).toContain('idList')
    expect(source).toContain('discover-mode')
    expect(source).not.toContain('筛选条件')
    expect(source).toContain('interaction')
    expect(source).toContain("colKey: 'source_strategy'")
    expect(source).toContain("colKey: 'source_task'")
    expect(source).toContain('updated_by_name')
    expect(source).toContain('interactionLabel')
    expect(source).toContain('result?.items')
    expect(source).toContain('discovery.importResults')
    expect(source).toContain('loadManualContext')
    expect(source).toContain('team_id: selectedTeamID()')
    expect(source).toContain('所属游戏')
    expect(source).toContain('game_id')
    expect(source).toContain('gameName')
    expect(source).not.toContain('导入链接')
    expect(source).not.toContain('进入审核模式')
  })

  it('filters by exact content ID and defaults to published_at descending', () => {
    expect(source).toContain('const contentID = ref(')
    expect(source).toContain("content_id: contentID.value.trim() || undefined")
    expect(source).toContain("const sortBy = ref('published_at')")
    expect(source).toContain("const sortOrder = ref('desc')")
    expect(source).toContain("{ colKey: 'published_at', title: '发布时间', width: 150, sorter: true }")
  })

  it('consolidates video search and fetches IDs or links synchronously', () => {
    expect(source).toContain("openManual('id')")
    expect(source).toContain("openManual('keyword')")
    expect(source).toContain('function parseSearchTargets()')
    expect(source).toContain('new Set(')
    expect(source).toContain("query: targets.join('\\n')")
    expect(source).not.toContain('discovery.importUrl')
    expect(source).not.toContain('ID 搜索任务已创建')
    expect(source).not.toContain("manualMode === 'url'")
  })

  it('hides unavailable Douyin author search and keeps keyword pagination', () => {
    expect(source).not.toContain("openManual('author')")
    expect(source).not.toContain('discovery.authorSearch')
    expect(source).toContain('manualPagination')
    expect(source).toContain('next_offset')
    expect(source).toContain('has_more')
    expect(source).toContain('searchManual(true)')
  })

  it('shows direct search results and keyword-only pagination', () => {
    expect(source).toContain('v-if="manualResults.length"')
    expect(source).toContain("manualMode === 'keyword' && manualSearched")
    expect(source).not.toContain("manualMode === 'id' && manualPagination")
  })

  it('renders synchronous search results without polling a crawl task', () => {
    expect(source).not.toContain('discovery.getTask')
    expect(source).not.toContain('waitForSearchTask')
    expect(source).not.toContain('setTimeout')
    expect(source).toContain('manualResults.value = Array.isArray(result?.items) ? result.items : []')
  })

  it('resolves and locks the assigned team for non-admin users', () => {
    expect(source).toContain('const manualTeamLocked = ref(false)')
    expect(source).toContain('const data = await users.listTeams()')
    expect(source).toContain("manualTeamLocked.value = user?.role !== 'admin'")
    expect(source).toContain(':disabled="manualTeamLocked"')
    expect(source).toContain('team.name')
  })

  // 目录仍按 scope=all 取（名称解析与人工导入要看全团队），但选项只列启用游戏：
  // 停用游戏已不能新建策略，勾进筛选就成了一个看不见的裸条件。默认负责游戏与启用集合
  // 取交集；交集为空时不传 game_ids（按既有裁定，负责游戏只是默认筛选，不是可见边界）。
  it('offers only enabled games as options while still mapping names from the full catalog', () => {
    expect(source).toContain('const gameFilter = ref([])')
    expect(source).toContain('const isAdmin = computed')
    expect(source).toContain('function defaultGameIDs()')
    expect(source).toContain('gameFilter.value = defaultGameIDs()')
    expect(source).toContain("users.listGames({ scope: 'all' })")
    expect(source).toContain("import { enabledGames } from '../../../shared/utils/games.js'")
    expect(source).toContain('const enabledGameOptions = computed(() => enabledGames(games.value))')
    expect(source).toContain('const enabledIds = new Set(enabledGameOptions.value.map((game) => game.id))')
    expect(source).toContain('return ids.filter((id) => enabledIds.has(id))')
    expect(source.match(/v-for="game in enabledGameOptions"/g)).toHaveLength(2)
    expect(source).not.toContain('v-for="game in games"')
    expect(source).toContain('const enabled = enabledGameOptions.value')
    expect(source).toContain("enabled.find((g) => g.id === 'other') || enabled[0]")
    expect(source).toContain('game_ids: gameFilter.value.length ? gameFilter.value : undefined')
    expect(source).toContain('team_id: teamFilter.value || undefined')
    expect(source).toContain('function changeTeamFilter()')
    expect(source).toContain('gameFilter.value = []')
    expect(source).toContain('v-model="gameFilter"')
    expect(source).toContain('v-model="manualGameId"')
    expect(source).toContain('v-model="teamFilter"')
    expect(source).toContain('v-if="isAdmin"')
    // 名称解析仍读全量目录，停用游戏的历史行不会显示成裸 ID。
    expect(source).toContain('const game = games.value.find((item) => item.id === id)')
  })

  it('imports only the results selected by the user', () => {
    expect(source).toContain('manualResults.value.filter')
    expect(source).toContain('items: selected')
  })

  it('keeps the operation column pinned while allowing the source columns to scroll', () => {
    expect(source).toContain("fixed: 'right'")
    expect(source).toContain('class="table-scroll-wrap"')
  })

  // 横向滚动宽度必须由列宽推导，不能写死。
  // 断言「推导」而不是某个字面量——字面量断言正是它漂移了还没人发现的原因。
  //
  // 注意别把这条测试的理由写成「写死值把内容列挤没了」：实测证伪。
  // 写死的 2060 从未生效，TDesign 取列宽合计（滚动容器 scrollWidth 实测 2378），
  // Chrome 里内容列恒 340、.title-copy 恒 224px，窗口 900→2560 都不变。
  // 内容列被压是另一回事——是 WebKit 不认 minWidth（见下方列宽那条测试）。
  it('derives the horizontal scroll width from the column widths', () => {
    expect(source).not.toContain(':scroll="{ x:')
    expect(source).toContain(':scroll="tableScroll"')
    expect(source).toContain('const tableScroll = computed')
    expect(source).toContain('col.width ?? col.minWidth ?? 0')

    // 把 columns 里声明的宽度抄出来自己算一遍。
    const start = source.indexOf('const columns = [')
    const end = source.indexOf('const tableScroll = computed')
    expect(start).toBeGreaterThan(-1)
    expect(end).toBeGreaterThan(start)
    const block = source.slice(start, end)

    const widths = [...block.matchAll(/\bwidth:\s*(\d+)/g)].map((m) => Number(m[1]))
    const total = widths.reduce((a, b) => a + b, 0)

    // 分母不为 0：一个都没抽到就说明这段断言是空转的。
    expect(widths.length).toBeGreaterThan(10)
    // 推导值必须**大于**原先写死的 2060，否则等于没修。
    expect(total).toBeGreaterThan(2060)
    // 现在每列都有 width，minWidth 不该再出现在声明里（写了也不生效，见下条测试）。
    expect(block).not.toContain('minWidth:')
  })

  // 标题有两种渲染分支：有 source_url 时是 <a>，否则是 <span>。
  // 省略号规则若只写 span，有外链的行会整段自由折行，把行高从 56px 撑到 150px+。
  it('clips the content title in both render branches without changing the link colour', () => {
    const clipRule = source.match(/\.title-copy > a,\s*\n?\.title-copy > span \{[^}]*\}/)
    expect(clipRule, 'title 省略号规则必须同时覆盖 <a> 与 <span>').not.toBeNull()
    expect(clipRule[0]).toContain('text-overflow: ellipsis')
    expect(clipRule[0]).toContain('white-space: nowrap')
    expect(clipRule[0]).toContain('overflow: hidden')
    // 颜色只在 span 分支上声明，避免覆盖 .wt-primary-link 的主题色
    expect(clipRule[0]).not.toContain('color:')
  })

  // 封面是未经代理的上游 CDN 绝对地址：可能被 CSP 拦截、防盗链、404 或换域名。
  // 这些都应退化到「暂无封面」，而不是留一个破图。
  it('degrades a failed cover to the empty-cover placeholder instead of a broken image', () => {
    expect(source).toContain('function markCoverFailed(')
    expect(source).toContain('function coverAvailable(')
    const guards = source.match(/v-if="coverAvailable\(/g) || []
    const handlers = source.match(/@error="markCoverFailed\(/g) || []
    expect(guards).toHaveLength(3)
    expect(handlers).toHaveLength(3)
    // 破图不再出现：所有封面按钮都由 coverAvailable 把关
    expect(source).not.toContain('v-if="row.cover_url"')
    expect(source).not.toContain('v-if="detail.cover_url"')
  })

  it('still links a materialised content row into the material library', () => {
    expect(source).toContain('function viewMaterial(')
    expect(source).toMatch(/router\.push\(\{ path: '\/material-library', query: \{ material_id:/)
    expect(source).toContain("material_created: '已转素材'")
  })

  it('converges row actions by processing status with 详情 as the single label', () => {
    expect(source).toContain('@click="openDetail(row)">详情</t-button>')
    expect(source).toContain(`v-if="!isLibrary && row.status === 'pending'" size="small" theme="primary" @click="materialize(row)">转素材</t-button>`)
    expect(source).toContain(`v-if="!isLibrary && row.status === 'pending'" size="small" class="wt-secondary-button" variant="outline" @click="updateStatus(row, 'ignored')">忽略</t-button>`)
    expect(source).toContain(`v-if="!isLibrary && row.status === 'ignored'" size="small" class="wt-secondary-button" variant="outline" @click="updateStatus(row, 'pending')">恢复</t-button>`)
    expect(source).toContain(`v-if="!isLibrary && row.status === 'material_created'" size="small" class="wt-secondary-button" variant="outline" @click="viewMaterial(row)">素材详情</t-button>`)
    expect(source).not.toContain('>查看</t-button>')
    expect(source).not.toContain('查看素材')
  })

  it('keeps batch actions in a separate selection toolbar below the filters', () => {
    expect(source).toContain('已选 {{ selectedRowKeys.length }} 项')
    expect(source).toContain('v-if="!isLibrary && selectedRowKeys.length" class="wt-selection-toolbar"')
    const filters = source.slice(source.indexOf('<div class="filter-bar">'), source.indexOf('<div class="table-scroll-wrap">'))
    expect(filters.indexOf('class="wt-selection-toolbar"')).toBeGreaterThan(filters.indexOf('</div>\n        <div v-if="!isLibrary'))
    expect(filters).not.toContain('<t-space wrap>')
  })

  it('renders a large content decision workspace instead of a CRUD field table', () => {
    expect(source).toContain('size="min(92vw, 960px)"')
    expect(source).toContain('class="detail-workspace"')
    expect(source).toContain('class="detail-cover"')
    expect(source).toContain('row.strategy_name')
    expect(source).toContain('row.crawl_task_name')
    expect(source).toContain('素材详情')
    expect(source).toContain("router.push(`/crawl-tasks?strategy_id=${detail.strategy_id}`)")
    expect(source).toContain("router.push(`/crawl-tasks?task_id=${detail.crawl_task_id}`)")
  })

  it('opens platform pages and displays complete interaction and author data', () => {
    expect(source).toContain('target="_blank"')
    expect(source).toContain('rel="noopener noreferrer"')
    expect(source).toContain('class="wt-primary-link"')
    expect(source).toContain('author_home_url')
    expect(source).toContain('author_sec_uid')
    expect(source).toContain('author_uid')
    expect(source).toContain('view_count')
    expect(source).toContain('comment_count')
    expect(source).toContain('share_count')
  })

  it('enlarges covers without intercepting title navigation', () => {
    expect(source).toContain('t-image-viewer')
    expect(source).toContain('function openImageViewer(url)')
    expect(source).toContain('imageViewerVisible')
    expect(source).toContain('imageViewerImages')
    expect(source).toContain('cursor: zoom-in')
    expect(source).toContain('@click.stop="openImageViewer(row.cover_url)"')
    expect(source).toContain('@click.stop="openImageViewer(detail.cover_url)"')
  })

  it('keeps review progress, failure state, and explicit exit behavior explicit', () => {
    expect(source).toContain('reviewQueue')
    expect(source).toContain('reviewIndex')
    expect(source).toContain('当前 {{ reviewIndex + 1 }} / {{ reviewQueue.length }}')
    expect(source).toContain('reviewError')
    expect(source).toContain('reviewProcessing')
    expect(source).toContain("reviewError.value = e.message || '审核操作失败，当前内容保持可处理状态'")
    expect(source).toContain("reviewAction('skip')")
    expect(source).toContain('function exitReviewMode(')
    expect(source).toContain('取消审核')
  })
})
