import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ContentPoolPage.vue', import.meta.url), 'utf8')

describe('content pool page', () => {
  it('uses the shared resource layout and exposes the M3-A actions', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceStatGrid')
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
    expect(source).toContain("colKey: 'source'")
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

  // 这里原先有一条「素材库只读投影」的用例，钉的是 `/material-library` 由内容池页按
  // `route.path` 自己渲染成投影的三条字面量。**故意删除**：那个地址现在由
  // `modules/materials/pages/MaterialLibraryPage.vue` 承担（两张路由表都已改指），
  // 它钉的是一个不再发生的渲染。
  //
  // 它守的行为契约没有消失，只是搬到了实现它的地方：
  // `MaterialLibraryPage.test.js` 钉 `?material_id=` 深链落到单条接口。
  // 内容池自己那条「已转素材」的状态仍在，跳转也仍在 —— 下面这条把它钉住。
  it('still links a materialised content row into the material library', () => {
    expect(source).toContain('function viewMaterial(')
    expect(source).toMatch(/router\.push\(\{ path: '\/material-library', query: \{ material_id:/)
    // 「已转素材」仍是内容池自己的状态，列定义与徽章都还认它。
    expect(source).toContain("material_created: '已转素材'")
  })

  it('renders a large content decision workspace instead of a CRUD field table', () => {
    expect(source).toContain('size="min(72vw, 1200px)"')
    expect(source).toContain('class="detail-workspace"')
    expect(source).toContain('class="detail-cover"')
    expect(source).toContain('row.strategy_name')
    expect(source).toContain('row.crawl_task_name')
    expect(source).toContain('查看素材')
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
