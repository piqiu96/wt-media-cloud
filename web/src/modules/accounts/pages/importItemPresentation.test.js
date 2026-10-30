import { describe, expect, it } from 'vitest'
import { importItemActions, importItemPresentation } from './importItemPresentation.js'
import { importResultSummary } from './importResultSummary.js'

const keys = (item, options) => importItemActions(item, options).map(action => action.key)
describe('import result operations', () => {
  it('directs duplicate identities to existing accounts without retrying or opening the redundant window', () => {
    const item = { status: 'failed', existing_account_id: '42', bit_profile_id: 'redundant', message: '已恢复' }
    expect(keys(item)).toEqual(['view', 'skip'])
    expect(importItemPresentation(item)).toMatchObject({ label: '已存在', tone: 'warning' })
    expect(importItemPresentation(item).message).toContain('无需重复接入')
    expect(importResultSummary({ total: 1, failed: 1 }, false, [item])).toMatchObject({ title: '本批账号已存在', tone: 'warning' })
  })
  it('guides manual login through open and inspection instead of replaying the Cookie', () => {
    const item = { status: 'failed', step: 'manual', bit_profile_id: 'profile' }
    expect(keys(item)).toEqual(['open', 'reconcile', 'skip'])
    expect(importItemActions(item)[1].label).toBe('检查并同步')
    expect(importItemPresentation(item)).toMatchObject({ label: '待人工登录', tone: 'warning' })
  })
  it('requires reconciliation for historical running or uncertain rows and disables new writes while blocked', () => {
    for (const status of ['running', 'uncertain']) {
      const item = { status, bit_profile_id: 'profile' }
      expect(keys(item)).toEqual(['reconcile', 'open'])
      expect(importItemPresentation(item).label).toBe('待确认')
    }
    const actions = importItemActions({ status: 'failed', creation_unavailable: true }, { blocked: true })
    expect(actions.filter(action => ['retry', 'reuse'].includes(action.key)).every(action => action.disabled)).toBe(true)
    expect(actions.find(action => action.key === 'skip').disabled).toBe(false)
  })
  it('allows returning skipped rows to processing and keeps failure reasons intelligible after restore', () => {
    expect(keys({ status: 'skipped' })).toEqual(['restore'])
    expect(importItemPresentation({ status: 'skipped', message: '等待接入' }).message).toContain('暂不接入')
    expect(importItemPresentation({ status: 'failed', message: '已恢复' }).message).toContain('请重试')
    expect(importItemPresentation({ status: 'failed', message: '读取登录信息超时' }).message).toBe('读取登录信息超时')
  })
  it('keeps all operations disabled while a request is in flight, with at most one highlighted action', () => {
    const actions = importItemActions({ status: 'failed', bit_profile_id: 'p', creation_unavailable: true }, { busy: true })
    expect(actions.every(action => action.disabled)).toBe(true)
    expect(actions.filter(action => action.primary)).toHaveLength(1)
    expect(keys({ status: 'succeeded', account_id: 'saved' })).toEqual(['view'])
    expect(keys({ status: 'pending' })).toEqual(['skip'])
  })
})
