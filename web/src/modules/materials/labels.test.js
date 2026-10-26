import { describe, expect, it } from 'vitest'
import {
  DOWNLOAD_STATUSES,
  JOIN_FILTERS,
  MATERIAL_STATUSES,
  MY_USAGE_FILTERS,
  TEAM_USAGE_FILTERS,
  USAGE_STATUSES,
  VIDEO_STATUSES,
  downloadActionLabel,
  downloadStatusLabel,
  downloadStatusTone,
  gameName,
  joinStatus,
  libraryNextStepHint,
  materialStatusLabel,
  materialStatusTone,
  myUsageLabel,
  myUsageTone,
  nextStepHint,
  shortDigest,
  usageFacts,
  usageOperators,
  usageOutputCount,
  usageStatusLabel,
  usageStatusTone,
  usageSummary,
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

// 素材库的三个使用维度：加入状态是查看者自己的关系，我的使用是查看者是否做过
// 成功成片，使用情况是本团队成功成片的汇总。发布相关口径属于 M6，这里不出现。
describe('library usage dimensions', () => {
  it('offers the server-side filters with user-facing labels', () => {
    expect(JOIN_FILTERS.map((item) => item.value)).toEqual(['joined', 'not_joined'])
    expect(MY_USAGE_FILTERS.map((item) => item.value)).toEqual(['used', 'unused'])
    expect(TEAM_USAGE_FILTERS.map((item) => item.value)).toEqual(['used', 'unused'])
    expect(JOIN_FILTERS.map((item) => item.label)).toEqual(['已加入', '未加入'])
  })

  it('reads 加入状态 from the viewer relation, not from a stored lifecycle', () => {
    expect(joinStatus({ usage_status: 'active' })).toEqual({ label: '已加入', tone: 'success' })
    expect(joinStatus({ usage_status: 'removed' })).toEqual({ label: '已移出', tone: 'neutral' })
    expect(joinStatus({})).toEqual({ label: '未加入', tone: 'neutral' })
    expect(joinStatus(undefined)).toEqual({ label: '未加入', tone: 'neutral' })
  })

  it('names 我的使用 from successful outputs only', () => {
    expect(myUsageLabel('used')).toBe('已使用')
    expect(myUsageLabel('unused')).toBe('未使用')
    expect(myUsageLabel(undefined)).toBe('-')
    expect(myUsageTone('used')).toBe('info')
    expect(myUsageTone('unused')).toBe('neutral')
  })

  it('summarises the team usage into one line', () => {
    const row = {
      usage: {
        output_count: 12,
        operator_count: 2,
        operators: [{ id: 9, name: '张三', output_count: 8 }, { id: 11, name: '李四', output_count: 4 }],
      },
    }
    expect(usageSummary(row)).toBe('成片 12 · 使用 2 人')
    // 名单只进弹窗；摘要不因为读不到姓名就改口。
    expect(usageSummary({ usage: { output_count: 12, operator_count: 2 } })).toBe('成片 12 · 使用 2 人')
    expect(usageOperators(row).map((operator) => operator.name)).toEqual(['张三', '李四'])
    expect(usageOperators({ usage: { output_count: 12, operator_count: 2 } })).toEqual([])
  })

  // 历史成片没有负责人：只写成片数，不写「负责人未记录」，也没有第二行。
  it('writes only the output count when no operator is known', () => {
    expect(usageSummary({ usage: { output_count: 3, operator_count: 0 } })).toBe('成片 3')
    expect(usageSummary({ usage: { output_count: 0, operator_count: 0 } })).toBe('—')
    // 没有 usage 与「usage 是 0」是两件事，但页面都只能画 —。
    expect(usageSummary({})).toBe('—')
    expect(usageSummary(undefined)).toBe('—')
    expect(usageOutputCount({ usage: { output_count: 3 } })).toBe(3)
    expect(usageOutputCount({ usage: { output_count: 0 } })).toBe(0)
    expect(usageOutputCount({})).toBeNull()
    expect(usageOutputCount(undefined)).toBeNull()
  })

  it('lists only the facts that carry a value', () => {
    const facts = usageFacts({
      usage: {
        output_count: 7,
        operator_count: 2,
        operators: [{ id: 9, name: '张三', output_count: 4 }, { id: 11, name: '李四', output_count: 1 }],
      },
    })
    expect(facts.map((fact) => fact.label)).toEqual(['成功成片', '使用人数', '使用人'])
    expect(facts.map((fact) => fact.value)).toEqual(['7', '2', '张三、李四'])
  })

  it('drops the usage section entirely when there is nothing to say', () => {
    // 有数没人：只留成功成片，不补「未记录负责人的成片」。
    expect(usageFacts({ usage: { output_count: 3, operator_count: 0 } }).map((fact) => fact.label))
      .toEqual(['成功成片'])
    // 0 与缺省都不渲染；0 之前的「0 是数据」现在由列表的 — 承担，详情不摆空事实。
    expect(usageFacts({ usage: { output_count: 0, operator_count: 0 } })).toEqual([])
    expect(usageFacts({})).toEqual([])
    expect(usageFacts(undefined)).toEqual([])
  })

  it('points the library next step at joining or My Materials, never at a local download', () => {
    expect(libraryNextStepHint({ usage_status: 'removed', video_status: 'ready' })).toBe('已移出我的素材，可重新加入')
    expect(libraryNextStepHint({ usage_status: 'active', video_status: 'ready' })).toContain('「我的素材」下载')
    expect(libraryNextStepHint({ usage_status: 'active', video_status: 'not_downloaded' })).toContain('文件准备完成后')
    expect(libraryNextStepHint({ video_status: 'ready' })).toBe('加入我的素材后可下载，并在合成任务页选用')
    expect(libraryNextStepHint({})).toBe('加入我的素材；文件准备完成后可下载')
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

  // 读不到不等于「在使用」：「使用中」正是唯一能触发下载与加入合成的那个取值。
  // 缺值兜底成它，就是替服务端宣布了一条关系。素材库那一侧另有 joinStatus：
  // 同一个 usage_status，在素材库说「已加入 / 已移出」，在我的素材说「使用中 / 已放弃」。
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
//
// 提示里指向的是**页面名字**（用户裁定「下一步提示指向合成任务页」）：合成的入口在
// CHG-20261015-098 从我的素材搬去了合成任务页，说「加入合成」会让运营在这一页上找一颗
// 已经不存在的按钮。文案只承诺「可在合成任务页选用」，不描述那一页怎么操作。
describe('next-step hint', () => {
  it('speaks from the file state', () => {
    expect(nextStepHint({ video_status: 'not_downloaded' })).toBe('下载到本机后可在合成任务页选用')
    expect(nextStepHint({ video_status: 'downloading' })).toBe('文件准备中，完成后即可下载到本机')
    expect(nextStepHint({ video_status: 'ready' })).toBe('可在合成任务页选用，或重新下载到其他机器')
    expect(nextStepHint({ video_status: 'failed' })).toBe('重新下载，或查看最近一次失败原因')
  })

  // 已放弃的关系先要恢复，才轮到文件那一维：一句「去下载」会让运营以为这条路还通着。
  // 恢复之后也不必回这一页——合成任务页只列「使用中」的素材，恢复即回到候选里。
  it('sends a given-up relation to restore before anything else', () => {
    const removed = '恢复使用后可继续下载，并可在合成任务页选用'
    expect(nextStepHint({ usage_status: 'removed', video_status: 'ready' })).toBe(removed)
    expect(nextStepHint({ usage_status: 'removed' })).toBe(removed)
  })

  it('invents nothing when it cannot read the file state', () => {
    expect(nextStepHint({})).toBe('-')
    expect(nextStepHint(undefined)).toBe('-')
  })
})
