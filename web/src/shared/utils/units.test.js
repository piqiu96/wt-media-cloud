import { describe, expect, it } from 'vitest'
import { formatByteRate, formatBytes, formatEta } from './units.js'

describe('formatBytes', () => {
  // 未知不是零：0 在传输里是「还没量过」，显示成 `0 B` 会让它看起来像「空文件」。
  it('renders an unknown size as a dash rather than zero bytes', () => {
    expect(formatBytes(0)).toBe('-')
    expect(formatBytes(-1)).toBe('-')
    expect(formatBytes(undefined)).toBe('-')
    expect(formatBytes(null)).toBe('-')
    expect(formatBytes('不是数')).toBe('-')
  })

  it('scales through the units', () => {
    expect(formatBytes(1)).toBe('1 B')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1023)).toBe('1023 B')
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB')
    expect(formatBytes(3 * 1024 * 1024 * 1024)).toBe('3.0 GB')
    expect(formatBytes(2 * 1024 ** 4)).toBe('2.0 TB')
    // 最大的单位是 TB：再大也不换下一个名字（没有 PB 这一档就说 TB）。
    expect(formatBytes(5 * 1024 ** 5)).toBe('5120.0 TB')
  })
})

describe('formatByteRate', () => {
  it('is empty when there is no rate instead of a dash with a unit', () => {
    expect(formatByteRate(0)).toBe('')
    expect(formatByteRate(null)).toBe('')
    expect(formatByteRate(2048)).toBe('2.0 KB/s')
  })
})

describe('formatEta', () => {
  it('stays empty while the time is unknown', () => {
    expect(formatEta(0)).toBe('')
    expect(formatEta(-5)).toBe('')
    expect(formatEta(undefined)).toBe('')
  })

  it('rounds up so a remaining second is never reported as none', () => {
    expect(formatEta(1)).toBe('约 1 秒')
    expect(formatEta(59.2)).toBe('约 60 秒')
    expect(formatEta(60)).toBe('约 1 分钟')
    expect(formatEta(3600)).toBe('约 1.0 小时')
  })
})
