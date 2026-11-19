import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { expect, it, vi } from 'vitest'
import PublishingResourcePicker from './PublishingResourcePicker.vue'

vi.mock('../../../shared/ui/resource/ResourceOverlay.vue', () => ({
  default: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }),
}))
vi.mock('tdesign-vue-next', async (importOriginal) => ({
  ...await importOriginal(),
  // Exercise the picker's real row slots with the recovery row returned by files.list.
  Table: defineComponent({ setup: (_, { slots }) => () => {
    const row = { id: 1, name: '封面', deleted_at: '2026-11-19T03:00:00Z', resource_path: '', public_url: '' }
    return h('div', [slots.scope?.({ row }), slots.op?.({ row })])
  } }),
}))

it('renders an unavailable recovery image without requesting its removed preview URL', async () => {
  const html = await renderToString(createSSRApp(PublishingResourcePicker, { visible: true, kind: 'image', category: '1' }))
  expect(html).toContain('文件正在删除')
  expect(html).not.toContain('<img')
})
