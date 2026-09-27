import { describe, expect, it } from 'vitest'
import { VIDEO_STATUSES, canDownload, gameName, shortDigest, videoStatusLabel, videoStatusTone } from './labels.js'

describe('material labels', () => {
  // 四种状态全覆盖：漏掉一种就是页面上出现空白徽章，而空白看起来像「没问题」。
  it('names and tones every status in the frozen enum', () => {
    expect(VIDEO_STATUSES).toEqual(['not_downloaded', 'downloading', 'ready', 'failed'])
    for (const status of VIDEO_STATUSES) {
      expect(videoStatusLabel(status), status).not.toBe(status)
      expect(videoStatusLabel(status), status).not.toBe('-')
      expect(videoStatusTone(status), status).toBeTruthy()
    }
    expect(videoStatusLabel('ready')).toBe('可下载')
    expect(videoStatusTone('failed')).toBe('danger')
  })

  it('passes through a status it does not know instead of hiding it', () => {
    expect(videoStatusLabel('mystery')).toBe('mystery')
    expect(videoStatusLabel('')).toBe('-')
  })

  it('treats only a ready material as downloadable', () => {
    expect(canDownload({ video_status: 'ready' })).toBe(true)
    for (const status of ['not_downloaded', 'downloading', 'failed']) {
      expect(canDownload({ video_status: status }), status).toBe(false)
    }
    expect(canDownload(null)).toBe(false)
  })

  // 素材带了 game_id 而游戏表里没有它时，显示 id 而不是「-」：
  // 「不知道叫什么」和「没有游戏」是两件事。
  it('falls back to the game id rather than claiming there is no game', () => {
    expect(gameName([{ id: 'naruto', name: '火影忍者' }], 'naruto')).toBe('火影忍者')
    expect(gameName([{ id: 'naruto', name: '火影忍者' }], 'other')).toBe('other')
    expect(gameName([], '')).toBe('-')
  })

  // 体积的格式化搬去了 shared/utils/units.js —— 它不是素材的概念，下载中心也要用，
  // 两个地方各写一份正是「三份各写各的日期格式化」那条老路。用例随之搬走。

  it('shortens a digest without inventing one', () => {
    const digest = 'a'.repeat(64)
    expect(shortDigest(digest)).toBe(`${'a'.repeat(16)}…`)
    expect(shortDigest('')).toBe('-')
  })
})
