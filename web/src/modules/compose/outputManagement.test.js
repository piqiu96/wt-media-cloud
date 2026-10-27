import { describe, expect, it } from 'vitest'
import { canQueueOutputDownload, outputDownloadState, outputTitle } from './outputManagement.js'

const task = (status, extra = {}) => ({
  asset_type: 'composite_output', asset_id: 59, execution_scope: 'local_agent',
  purpose: 'user_download', status, file_name: '成片-59.mp4', ...extra,
})

describe('output management facts', () => {
  it('uses material and strategy facts for a stable output title', () => {
    expect(outputTitle({ material_id: 161, compose_strategy_id: 34 }, '来源标题', '测试')).toBe('来源标题 · 测试')
    expect(outputTitle({ material_id: 161, compose_strategy_id: 34 }, '', '')).toBe('素材 161 · 策略 34')
  })

  it('only reports downloaded after the desktop confirms the file exists', () => {
    expect(outputDownloadState([], 59, null, false).key).toBe('not_downloaded')
    expect(outputDownloadState([task('running')], 59, null, true).key).toBe('downloading')
    expect(outputDownloadState([task('failed')], 59, null, true).key).toBe('failed')
    expect(outputDownloadState([task('success')], 59, null, false).key).toBe('unverified')
    expect(outputDownloadState([task('success')], 59, { presence: 'absent' }, true).key).toBe('missing')
    expect(outputDownloadState([task('success')], 59, { presence: 'present_current', directory: '/saved' }, true).key).toBe('downloaded')
  })

  it('keeps an existing file even if a later retry failed', () => {
    const state = outputDownloadState([task('failed'), task('success')], 59, { presence: 'present_elsewhere' }, true)
    expect(state.key).toBe('downloaded')
    expect(state.fileName).toBe('成片-59.mp4')
  })

  it('only lets actionable rows enter bulk download selection', () => {
    for (const key of ['not_downloaded', 'failed', 'missing']) expect(canQueueOutputDownload({ key })).toBe(true)
    for (const key of ['downloaded', 'downloading', 'unverified', 'unknown']) expect(canQueueOutputDownload({ key })).toBe(false)
  })
})
