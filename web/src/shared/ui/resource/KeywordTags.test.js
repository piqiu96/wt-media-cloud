import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it, vi } from 'vitest'
import KeywordTags from './KeywordTags.vue'

const input = vi.hoisted(() => ({ paste: null }))
vi.mock('tdesign-vue-next', async (original) => ({
  ...await original(),
  Input: {
    props: ['onPaste'],
    setup(props) { input.paste = props.onPaste; return () => null },
  },
}))

describe('keyword tags clipboard input', () => {
  it('handles the TDesign paste context and merges multiline entries without duplicates', async () => {
    const update = vi.fn()
    await renderToString(createSSRApp({ render: () => h(KeywordTags, { modelValue: ['已有'], max: 3, 'onUpdate:modelValue': update }) }))
    const preventDefault = vi.fn()
    input.paste({ e: { preventDefault }, pasteValue: '已有\n新词，另一个;超额' })
    expect(preventDefault).toHaveBeenCalledOnce()
    expect(update).toHaveBeenCalledWith(['已有', '新词', '另一个'])
  })
})
