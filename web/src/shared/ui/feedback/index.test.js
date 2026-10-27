import { beforeEach, describe, expect, it, vi } from 'vitest'

const { success, error, confirm, alert, destroy } = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), confirm: vi.fn(), alert: vi.fn(), destroy: vi.fn() }))
vi.mock('tdesign-vue-next', () => ({ MessagePlugin: { success, error, info: vi.fn() }, DialogPlugin: { confirm, alert } }))

beforeEach(() => { vi.clearAllMocks() })

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

describe('notice', () => {
  // 单按钮告知：没有取消按钮，确认文案默认「知道了」；点确认或右上角关闭都只收口一次。
  it('acknowledges with one button and closes once', async () => {
    alert.mockReturnValue({ destroy })
    const done = feedback.notice({ title: '已转入素材库', message: '素材 12 已转入团队「甲」的素材库' })
    const dialog = alert.mock.lastCall[0]
    expect(dialog).toEqual(expect.objectContaining({
      header: '已转入素材库', body: '素材 12 已转入团队「甲」的素材库', closeOnOverlayClick: false,
    }))
    expect(dialog.confirmBtn).toEqual({ content: '知道了', theme: 'primary' })
    expect(dialog.cancelBtn).toBeUndefined()

    dialog.onConfirm()
    expect(await done).toBe(true)
    dialog.onClose()
    expect(destroy).toHaveBeenCalledOnce()
  })

  it('uses the caller confirm text', () => {
    alert.mockReturnValue({ destroy })
    feedback.notice({ title: '提示', message: '完成', confirmText: '好的' })
    expect(alert.mock.lastCall[0].confirmBtn).toEqual({ content: '好的', theme: 'primary' })
  })
})
