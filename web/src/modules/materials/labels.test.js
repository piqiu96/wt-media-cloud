import { describe, expect, it } from 'vitest'
import {
  DUPLICATE_RISKS,
  MATERIAL_STATUSES,
  USAGE_FIELDS,
  VIDEO_STATUSES,
  downloadActionLabel,
  duplicateRiskLabel,
  duplicateRiskTone,
  gameName,
  materialStatusLabel,
  materialStatusTone,
  shortDigest,
  usageFacts,
  usageLines,
  videoStatusLabel,
  videoStatusTone,
} from './labels.js'

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

  // 门控与提示语都被拿掉了，而不只是被绕过（CHG-20260930-069 走查反馈：状态徽章
  // 已说明一切，按钮下的小字只是噪声）。留在模块里，下一个页面就会再挂它一次，
  // 而那份代码看起来完全合理。
  it('no longer exports a readiness gate or a download hint at all', async () => {
    const labels = await import('./labels.js')
    expect(labels.canDownload).toBeUndefined()
    expect(labels.downloadHint).toBeUndefined()
  })

  // 走查三轮（交互对齐 §7.4）：主操作文案由状态驱动——失败行的下一步是「重试」，
  // 其余状态都是「下载」。放行内与详情抽屉共用，两处文案分叉就是同义词混用的起点。
  it('names the primary action from the video status', () => {
    expect(downloadActionLabel('failed')).toBe('重试')
    for (const status of ['not_downloaded', 'downloading', 'ready', 'mystery', undefined]) {
      expect(downloadActionLabel(status), String(status)).toBe('下载')
    }
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

// 走查五轮（2026-09-30 用户走查 + 设计图）：列表要同时给出两个状态维度。文件状态说
// 「源视频准备好了没有」，素材状态说「这条素材还提供给运营选用吗」，规范 §7.2 明令
// 两者不得合并——可以同时是「已暂停 + 可下载」。
//
// 取值来自规范 §16.2 的三个生命周期状态。字段名 `material.status` 是**前端先声明的
// 读取协议**：服务端还没有这个字段（第五章「素材状态和生命周期」未落，change.md §3
// 明确不做），用户裁定先落样式与协议，数据打通后再同步。
describe('material status dimension', () => {
  it('names and tones every lifecycle state the spec draws', () => {
    expect(MATERIAL_STATUSES).toEqual(['available', 'paused', 'delisted'])
    for (const status of MATERIAL_STATUSES) {
      expect(materialStatusLabel(status), status).not.toBe(status)
      expect(materialStatusLabel(status), status).not.toBe('-')
      expect(materialStatusTone(status), status).toBeTruthy()
    }
    expect(materialStatusLabel('available')).toBe('可用')
    expect(materialStatusLabel('paused')).toBe('已暂停')
    expect(materialStatusLabel('delisted')).toBe('已下架')
  })

  // 「已退役」是本项目放弃的旧词（规范 §16.2 用词说明），但它仍是一个**服务端不会
  // 发来**的值：不认识的值原样透出，同 videoStatusLabel 的处理，别把它吞成 `-`。
  it('passes through a state it does not know instead of hiding it', () => {
    expect(materialStatusLabel('mystery')).toBe('mystery')
  })

  // 读不到状态**不等于**可用。退回一个默认状态就是在替服务端做它没做的判断，
  // 而「可用」是这三态里唯一能触发「加入我的素材」的那一个。
  it('never guesses a lifecycle state from a missing field', () => {
    expect(materialStatusLabel(undefined)).toBe('-')
    expect(materialStatusLabel('')).toBe('-')
    expect(materialStatusTone(undefined)).toBe('neutral')
  })
})

// 使用情况：列表「使用情况」列与详情「使用情况」卡共读同一份协议。
describe('usage protocol', () => {
  it('declares the fields the list and the detail both read', () => {
    expect(USAGE_FIELDS).toEqual([
      'clip_count', 'published_count', 'last_produced_at', 'last_published_at', 'duplicate_risk',
    ])
    expect(DUPLICATE_RISKS).toEqual(['normal', 'suspected'])
  })

  it('renders the two list lines the design draws', () => {
    const row = { usage: { clip_count: 12, published_count: 8, last_published_at: '2025-09-20T05:17:00Z' } }
    expect(usageLines(row).counts).toBe('成片 12 · 发布 8')
    expect(usageLines(row).recent).toBe('最近发布 2025/9/20')
  })

  // 0 是数据，缺省不是：0 说的是「一条成片都没生产」，缺省说的是「这个字段还没来」。
  // 把两者画成同一个样子，运营就分不清「没做过」和「还没统计」。
  it('keeps 0 a number and only an absent value a dash', () => {
    const zero = { usage: { clip_count: 0, published_count: 0 } }
    expect(usageLines(zero).counts).toBe('成片 0 · 发布 0')
    expect(usageLines(zero).recent).toBe('最近发布 -')
  })

  it('stands up the skeleton rather than a guess while the field does not exist yet', () => {
    expect(usageLines({}).counts).toBe('成片 - · 发布 -')
    expect(usageLines(undefined).counts).toBe('成片 - · 发布 -')
    expect(usageLines({}).recent).toBe('最近发布 -')
  })

  it('lays the five facts of the detail card in the designed order', () => {
    const facts = usageFacts({
      usage: {
        clip_count: 7213,
        published_count: 8,
        last_produced_at: '2025-09-18T05:00:00Z',
        last_published_at: '2025-09-20T05:17:00Z',
        duplicate_risk: 'normal',
      },
    })
    expect(facts.map((fact) => fact.label)).toEqual(['已生产成片', '已发布', '最近生产', '最近发布', '重复风险'])
    expect(facts.map((fact) => fact.value)).toEqual(['7,213', '8', '2025/9/18', '2025/9/20', '正常'])
    // 四个数量/时间事实是普通文字，只有重复风险是一颗有色的徽章。
    expect(facts[4].tone).toBe('success')
    expect(facts.slice(0, 4).every((fact) => fact.tone === undefined)).toBe(true)
  })

  it('does not paint a risk badge green before the server says it is normal', () => {
    expect(duplicateRiskLabel(undefined)).toBe('-')
    expect(duplicateRiskLabel('mystery')).toBe('mystery')
    expect(duplicateRiskTone(undefined)).toBe('neutral')
    expect(duplicateRiskTone('suspected')).toBe('danger')
  })
})
