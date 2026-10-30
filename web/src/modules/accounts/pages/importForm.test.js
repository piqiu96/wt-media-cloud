import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'
const source = readFileSync(new URL('./AccountImportDialog.vue', import.meta.url), 'utf8')
function command(name) {
  const match = source.match(new RegExp(`(?:async )?function ${name}\\([^]*?\\n}`))
  if (!match) throw new Error(`Missing import command: ${name}`)
  return match[0]
}
function context(confirm = async () => false) {
  const values = { raw: '', platform: 'bilibili', groupId: 'default', categoryType: '', categoryIds: [], tags: [], remark: '', preview: [], requestKey: '', busy: false, keepRunning: false, pauseClose: false, screen: 'input' }
  const state = Object.fromEntries(Object.entries(values).map(([k, value]) => [k, { value }]))
  state.feedback = { confirmWarning: vi.fn(confirm) }; state.emit = vi.fn()
  const commands = new Function(...Object.keys(state), `let defaultGroupId = 'default'; ${['resetInput', 'inputHasChanges', 'close'].map(command).join('\n')}; return { resetInput, inputHasChanges, close }`)(...Object.values(state))
  return { ...state, ...commands }
}
describe('import input lifecycle', () => {
  it('a fresh import resets all abandoned metadata, validation and request identity', () => {
    const c = context()
    c.platform.value = 'baidu'; c.groupId.value = 'other'; c.categoryType.value = 'game'; c.categoryIds.value = ['1']; c.tags.value = ['矩阵']; c.remark.value = '上次备注'; c.raw.value = 'synthetic'; c.requestKey.value = 'old'; c.preview.value = [{ valid: true }]
    c.resetInput()
    expect(c.platform.value).toBe('bilibili'); expect(c.groupId.value).toBe('')
    expect(c.categoryType.value).toBe(''); expect(c.categoryIds.value).toEqual([])
    expect(c.tags.value).toEqual([]); expect(c.remark.value).toBe(''); expect(c.raw.value).toBe('')
    expect(c.requestKey.value).toBe(''); expect(c.preview.value).toEqual([])
    expect(c.inputHasChanges()).toBe(false)
  })
  it('asks about metadata-only drafts and retains them when discard is cancelled', async () => {
    const c = context(); c.tags.value = ['待接入']; await c.close()
    expect(c.feedback.confirmWarning).toHaveBeenCalledOnce()
    expect(c.emit).not.toHaveBeenCalled(); expect(c.tags.value).toEqual(['待接入'])
    c.feedback.confirmWarning.mockResolvedValueOnce(true); await c.close()
    expect(c.emit).toHaveBeenCalledWith('update:visible', false)
  })
  it('closes untouched input or saved results without asking about old batch metadata', async () => {
    const c = context(); await c.close()
    c.screen.value = 'results'; c.tags.value = ['已保存']; await c.close()
    expect(c.feedback.confirmWarning).not.toHaveBeenCalled()
    expect(c.emit).toHaveBeenCalledTimes(2)
  })
  it('blocks close while validating; execution pauses only after the current item', async () => {
    const c = context(); c.busy.value = true; await c.close()
    expect(c.emit).not.toHaveBeenCalled(); expect(c.pauseClose.value).toBe(false)
    c.screen.value = 'results'; c.keepRunning.value = true; await c.close()
    expect(c.emit).not.toHaveBeenCalled(); expect(c.pauseClose.value).toBe(true); expect(c.keepRunning.value).toBe(false)
  })
})
