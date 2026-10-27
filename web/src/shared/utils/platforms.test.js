import { describe, expect, it } from 'vitest'
import {
  ACCOUNT_OPENING_PLATFORM_OPTIONS,
  ACCOUNT_PLATFORM_OPTIONS,
  MINING_PLATFORM_OPTIONS,
  PLATFORM_LABELS,
  platformLabel,
  platformOptions,
} from './platforms.js'

describe('platformLabel', () => {
  it('renders every supported code as its Chinese label', () => {
    expect(PLATFORM_LABELS).toEqual({
      douyin: '抖音',
      bilibili: 'B站',
      xhs: '小红书',
      baidu: '百度',
    })
    expect(platformLabel('douyin')).toBe('抖音')
    expect(platformLabel('bilibili')).toBe('B站')
    expect(platformLabel('xhs')).toBe('小红书')
    expect(platformLabel('baidu')).toBe('百度')
  })

  it('falls back instead of leaking an unknown code into the page', () => {
    expect(platformLabel('weibo')).toBe('-')
    expect(platformLabel('weibo', '未知平台')).toBe('未知平台')
    expect(platformLabel('')).toBe('-')
    expect(platformLabel(null)).toBe('-')
    expect(platformLabel(undefined)).toBe('-')
  })

  it('keeps the retired code out of the display map', () => {
    expect(PLATFORM_LABELS.baijiahao).toBeUndefined()
    expect(PLATFORM_LABELS.xiaohongshu).toBeUndefined()
  })
})

describe('platformOptions', () => {
  it('maps codes to option objects in the given order', () => {
    expect(platformOptions(['baidu', 'douyin'])).toEqual([
      { value: 'baidu', label: '百度' },
      { value: 'douyin', label: '抖音' },
    ])
  })

  it('scopes each capability to the platforms that actually connect', () => {
    expect(MINING_PLATFORM_OPTIONS).toEqual([{ value: 'douyin', label: '抖音' }])
    expect(ACCOUNT_PLATFORM_OPTIONS).toEqual([
      { value: 'bilibili', label: 'B站' },
      { value: 'baidu', label: '百度' },
    ])
    expect(ACCOUNT_OPENING_PLATFORM_OPTIONS).toEqual([
      { value: 'douyin', label: '抖音' },
      { value: 'bilibili', label: 'B站' },
      { value: 'baidu', label: '百度' },
    ])
  })
})
