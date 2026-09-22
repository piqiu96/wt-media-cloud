import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const read = (name) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8')

describe('discovery strategy and task pages', () => {
  it('exposes keyword strategy configuration while author strategies await maintenance', () => {
    const source = read('DiscoveryStrategiesPage.vue')
    expect(source).toContain('strategy_type')
    expect(source).toContain('关键词')
    expect(source).toContain('博主（维护中）')
    expect(source).toContain('value="author" disabled')
    expect(source).toContain("row.strategy_type === 'author'")
    expect(source).toContain('立即执行')
    expect(source).toContain('自动转素材')
    expect(source).toContain('value="AND"')
    expect(source).toContain('value="OR"')
    expect(source).toContain('like_threshold')
    expect(source).toContain('favorite_threshold')
    expect(source).toContain('latestSummary')
    expect(source).toContain('内容结果')
    expect(source).toContain('编辑')
  })

  it('keeps task status and result statistics read-only', () => {
    const source = read('CrawlTasksPage.vue')
    expect(source).toContain('待执行')
    expect(source).toContain('partial_success')
    expect(source).toContain('部分成功')
    expect(source).toContain('parent_task_id')
    expect(source).toContain('关联原任务')
    expect(source).toContain('重试任务已创建')
    expect(source).toContain('执行中')
    expect(source).toContain('新增')
    expect(source).toContain('查看详情')
    expect(source).toContain('策略快照')
    expect(source).toContain('自动转素材')
    expect(source).toContain('重试失败项')
    expect(source).toContain('processing_status')
    expect(source).toContain('client.retryFailed')
    expect(source).toContain('内容结果')
  })
})
