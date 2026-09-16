import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const read = (name) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8')

describe('discovery strategy and task pages', () => {
  it('exposes keyword and author strategy configuration', () => {
    const source = read('DiscoveryStrategiesPage.vue')
    expect(source).toContain('strategy_type')
    expect(source).toContain('关键词')
    expect(source).toContain('博主')
    expect(source).toContain('立即执行')
    expect(source).toContain('编辑')
  })

  it('keeps task status and result statistics read-only', () => {
    const source = read('CrawlTasksPage.vue')
    expect(source).toContain('待执行')
    expect(source).toContain('执行中')
    expect(source).toContain('新增')
    expect(source).toContain('查看详情')
  })
})
