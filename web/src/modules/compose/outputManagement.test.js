import { describe, expect, it } from 'vitest'
import { canQueueOutputDownload, canDownloadOutput, outputDownloadState, outputTitle, outputVideoState } from './outputManagement.js'

const task = (status, extra = {}) => ({
  asset_type: 'composite_output', asset_id: 59, execution_scope: 'local_agent',
  purpose: 'user_download', status, file_name: '成片-59.mp4', ...extra,
})

describe('output management facts', () => {
  it('uses only the fixed output name and leaves missing history unknown', () => {
    expect(outputTitle({ name: '原素材 · 原组' }, '改名素材', '新策略')).toBe('原素材 · 原组')
    expect(outputTitle({ material_id: 161 }, '现素材', '现策略')).toBe('—')
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
  it('projects verified object and device facts separately without inventing availability', () => {
    expect(outputVideoState({video:{status:'unknown'}},{key:'unverified'}).label).toBe('—')
    expect(outputVideoState({video:{status:'ready'}},{key:'unverified'}).label).toBe('可用')
    expect(outputVideoState({video:{status:'failed'}},{key:'downloaded'}).label).toBe('可用')
    expect(outputVideoState({video:{status:'ready'}},{key:'downloading'}).label).toBe('下载中')
    expect(outputVideoState({video:{status:'ready'}},{key:'failed',reason:'连接中断'})).toMatchObject({label:'失败',reason:'连接中断'})
    expect(canDownloadOutput({actions:{download:false}},{key:'not_downloaded'})).toBe(false)
    expect(canDownloadOutput({actions:{download:true}},{key:'unverified'})).toBe(false)
    expect(canDownloadOutput({actions:{download:true}},{key:'missing'})).toBe(true)
  })

})
