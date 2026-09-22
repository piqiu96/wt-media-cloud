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
    expect(source).toContain('筛选条件')
    expect(source).toContain('发布时间')
    expect(source).toContain('内容数量')
    expect(source).toContain('来源标签')
    expect(source).toContain('discover-mode')
    expect(source).toContain('interaction')
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
    expect(source).toContain(":scroll=\"{ x: '2060px' }\"")
    expect(source).toContain('class="table-scroll-wrap"')
  })

  it('supports the read-only material library projection without duplicating a page', () => {
    expect(source).toContain("route.path === '/material-library'")
    expect(source).toContain("status: isLibrary.value ? 'material_created'")
    expect(source).toContain('material_id: materialFilter.value || undefined')
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
