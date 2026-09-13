import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ProxyPage.vue', import.meta.url), 'utf8')

describe('proxy resource page', () => {
  it('uses shared resource layout and preserves proxy operations', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceCard')
    expect(source).toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('wt-resource-table')
    expect(source).toContain('proxyStatItems')
    expect(source).toContain('@click="openCreate"')
    expect(source).toContain('@click="deleteProxy(row)"')
    expect(source).toContain('@click="triggerCheck(row)"')
  })
})
