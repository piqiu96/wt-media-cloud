import { describe, expect, it, vi } from 'vitest'
import { canSelectForDownload, submitMaterialDownloads } from './myMaterialsBatch.js'

const row = (usage_id, id, download_status = '', usage_status = 'active') => ({ usage_id, id, download_status, usage_status })

describe('my materials bulk download', () => {
  it('excludes downloaded, downloading and removed relations from selection', () => {
    expect(canSelectForDownload(row(1, 11))).toBe(true)
    expect(canSelectForDownload(row(2, 12, 'failed'))).toBe(true)
    expect(canSelectForDownload(row(3, 13, 'downloaded'))).toBe(false)
    expect(canSelectForDownload(row(4, 14, 'downloading'))).toBe(false)
    expect(canSelectForDownload(row(5, 15, '', 'removed'))).toBe(false)
  })

  it('submits material ids only for eligible selected usage ids and reports partial failure', async () => {
    const createDownload = vi.fn(async (id) => { if (id === 12) throw new Error('unavailable') })
    const rows = [row(1, 11), row(2, 12, 'failed'), row(3, 13, 'downloaded'), row(4, 14, '', 'removed')]
    const result = await submitMaterialDownloads(rows, [1, 2, 3, 4, 99], createDownload)
    expect(createDownload.mock.calls).toEqual([[11], [12]])
    expect(result).toMatchObject({ total: 5, submitted: 1, skipped: 3 })
    expect(result.failures).toHaveLength(1)
    expect(result.failures[0].row.usage_id).toBe(2)
  })
})
