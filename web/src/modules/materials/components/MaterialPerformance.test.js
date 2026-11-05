import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { describe, expect, it } from 'vitest'
import MaterialPerformance from './MaterialPerformance.vue'

const render = material => renderToString(createSSRApp(MaterialPerformance, { material }))
describe('material performance', () => {
  it('shows likes, favorites and comments, followed by the measured update time', async () => {
    const html = await render({ like_count: 1234, favorite_count: 56, comment_count: 7, metrics_updated_at: '2026-11-05T10:00:00Z' })
    for (const text of ['点赞', '收藏', '评论', '1,234', '56', '7', '数据更新时间', '2026']) expect(html).toContain(text)
    expect(html).not.toContain('浏览')
    expect(html.indexOf('数据更新时间')).toBeGreaterThan(html.indexOf('评论'))
  })
  it('keeps measured zero and unknown metrics distinct and never invents a metric update time', async () => {
    const html = await render({ like_count: 0, favorite_count: 0, comment_count: null, metric_presence: { favorite_count: false }, updated_at: '2026-11-05T10:00:00Z' })
    expect(html).toContain('>0</strong>')
    expect(html.match(/>—<\/strong>/g)).toHaveLength(2)
    expect(html).toContain('数据更新时间：—')
    expect(html).not.toContain('2026')
  })
})
