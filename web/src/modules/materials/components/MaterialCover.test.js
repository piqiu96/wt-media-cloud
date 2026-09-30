import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MaterialCover.vue', import.meta.url), 'utf8')

// 无封面或加载失败都必须显示占位，而不是一个裂图图标（CHG-20260930-069 验收线）。
describe('material cover', () => {
  it('falls back to a placeholder when the image fails to load', () => {
    expect(source).toContain('@error')
    expect(source).toContain('failed')
  })

  it('shows the placeholder when there is no cover url at all', () => {
    expect(source).toMatch(/v-if="url && !failed"/)
  })

  // 换了一条素材（换 url）后必须重置失败标记，否则上一张裂图的占位会盖住好图。
  it('resets the failure mark when the url changes', () => {
    expect(source).toContain("watch(() => props.url")
  })
})
