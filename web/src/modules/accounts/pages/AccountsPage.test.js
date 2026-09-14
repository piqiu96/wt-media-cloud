import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./AccountsPage.vue', import.meta.url), 'utf8')

describe('media account resource page', () => {
  it('uses shared resource layout and preserves account operations', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceCard')
    expect(source).toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('wt-resource-table')
    expect(source).toContain('resourceAccountStats')
    expect(source).toContain('@click="openCreate"')
    expect(source).toContain('@click="checkFromRow(row)"')
    expect(source).toContain('@click="toggleBizStatus(row)"')
  })

  it('uses a compact account action column without a separate cookie column', () => {
    expect(source).toContain('<t-dropdown')
    expect(source).not.toContain('{ colKey: "cookie", title: "Cookie"')
    expect(source).toContain('@click="openCookieDialog(row)"')
    expect(source).toContain('@click="openEdit(row)"')
  })
})
