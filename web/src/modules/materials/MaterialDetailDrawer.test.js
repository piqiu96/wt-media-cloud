import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MaterialDetailDrawer.vue', import.meta.url), 'utf8')

// 走查修正（CHG-20260930-069）：作者主页、平台原视频页、文件大小与云端视频地址
// 都在详情里。行内只剩识别信息。
describe('material detail drawer', () => {
  it('opens the author home and the source page from the material body', () => {
    expect(source).toContain('material.author_home_url')
    expect(source).toContain('material.source_url')
  })

  it('shows the file size through the shared formatter', () => {
    expect(source).toContain('formatBytes(material.video_size_bytes)')
    expect(source).toContain("from '../../shared/utils/units.js'")
  })

  // 云端视频地址只在就绪时去取：未就绪的素材没有地址，一次必然 409 的请求
  // 不该发出去。
  it('asks for the cloud video address only when the material is ready', () => {
    expect(source).toContain('material.video_status === \'ready\'')
    expect(source).toContain('getVideoUrl(')
  })

  it('renders the cover with a placeholder when it is missing or fails to load', () => {
    expect(source).toContain('MaterialCover')
  })
})
