import { describe, expect, it } from 'vitest'
import { isRetryableTask, retryHint } from './retryPolicy.js'

describe('retry eligibility', () => {
  it('takes a cancelled task whatever its error code says', () => {
    expect(isRetryableTask({ status: 'cancelled', error_code: '' })).toBe(true)
    expect(isRetryableTask({ status: 'cancelled', error_code: 'storage_error' })).toBe(true)
  })

  // 失败原因需要修数据（素材缺失、模板停用）时，修完仍旧从这条任务重试：一条 (策略, 素材)
  // 只有一条任务，抽屉那边不会再给第二个入口。
  it('takes every failed task except the uncertain outcome', () => {
    expect(isRetryableTask({ status: 'failed', error_code: 'storage_error' })).toBe(true)
    expect(isRetryableTask({ status: 'failed', error_code: 'task_timeout' })).toBe(true)
    expect(isRetryableTask({ status: 'failed', error_code: 'execution_failed' })).toBe(true)
    expect(isRetryableTask({ status: 'failed', error_code: 'input_missing' })).toBe(true)
    expect(isRetryableTask({ status: 'failed', error_code: 'template_unavailable' })).toBe(true)
    expect(isRetryableTask({ status: 'failed', error_code: 'something_new' })).toBe(true)
  })

  // 执行结果不明确有自己的管理员核对通道：绕过核对直接重跑会盖掉待核对的现场。
  it('refuses the uncertain outcome, which has its own review path', () => {
    expect(isRetryableTask({ status: 'failed', error_code: 'execution_uncertain' })).toBe(false)
  })

  it('refuses the states that are not failures', () => {
    expect(isRetryableTask({ status: 'pending', error_code: '' })).toBe(false)
    expect(isRetryableTask({ status: 'running', error_code: '' })).toBe(false)
    expect(isRetryableTask({ status: 'success', error_code: '' })).toBe(false)
    expect(isRetryableTask(null)).toBe(false)
  })
})

describe('retry hint', () => {
  // 图三那条（素材 175 / 策略 18，storage_error）的形状：这条任务可以重试，但当前账号不是
  // 高级运营，按钮藏起来了 —— 这句话就是那个按钮缺席的解释。
  it('explains that a retryable task needs a senior role', () => {
    expect(retryHint({ status: 'failed', error_code: 'storage_error' }, false)).toBe('可重试，但当前账号无此权限（需高级运营或管理员）')
    expect(retryHint({ status: 'cancelled', error_code: '' }, false)).toBe('可重试，但当前账号无此权限（需高级运营或管理员）')
  })

  it('says nothing when the button is right there', () => {
    expect(retryHint({ status: 'failed', error_code: 'storage_error' }, true)).toBe('')
    expect(retryHint({ status: 'failed', error_code: 'input_missing' }, true)).toBe('')
    expect(retryHint({ status: 'cancelled', error_code: '' }, true)).toBe('')
  })

  // 执行结果不明确由那条专用告警解释（出路是管理员核对），这里补一句话只会把人引错方向。
  it('stays quiet for the outcome that has its own explanation', () => {
    expect(retryHint({ status: 'failed', error_code: 'execution_uncertain' }, true)).toBe('')
    expect(retryHint({ status: 'failed', error_code: 'execution_uncertain' }, false)).toBe('')
  })

  it('stays quiet when the task has no retry entry at all', () => {
    expect(retryHint({ status: 'success', error_code: '' }, true)).toBe('')
    expect(retryHint({ status: 'pending', error_code: 'storage_error' }, false)).toBe('')
    expect(retryHint(null, true)).toBe('')
  })
})
