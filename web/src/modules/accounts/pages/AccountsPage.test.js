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
})
