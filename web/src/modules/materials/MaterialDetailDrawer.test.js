import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MaterialDetailDrawer.vue', import.meta.url), 'utf8')

// 走查修正（CHG-20260930-069）：作者主页、平台原视频页、文件大小与云端视频地址
// 都在详情里。行内只剩识别信息。
describe('material detail drawer', () => {
  // 走查反馈：详情里的素材 ID 与列表同形，不带 # 前缀。
  it('shows the material id as a plain number', () => {
    expect(source).toContain('<dd>{{ material.id }}</dd>')
    expect(source).not.toContain('#{{ material.id }}')
  })

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

  // 走查反馈（CHG-20260930-069）：来源行采集时的内容池统计以**独立区块**展现，不混进
  // dl 的字段里——它们是采集时刻的快照，不是素材的属性。抖音接口不提供播放数
  // （statistics.play_count 恒为 0，已对全部来源行核对），区块只展示拿得到数的四项；
  // view_count 字段仍在契约里，只是不在这里渲染。
  it('shows the source content-pool statistics as their own section', () => {
    const template = source.slice(source.indexOf('<template>'))
    expect(source).toContain('material.like_count')
    expect(source).toContain('material.favorite_count')
    expect(source).toContain('material.comment_count')
    expect(source).toContain('material.share_count')
    for (const label of ['点赞', '收藏', '评论', '分享']) {
      expect(template).toContain(label)
    }
    // 断言只看模板：脚本注释里「抖音不提供播放数」是这条决定本身的记录。
    expect(template).not.toContain('播放')
    expect(template).not.toContain('material.view_count')
  })

  // 下载按钮与列表行同一形状（CHG-20260930-069 走查反馈）：只叫「下载」，
  // 下面不挂提示小字——状态徽章已经说明了点下去会发生什么。
  it('names the download action just「下载」without a hint under it', () => {
    expect(source).toContain("@click=\"$emit('download', material)\">下载</t-button>")
    expect(source).not.toContain('downloadHint')
    expect(source).not.toContain('下载到本机')
  })
})
