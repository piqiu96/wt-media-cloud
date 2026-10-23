// 顶栏工作环境胶囊的聚合：把「本机 Agent + 执行凭证 + 云端比特账号绑定」算成一个
// 二元结论（可工作 / 需检查），并给出每个阻断项的说明。
//
// 纯函数，便于 vitest 直测；组件（WorkEnvPill.vue）只负责写回响应式状态、轮询和渲染。
// 取数也收在这里（loadWorkEnvInputs），保证「哪几项要刷」只有一个定义。

import { createLocalAgentStatusPage } from './local-agent-status.js'
import { localAgentStateFromStatus } from './service.js'

// 比特账号一致性只能拿掩码比：Cloud 只回绑定账号的前4+后4，本机账号由 Desktop
// 按同一规则掩掉后才进页面。任一侧读不出可比的掩码（空或整体 `****`）都不能
// 证明一致，按不一致提示 —— 多给一次自助换绑入口，好过静默当作已一致。
function masksAgree(boundMasked, localMasked) {
  const bound = typeof boundMasked === 'string' ? boundMasked.trim() : ''
  const local = typeof localMasked === 'string' ? localMasked.trim() : ''
  if (!bound || !local || bound === '****' || local === '****') return false
  return bound === local
}

// 「本机账号 ≠ 绑定账号」的唯一判定源：个人中心换绑入口的可见性与胶囊的
// workable/blocker 都走这里，避免两处各写一份判定而漂移。
export function bitAccountDiffers(binding, localMasked) {
  const local = typeof localMasked === 'string' ? localMasked.trim() : ''
  return !!binding?.bound && !!local && !masksAgree(binding?.main_user_id_masked || '', local)
}

/**
 * 一次性读齐工作环境判定的四项输入。
 *
 * 四项互相独立：某一项失败（最常见的是 Agent sidecar 还没起来时 local_agent_status
 * 直接返回错误）不能让其余几项从界面上消失——否则胶囊会因为一个正在启动的组件
 * 整块变成「需检查」，而页面上本来就读得到的绑定信息一并丢掉。
 *
 * @param {object} deps
 * @param {object} deps.session session 客户端（me()）
 * @param {object} deps.bitAccounts 比特账号客户端（get()）
 * @param {object} deps.service 本机 Agent 服务（status() / tokenFacts()）
 * @param {boolean} [deps.reuseScan] 是否允许复用比特浏览器扫描结果（后台轮询用）
 * @returns {Promise<{cloudUser, binding, snapshot, tokenFacts, errors}>}
 */
export async function loadWorkEnvInputs({ session, bitAccounts, service, reuseScan = false }) {
  const [me, binding, status, token] = await Promise.allSettled([
    session.me(),
    bitAccounts.get(),
    service.status({ reuseScan }),
    service.tokenFacts(),
  ])
  const errors = [me, binding, status, token]
    .filter((r) => r.status === 'rejected')
    .map((r) => r.reason)
  return {
    cloudUser: me.status === 'fulfilled' ? me.value : null,
    binding: binding.status === 'fulfilled' ? binding.value : null,
    snapshot: status.status === 'fulfilled' ? status.value : null,
    tokenFacts: token.status === 'fulfilled' ? token.value : null,
    errors,
  }
}

/**
 * @param {object} input
 * @param {object|null} input.snapshot   本机 Agent 状态（Agent 上报的 snake_case 形状，
 *                                       即 normalizeLocalAgentStatus 的产物）
 * @param {object|null} input.cloudUser  当前登录用户（session.me() 的产物，可为 null）
 * @param {object|null} input.binding    云端比特账号绑定（bitAccount.get() 的产物）
 * @param {object|null} input.tokenFacts 本进程执行凭证的事实（tokenFacts() 的产物）
 */
export function aggregateWorkEnv({ snapshot, cloudUser, binding, tokenFacts }) {
  const state = localAgentStateFromStatus(snapshot)
  const page = createLocalAgentStatusPage(state, {
    cloudUser,
    tokenConfigured: !!tokenFacts?.configured,
  })
  const bitBound = !!binding?.bound
  // 比特账号不一致（在比特浏览器客户端换过账号、系统还没跟上）也是「需检查」：
  // 此时本机浏览器操作会被执行门挡住，提前在这里说出来。
  const accountDiffers = bitAccountDiffers(binding, state.mainUserIdMasked)
  const workable = page.ready && bitBound && !accountDiffers

  // 阻断项按「云端绑定 → 账号不一致 → 本机」的优先级排，面板照此逐条展示。
  const blockers = []
  if (!bitBound) blockers.push('尚未绑定比特浏览器账号，请到「个人信息」页完成绑定')
  else if (accountDiffers) blockers.push('比特浏览器登录账号与绑定账号不一致，请到「个人信息」页确认换绑')
  if (!page.ready) blockers.push(page.trustReason)

  return { page, bitBound, bitAccountDiffers: accountDiffers, workable, blockers }
}
