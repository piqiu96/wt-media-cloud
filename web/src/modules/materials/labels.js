import { formatDate } from '../../shared/utils/datetime.js'

// 素材的状态与展示文案。
//
// 四种视频状态直接来自冻结的 `video_status` enum（contracts/business-schemas/v1/
// content-production.yaml），不多不少 —— 页面上出现第五种状态（比如「下载中」之外的
// 「解析中」）就意味着前端自己发明了一个服务端不会写的值，那种状态永远不会出现，
// 而它对应的进度条会永远停在初始位置。
export const VIDEO_STATUSES = ['not_downloaded', 'downloading', 'ready', 'failed']

const VIDEO_STATUS_LABELS = {
  not_downloaded: '未准备',
  downloading: '准备中',
  ready: '可下载',
  failed: '准备失败',
}

const VIDEO_STATUS_TONES = {
  not_downloaded: 'neutral',
  downloading: 'info',
  ready: 'success',
  failed: 'danger',
}

export function videoStatusLabel(value) {
  return VIDEO_STATUS_LABELS[value] || value || '-'
}

export function videoStatusTone(value) {
  return VIDEO_STATUS_TONES[value] || 'neutral'
}

// ── 下载状态（按用户派生的下载生命周期）────────────────────────────────────
//
// `video_status` 回答「源视频准备好了没有」，这里回答「**这个运营**在这条素材上下载到了
// 哪一步」。两者正交：同一条素材可以「可下载 + 还没下过」，也可以「下载中 + 之前已成功
// 过」（正在重新下载）。
//
// 取值来自 `MaterialUsage.download_status` —— 后端按「最新一条 user_download 任务」派生，
// pending/running→downloading、success→downloaded、failed/cancelled→failed。空串是第四档
// 「从未下载过」，CHG-069 任务 23 起**我的素材按钮矩阵与文件状态列就由它驱动**（任务状态
// 与业务状态严格隔离），所以它不再是「回落 video_status」的缺省，而是矩阵里的一格。
//
// 不并入 `VIDEO_STATUSES`：那是冻结的 `video_status` enum，一个服务端不会写的值出现在
// 那里，页面上就会出现永远等不到的状态。
export const DOWNLOAD_STATUSES = ['', 'downloading', 'downloaded', 'failed']

const DOWNLOAD_STATUS_LABELS = {
  '': '未下载',
  downloading: '下载中',
  downloaded: '已下载',
  failed: '下载失败',
}

const DOWNLOAD_STATUS_TONES = {
  '': 'neutral',
  downloading: 'info',
  downloaded: 'success',
  failed: 'danger',
}

export function downloadStatusLabel(value) {
  return DOWNLOAD_STATUS_LABELS[value] || value || '-'
}

export function downloadStatusTone(value) {
  return DOWNLOAD_STATUS_TONES[value] || 'neutral'
}

/**
 * 主操作的文案由下载状态驱动（交互规范 §7.4：异常状态给出「状态 + 下一步恢复动作」）。
 * 失败行的下一步是「重新下载」，其余是「下载」——服务端的 CreateDownload 对未下载/
 * 失败都会先建准备任务再下，按钮行为不变，只是把「失败后再次执行」这个语义说出来。
 * 行内与详情抽屉共用，两处文案分叉就是同义词混用的起点。
 */
export function downloadActionLabel(status) {
  return status === 'failed' ? '重新下载' : '下载'
}

/**
 * 游戏名。查不到就退回 id 而不是 `-`：素材的 `game_id` 是一份真实数据，
 * 游戏表里没有它只是「这一页拿不到名字」，把它显示成「没有游戏」是另一回事。
 */
export function gameName(games, id) {
  if (!id) return '-'
  return games.find((item) => item.id === id)?.name || id
}

/** 摘要在表里放不下整串，但也不该断成两行：截到 16 位再加省略号。 */
export function shortDigest(value) {
  return value ? `${value.slice(0, 16)}…` : '-'
}

// ── 素材状态（第二个状态维度）─────────────────────────────────────────────
//
// 文件状态回答「源视频准备好了没有」，素材状态回答「这条素材还提供给运营选用吗」。
// 规范 §7.2 明令两者不得合并：同一个素材可以同时是「已暂停 + 可下载」，把两个维度
// 压成一个混合状态，最先丢掉的就是「暂停了但文件还在」这种运营真正要做判断的情形。
// 三个取值来自规范 §16.2 的素材生命周期（用词说明：「已下架」不是「已退役」）。
//
// 读取协议是 `material.status`（materials.status 列，NOT NULL，键恒在）。
// 读不到显示 `—` 而不是退回默认值：缺值不等于「可用」，而「可用」正是唯一能触发
// 「加入我的素材」的那一个。没有任何接口能把素材改成 paused / delisted，所以今天读到的
// 值恒为 available —— 那是这批素材的真实状态，不是占位。
export const MATERIAL_STATUSES = ['available', 'paused', 'delisted']

const MATERIAL_STATUS_LABELS = {
  available: '可用',
  paused: '已暂停',
  delisted: '已下架',
}

const MATERIAL_STATUS_TONES = {
  available: 'success',
  paused: 'warning',
  delisted: 'neutral',
}

export function materialStatusLabel(value) {
  return MATERIAL_STATUS_LABELS[value] || value || '-'
}

export function materialStatusTone(value) {
  return MATERIAL_STATUS_TONES[value] || 'neutral'
}

// ── 使用状态（第三个状态维度）─────────────────────────────────────────────
//
// 文件状态说「源视频准备好了没有」，素材状态说「这条素材还提供给运营选用吗」，
// 使用状态说「当前这个人还在用这条素材吗」。三个各自回答一个问题，规范 §7.2 明令不合并。
//
// 取值就是 `material_usages.status` 的两档（Business Schema MaterialUsage 的
// `active / removed`），一档不多 —— 那个 CHECK 约束里没有第三个值。用户 2026-09-30
// 那句「如果当前代码尚未正式存在某个状态，不要直接新增 enum」挡的正是「已中断」。
//
// 读取协议是 `material.usage_status`：素材库列表不返回它（那是别人的关系），
// 只有「我的素材」的行与它打开的详情带着。读不到不退回 active —— 「使用中」正是
// 唯一能触发下载与加入合成的那个取值，兜底成它就是替服务端宣布了一条关系。
export const USAGE_STATUSES = ['active', 'removed']

const USAGE_STATUS_LABELS = {
  active: '使用中',
  removed: '已放弃',
}

const USAGE_STATUS_TONES = {
  active: 'success',
  removed: 'neutral',
}

export function usageStatusLabel(value) {
  return USAGE_STATUS_LABELS[value] || value || '-'
}

export function usageStatusTone(value) {
  return USAGE_STATUS_TONES[value] || 'neutral'
}

/**
 * 「下一步」是前端提示文案，不是业务状态（2026-09-30 走查七轮用户提示词明说）。
 * 它只把「这两个维度意味着接下来该做什么」说出来，不发请求、不改任何状态。
 *
 * 已放弃的关系先说恢复：这时候再提「去下载」，运营会以为那条路还通着。
 * 谁也不认识的取值画 `-`，不猜 —— 猜出来的下一步会指向一个不存在的动作。
 */
export function nextStepHint(material) {
  if (material?.usage_status === 'removed') return '恢复使用后可继续下载与加入合成'
  switch (material?.video_status) {
    case 'not_downloaded':
      return '下载到本机后可加入合成'
    case 'downloading':
      return '文件准备中，完成后即可下载到本机'
    case 'ready':
      return '可加入合成，或重新下载到其他机器'
    case 'failed':
      // 「重试」这个动作已被收起（走查裁定只留「重新下载」），所以这句提示也用同一个
      // 词 —— 指着一个界面上不存在的按钮说「下一步」，比不说更糟。
      return '重新下载，或查看最近一次失败原因'
    default:
      return '-'
  }
}

// ── 使用情况 ──────────────────────────────────────────────────────────────
//
// 列表的「使用情况」列与详情的「使用情况」卡读的同一份协议，形状是 `material.usage`：
//
//   clip_count         已生产成片数
//   published_count    已发布数
//   last_produced_at   最近生产时间（ISO）
//   last_published_at  最近发布时间（ISO）
//   duplicate_risk     normal | suspected    重复风险
//
// `usage` 还没有数据（要跨成片与发布聚合）。这里把键名定死，好让后端照着填 ——
// 键名对不上就是一次静默的空格子，没有东西会报错。
export const USAGE_FIELDS = ['clip_count', 'published_count', 'last_produced_at', 'last_published_at', 'duplicate_risk']

export const DUPLICATE_RISKS = ['normal', 'suspected']

const DUPLICATE_RISK_LABELS = { normal: '正常', suspected: '疑似重复' }

const DUPLICATE_RISK_TONES = { normal: 'success', suspected: 'danger' }

export function duplicateRiskLabel(value) {
  return DUPLICATE_RISK_LABELS[value] || value || '-'
}

export function duplicateRiskTone(value) {
  return DUPLICATE_RISK_TONES[value] || 'neutral'
}

// 0 是数据，缺省不是：0 说的是「一条成片都没生产」，缺省说的是「这个字段还没来」。
// 两者画成同一个样子，运营就分不清「没做过」和「还没统计」。
function usageCount(value) {
  if (value === null || value === undefined || value === '') return '-'
  return Number(value).toLocaleString()
}

/** 列表那一格的两行。读不到 usage 时立的是骨架（`-`），不是猜测值。 */
export function usageLines(material) {
  const usage = material?.usage || {}
  return {
    counts: `成片 ${usageCount(usage.clip_count)} · 发布 ${usageCount(usage.published_count)}`,
    recent: `最近发布 ${formatDate(usage.last_published_at)}`,
  }
}

/** 详情「使用情况」卡的五个事实，顺序即设计图的顺序。`tone` 只挂在重复风险上。 */
export function usageFacts(material) {
  const usage = material?.usage || {}
  return [
    { key: 'clip_count', label: '已生产成片', value: usageCount(usage.clip_count) },
    { key: 'published_count', label: '已发布', value: usageCount(usage.published_count) },
    { key: 'last_produced_at', label: '最近生产', value: formatDate(usage.last_produced_at) },
    { key: 'last_published_at', label: '最近发布', value: formatDate(usage.last_published_at) },
    {
      key: 'duplicate_risk',
      label: '重复风险',
      value: duplicateRiskLabel(usage.duplicate_risk),
      tone: duplicateRiskTone(usage.duplicate_risk),
    },
  ]
}
