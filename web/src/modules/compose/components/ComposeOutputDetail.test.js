import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it } from 'vitest'
import ComposeOutputDetail from './ComposeOutputDetail.vue'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('./ComposeOutputDetail.vue', import.meta.url), 'utf8')

// 这个组件同时伺候两个宿主（我的成片页与合成任务页的关联抽屉），共用同一段正文。
// 字符串断言看不见「同一段正文在两个宿主下渲染出什么」，所以这里真渲染一遍 ——
// 本仓第一处渲染断言，只为这条共享边界：编号一律裸数字，来源追踪里没有 `#`。
const detail = {
  output: { id: 42, publish_status: 'pending_publish', file_size_bytes: 2048, sha256: 'cafe', created_at: '2026-10-15T10:00:00Z', media: { duration_ms: 12500 } },
  trace: {
    material: { id: 157, title: '素材标题' },
    material_usage: { id: 9, status: 'active' },
    strategy: { id: 24, name: '策略名', template_code: 'insert', template_version: 2 },
    compose_task: { id: 252, status: 'success', created_at: '2026-10-15T10:00:00Z', config_snapshot: { insert_seconds: 3, guide_video_url: 'https://signed.example/x.mp4' } },
  },
}
const templates = [{ code: 'insert', version: 2, schema: { properties: {
  insert_seconds: { title: '插入位置（秒）', type: 'integer' },
  guide_video_url: { title: '引导视频下载地址', type: 'string' },
} } }]

async function render(props) {
  const app = createSSRApp({ render: () => h(ComposeOutputDetail, props) })
  app.component('t-button', { setup: (_, { slots }) => () => h('button', slots.default?.()) })
  return renderToString(app)
}

describe('compose output detail', () => {
  it('renders the source-tracking ids bare, with no hash prefix anywhere', async () => {
    const html = await render({ detail, tab: 'source' })
    for (const id of ['157', '24', '252', '9']) expect(html).toContain(id)
    expect(html).not.toMatch(/#\d/)
    for (const label of ['基本信息', '来源追踪', '配置快照']) expect(html).toContain(label)
  })

  it('renders only the panels the host allows and keeps signed URLs out of readable snapshots', async () => {
    const html = await render({ detail, tab: 'source', tabs: ['basic', 'source'] })
    expect(html).toContain('来源追踪')
    expect(html).not.toContain('配置快照')
    expect(html).not.toContain('guide_video_url')
    const full = await render({ detail, templates, tab: 'snapshot' })
    expect(full).toContain('插入位置（秒）')
    expect(full).toContain('3 秒')
    expect(full).not.toContain('guide_video_url')
    expect(full).not.toContain('signed.example')
  })

  it('renders the management view in asset order without production internals', async () => {
    const html = await render({
      detail,
      management: true,
      title: '素材标题 · 策略名',
      triggerType: 'manual',
      downloadState: { key: 'unverified', label: '下载记录完成 · 本机未核验', tone: 'neutral' },
      linkable: true,
    })
    for (const label of ['核心信息', '标题', '素材标题 · 策略名', '人工', '生成来源', '文件信息', '查看素材详情', '查看策略详情', '查看任务详情']) {
      expect(html).toContain(label)
    }
    expect(html.indexOf('核心信息')).toBeLessThan(html.indexOf('生成来源'))
    expect(html.indexOf('生成来源')).toBeLessThan(html.indexOf('文件信息'))
    expect(html).not.toContain('素材使用')
    expect(html).not.toContain('配置快照')
    expect(html).not.toContain('待发布')
    const core = html.slice(html.indexOf('核心信息'), html.indexOf('生成来源'))
    const source = html.slice(html.indexOf('生成来源'), html.indexOf('文件信息'))
    const file = html.slice(html.indexOf('文件信息'))
    expect(core).not.toContain('文件大小')
    expect(core).not.toContain('下载状态')
    expect(file).toContain('文件大小')
    expect(source).not.toContain('insert v2')
    expect(source).not.toContain('成功 ·')
    expect(source).not.toContain('任务创建')
  })

  it('uses the confirmed local file name as a link to its folder', async () => {
    const downloaded = await render({ detail, management: true, downloadState: { key: 'downloaded', label: '已下载', tone: 'success', fileName: '成片-42.mp4' } })
    const unverified = await render({ detail, management: true, downloadState: { key: 'unverified', label: '本机未核验', tone: 'neutral' } })
    expect(downloaded.indexOf('成片-42.mp4')).toBeGreaterThan(downloaded.indexOf('文件信息'))
    expect(downloaded).not.toContain('>打开文件夹</button>')
    expect(unverified).not.toContain('成片-42.mp4')
    expect(downloaded).toContain('文件校验信息')
    expect(downloaded).toContain('SHA-256 · cafe')
    expect(downloaded).not.toContain('<details')
    expect(source).toContain('<t-link class="output-detail__file-link" theme="primary" title="打开文件所在文件夹" @click="emit(\'open-folder\')">{{ downloadState.fileName }}</t-link>')
    expect(source).not.toContain('>打开文件夹</t-button>')
  })

  it('opens the cloud video from file information in a new browser window', async () => {
    const url = 'https://data.bucket.oss.longyanyue.cn/wt_media/dev/compose-outputs/42/1/output.mp4'
    const html = await render({ detail, management: true, previewUrl: url })
    const file = html.slice(html.indexOf('文件信息'))
    expect(file).toContain('云端视频')
    expect(file).toContain(`href="${url}"`)
    expect(file).toContain('target="_blank"')
    expect(file).toContain('rel="noopener noreferrer"')
    expect(file).toContain('打开云端视频')

    const unavailable = await render({ detail, management: true })
    expect(unavailable.slice(unavailable.indexOf('文件信息'))).toContain('暂不可用')
    expect(unavailable).not.toContain('打开云端视频')
  })
})
