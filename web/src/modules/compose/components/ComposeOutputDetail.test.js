import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it } from 'vitest'
import ComposeOutputDetail from './ComposeOutputDetail.vue'

// 这个组件同时伺候两个宿主（我的成片页与合成任务页的关联抽屉），共用同一段正文。
// 字符串断言看不见「同一段正文在两个宿主下渲染出什么」，所以这里真渲染一遍 ——
// 本仓第一处渲染断言，只为这条共享边界：编号一律裸数字，来源追踪里没有 `#`。
const detail = {
  output: { id: 42, file_size_bytes: 2048, sha256: 'cafe', created_at: '2026-10-15T10:00:00Z', media: { duration_ms: 12500 } },
  trace: {
    material: { id: 157, title: '素材标题' },
    material_usage: { id: 9, status: 'active' },
    strategy: { id: 24, name: '策略名', template_code: 'insert', template_version: 2 },
    compose_task: { id: 252, status: 'success', created_at: '2026-10-15T10:00:00Z', config_snapshot: { insert_seconds: 3, guide_video_url: 'https://signed.example/x.mp4' } },
  },
}

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

  // 面板白名单：任务详情里的成片抽屉不开「配置快照」——那一页会露出带签名的引导视频地址。
  // 末一行是阳性对照：同一份 detail 在白名单放开时**确实**会把它渲出来，所以上面那条
  // 「没有」不是空跑。
  it('renders only the panels the host allows', async () => {
    const html = await render({ detail, tab: 'source', tabs: ['basic', 'source'] })
    expect(html).toContain('来源追踪')
    expect(html).not.toContain('配置快照')
    expect(html).not.toContain('guide_video_url')
    const full = await render({ detail, tab: 'snapshot' })
    expect(full).toContain('guide_video_url')
  })
})
