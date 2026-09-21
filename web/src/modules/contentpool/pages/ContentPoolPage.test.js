import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ContentPoolPage.vue', import.meta.url), 'utf8')

describe('content pool page', () => {
  it('uses the shared resource layout and exposes the M3-A actions', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('导入链接')
    expect(source).toContain('转素材')
    expect(source).toContain('批量转素材')
    expect(source).toContain('进入审核模式')
    expect(source).toContain('转素材并下一条')
    expect(source).toContain('忽略并下一条')
    expect(source).toContain('client.batchMaterialize')
    expect(source).toContain('strategy_id')
    expect(source).toContain('crawl_task_id')
    expect(source).toContain('内容 ID')
    expect(source).toContain('关键词搜索')
    expect(source).toContain('result?.items')
    expect(source).toContain('discovery.importResults')
    expect(source).toContain('loadManualContext')
    expect(source).toContain('team_id: selectedTeamID()')
  })

  it('hides unavailable Douyin author search and keeps keyword pagination', () => {
    expect(source).not.toContain("openManual('author')")
    expect(source).not.toContain('discovery.authorSearch')
    expect(source).toContain('manualPagination')
    expect(source).toContain('next_offset')
    expect(source).toContain('has_more')
    expect(source).toContain('searchManual(true)')
  })

  it('renders synchronous search results without polling a crawl task', () => {
    expect(source).not.toContain('discovery.getTask')
    expect(source).not.toContain('waitForSearchTask')
    expect(source).not.toContain('setTimeout')
    expect(source).toContain('manualResults.value = Array.isArray(result?.items) ? result.items : []')
  })

  it('imports only the results selected by the user', () => {
    expect(source).toContain('manualResults.value.filter')
    expect(source).toContain('items: selected')
  })

  it('keeps the operation column pinned while allowing the source columns to scroll', () => {
    expect(source).toContain("fixed: 'right'")
    expect(source).toContain(":scroll=\"{ x: '1500px' }\"")
    expect(source).toContain('class="table-scroll-wrap"')
  })

  it('supports the read-only material library projection without duplicating a page', () => {
    expect(source).toContain("route.path === '/material-library'")
    expect(source).toContain("status: isLibrary.value ? 'material_created'")
  })
})
