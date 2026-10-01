import { describe, expect, it } from 'vitest'
import {
  DOWNLOAD_STATUSES,
  DUPLICATE_RISKS,
  MATERIAL_STATUSES,
  USAGE_FIELDS,
  USAGE_STATUSES,
  VIDEO_STATUSES,
  downloadActionLabel,
  downloadStatusLabel,
  downloadStatusTone,
  duplicateRiskLabel,
  duplicateRiskTone,
  gameName,
  materialStatusLabel,
  materialStatusTone,
  nextStepHint,
  shortDigest,
  usageFacts,
  usageLines,
  usageStatusLabel,
  usageStatusTone,
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

  // 下载状态是**按用户派生**的状态组（MaterialUsage.download_status），四档不多不少，
  // 与冻结的 video_status 分开：那个 enum 里没有这些值，页面上出现它们就是替服务端发明。
  // 空串是矩阵里真实的一格「从未下载过」（CHG-069 任务 23），不是「缺值兜底」——它
  // 有自己的一行按钮与一档文案，不再回落 video_status。
  it('names and tones the four download-lifecycle statuses without touching video_status', () => {
    expect(DOWNLOAD_STATUSES).toEqual(['', 'downloading', 'downloaded', 'failed'])
    for (const status of DOWNLOAD_STATUSES) {
      expect(downloadStatusLabel(status), JSON.stringify(status)).not.toBe('-')
      expect(downloadStatusTone(status), JSON.stringify(status)).toBeTruthy()
    }
    expect(downloadStatusLabel('')).toBe('未下载')
    expect(downloadStatusLabel('downloading')).toBe('下载中')
    expect(downloadStatusLabel('downloaded')).toBe('已下载')
    expect(downloadStatusLabel('failed')).toBe('下载失败')
    expect(downloadStatusTone('')).toBe('neutral')
    expect(downloadStatusTone('downloading')).toBe('info')
    expect(downloadStatusTone('downloaded')).toBe('success')
    expect(downloadStatusTone('failed')).toBe('danger')
    // 不并入冻结数组：VIDEO_STATUSES 仍是那四个云侧取值。
    expect(VIDEO_STATUSES).toEqual(['not_downloaded', 'downloading', 'ready', 'failed'])
  })

  // 拿掉了而不只是绕过：留在模块里，下一个页面就会再挂它一次。
  it('no longer exports a readiness gate or a download hint at all', async () => {
    const labels = await import('./labels.js')
    expect(labels.canDownload).toBeUndefined()
    expect(labels.downloadHint).toBeUndefined()
  })

  // 主操作文案由下载状态驱动（规范 §7.4：异常状态给出「状态 + 下一步恢复动作」）。
  // 失败行的下一步是「重新下载」，其余（未下载）是「下载」——服务端 CreateDownload
  // 对未下载/失败都会先建准备任务再下，按钮行为不变，只是把「失败后再次执行」说出来。
  it('names the primary action from the download status', () => {
    expect(downloadActionLabel('failed')).toBe('重新下载')
    for (const status of ['', 'not_downloaded', 'downloading', 'ready', 'downloaded', 'mystery', undefined]) {
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

  // 体积格式化在 shared/utils/units.js，用例随之搬走。

  it('shortens a digest without inventing one', () => {
    const digest = 'a'.repeat(64)
    expect(shortDigest(digest)).toBe(`${'a'.repeat(16)}…`)
    expect(shortDigest('')).toBe('-')
  })
})

// 第二个状态维度。文件状态说「源视频准备好了没有」，素材状态说「这条素材还提供给运营选用吗」，
// 规范 §7.2 明令不得合并——可以同时是「已暂停 + 可下载」。取值来自规范 §16.2 的三个生命周期状态。
//
// 下面「未知/缺值」的断言守的是协议与实现各错一半：服务端多发一个词、或某个读取路径漏了这列，
// 都不该被静默吞成一个看着正常的默认值。
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

  // 不认识的值原样透出（同 videoStatusLabel）：吞成 `-` 之后页面看着正常，
  // 而运营永远看不到那个多出来的取值。
  it('passes through a state it does not know instead of hiding it', () => {
    expect(materialStatusLabel('mystery')).toBe('mystery')
  })

  // 读不到状态不等于可用：退回默认值就是替服务端做判断，而「可用」是唯一能触发
  // 「加入我的素材」的那一个。列 NOT NULL 之后不是主路径，但漏列的投影会走到这里。
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

// 第三个状态维度。文件状态说「源视频准备好了没有」，素材状态说「这条素材还提供给运营选用吗」，
// 使用状态说「当前这个人还在用这条素材吗」。三者各自回答一个问题，规范 §7.2 明令不合并。
//
// 取值就是 `material_usages.status` 的两档（Business Schema MaterialUsage），一档不多：
// 用户 2026-09-30 走查七轮的那句「如果当前代码尚未正式存在某个状态，不要直接新增 enum」
// 挡的正是「已中断」——它在这张表里没有第三个取值。
describe('usage status dimension', () => {
  it('names both relation states the frozen schema allows', () => {
    expect(USAGE_STATUSES).toEqual(['active', 'removed'])
    for (const status of USAGE_STATUSES) {
      expect(usageStatusLabel(status), status).not.toBe(status)
      expect(usageStatusLabel(status), status).not.toBe('-')
      expect(usageStatusTone(status), status).toBeTruthy()
    }
    expect(usageStatusLabel('active')).toBe('使用中')
    expect(usageStatusLabel('removed')).toBe('已放弃')
    expect(usageStatusTone('active')).toBe('success')
  })

  // 读不到不等于「在使用」：素材库列表不带这个字段（那是别人的关系），而「使用中」正是
  // 唯一能触发下载与加入合成的那个取值。缺值兜底成它，就是替服务端宣布了一条关系。
  it('never reads a missing field as 使用中', () => {
    expect(usageStatusLabel(undefined)).toBe('-')
    expect(usageStatusLabel('')).toBe('-')
    expect(usageStatusTone(undefined)).toBe('neutral')
  })

  it('passes through a state it does not know instead of hiding it', () => {
    expect(usageStatusLabel('mystery')).toBe('mystery')
  })
})

// 「下一步」是前端提示文案，不是业务状态（用户提示词明说）。它只把「这个文件状态意味着
// 接下来该做什么」说出来，不发请求、不改状态。
describe('next-step hint', () => {
  it('speaks from the file state', () => {
    expect(nextStepHint({ video_status: 'not_downloaded' })).toBe('下载到本机后可加入合成')
    expect(nextStepHint({ video_status: 'downloading' })).toBe('文件准备中，完成后即可下载到本机')
    expect(nextStepHint({ video_status: 'ready' })).toBe('可加入合成，或重新下载到其他机器')
    expect(nextStepHint({ video_status: 'failed' })).toBe('重试下载，或查看最近一次失败原因')
  })

  // 已放弃的关系先要恢复，才轮到文件那一维：一句「去下载」会让运营以为这条路还通着。
  it('sends a given-up relation to restore before anything else', () => {
    expect(nextStepHint({ usage_status: 'removed', video_status: 'ready' })).toBe('恢复使用后可继续下载与加入合成')
    expect(nextStepHint({ usage_status: 'removed' })).toBe('恢复使用后可继续下载与加入合成')
  })

  it('invents nothing when it cannot read the file state', () => {
    expect(nextStepHint({})).toBe('-')
    expect(nextStepHint(undefined)).toBe('-')
  })
})
