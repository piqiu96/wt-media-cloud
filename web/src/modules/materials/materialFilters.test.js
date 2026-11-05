import { describe, expect, it } from 'vitest'
import { publishedRangeParams, metricRangeParams, metricLabel, materialVideoState } from './materialFilters.js'

describe('material filters and video facts', () => {
  it('includes the whole final calendar day and rejects inverted or invalid dates', () => {
    const bounds = publishedRangeParams(['2026-11-01','2026-11-05'])
    expect(new Date(bounds.published_from)).toEqual(new Date(2026,10,1))
    expect(new Date(bounds.published_before)).toEqual(new Date(2026,10,6))
    expect(publishedRangeParams([])).toEqual({})
    expect(() => publishedRangeParams(['2026-11-05','2026-11-01'])).toThrow()
    expect(() => publishedRangeParams(['2026-02-30','2026-03-01'])).toThrow()
  })
  it('preserves a zero threshold and distinguishes missing metrics from real zero', () => {
    expect(metricRangeParams({like_count_min:0,like_count_max:null,favorite_count_min:''})).toEqual({like_count_min:0})
    expect(metricLabel({view_count:0},'view_count')).toBe('0')
    expect(metricLabel({view_count:0,metric_presence:{view_count:false}},'view_count')).toBe('—')
    expect(metricLabel({},'like_count')).toBe('—')
  })
  it('does not require a local download for usable cloud video, nor trust historical downloads as local presence', () => {
    expect(materialVideoState({video_status:'ready',download_status:''}).label).toBe('可用')
    expect(materialVideoState({download_status:'downloaded'}).label).toBe('—')
    expect(materialVideoState({download_status:'downloaded'}, {presence:'present_current'}).label).toBe('可用')
    expect(materialVideoState({video_status:'ready',download_status:'downloading'}).label).toBe('下载中')
    expect(materialVideoState({video_status:'ready',download_status:'failed'}).label).toBe('失败')
    expect(materialVideoState({video_status:'downloading'}).label).toBe('准备中')
  })
})
