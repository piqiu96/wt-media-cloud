import { describe, expect, it, vi } from 'vitest'

const { success, error, confirm, destroy } = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), confirm: vi.fn(), destroy: vi.fn() }))
vi.mock('tdesign-vue-next', () => ({ MessagePlugin: { success, error, info: vi.fn() }, DialogPlugin: { confirm } }))

import { feedback } from './index.js'

describe('feedback toast', () => {
  it('shows ordinary failures in the top right and closes automatically', () => {
    feedback.toast.error('保存失败')
    expect(error).toHaveBeenCalledWith(expect.objectContaining({
      content: '保存失败', placement: 'top-right', duration: 4000,
    }))
  })

  it('preserves a success toast with follow-up actions and its duration', () => {
    const content = () => '立即下载'
    feedback.toast.success({ content, duration: 5000 })
    expect(success).toHaveBeenCalledWith(expect.objectContaining({
      content, placement: 'top-right', duration: 5000,
    }))
  })
})

describe('warning confirmation', () => {
  it('only proceeds after an explicit confirmation', async () => {
    confirm.mockReturnValue({ destroy })
    const accepted = feedback.confirmWarning({ title: '删除用户', message: '确定删除？', confirmText: '删除', danger: true })
    const dialog = confirm.mock.lastCall[0]
    expect(dialog).toEqual(expect.objectContaining({ theme: 'warning', header: '删除用户', body: '确定删除？', closeOnOverlayClick: false }))
    expect(dialog.confirmBtn).toEqual({ content: '删除', theme: 'danger' })
    dialog.onConfirm()
    expect(await accepted).toBe(true)
    expect(destroy).toHaveBeenCalledOnce()

    const canceled = feedback.confirmWarning({ title: '继续执行', message: '存在风险' })
    confirm.mock.lastCall[0].onCancel()
    expect(await canceled).toBe(false)
  })
})
