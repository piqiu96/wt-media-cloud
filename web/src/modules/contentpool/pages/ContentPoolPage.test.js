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
