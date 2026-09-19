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
    expect(source).toContain('内容 ID')
    expect(source).toContain('关键词搜索')
    expect(source).toContain('博主搜索')
    expect(source).toContain('result?.items')
    expect(source).toContain('discovery.importResults')
  })

  it('aligns author search and pagination with the Douyin provider contracts', () => {
    expect(source).toContain('sec_uid')
    expect(source).toContain('manualPagination')
    expect(source).toContain('next_offset')
    expect(source).toContain('max_cursor')
    expect(source).toContain('has_more')
    expect(source).toContain('searchManual(true)')
    expect(source).toContain('max_cursor: manualPagination.value.maxCursor || 0')
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
    expect(source).toContain(":scroll=\"{ x: '1100px' }\"")
    expect(source).toContain('class="table-scroll-wrap"')
  })

  it('supports the read-only material library projection without duplicating a page', () => {
    expect(source).toContain("route.path === '/material-library'")
    expect(source).toContain("status: isLibrary.value ? 'material_created'")
  })
})
