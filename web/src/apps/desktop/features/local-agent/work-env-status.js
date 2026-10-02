// 顶栏工作环境胶囊的聚合：把「本机 Agent + 云端设备绑定 + 当前设备身份」算成一个
// 二元结论（可工作 / 需检查），并给出每个阻断项的说明。
//
// 纯函数，便于 vitest 直测；组件（WorkEnvPill.vue）只负责写回响应式状态、轮询和渲染。
// 取数也收在这里（loadWorkEnvInputs），因为「哪几项要刷」曾经是缺陷本身。

import { createLocalAgentStatusPage } from './local-agent-status.js'
import { localAgentStateFromStatus } from './service.js'

// 比特账号一致性只能拿掩码比：Cloud 只回绑定账号的前4+后4（全文不进前端），
// 本机账号按同样规则截首尾比对。32 位 hex 下碰撞概率 2^-32，仅作 UI 提示；
// 权威校验在 Cloud 执行门（CheckLocalTrust 的全文比对）。
export function bitAccountMaskMatches(boundMasked, localId) {
  const bound = typeof boundMasked === 'string' ? boundMasked.trim() : ''
  const local = typeof localId === 'string' ? localId.trim() : ''
  if (!bound || bound === '****' || local.length < 8) return false
  return bound === `${local.slice(0, 4)}****${local.slice(-4)}`
}

// 与后端 maskDeviceIdentity（runtimebinding/repository/store_mysql.go）同规则：
// 前 4 + **** + 后 4；不足 9 位整体掩掉，空值仍为空。
export function maskBitAccountId(value) {
  const id = typeof value === 'string' ? value.trim() : ''
  if (!id) return ''
  return id.length <= 8 ? '****' : `${id.slice(0, 4)}****${id.slice(-4)}`
}

// 「本机账号 ≠ 绑定账号」的唯一判定源：个人中心切换入口的可见性与胶囊的
// workable/blocker 都走这里，避免两处各写一份判定而漂移。
export function bitAccountDiffers(binding, localMainUserId) {
  const local = typeof localMainUserId === 'string' ? localMainUserId.trim() : ''
  return !!binding?.bit_account_bound && !!local && !bitAccountMaskMatches(binding?.bit_main_user_id_masked || '', local)
}

/**
 * 一次性读齐工作环境判定的四个输入。
 *
 * 四项互相独立：某一项失败（最常见的是 Agent sidecar 还没起来时 local_agent_status
 * 直接返回错误）不能让其余三项从界面上消失——否则胶囊会因为一个正在启动的组件
 * 整块变成「需检查」，而页面上本来就读得到的绑定信息一并丢掉。
 *
 * @param {object} deps
 * @param {object} deps.session session 客户端（me()）
 * @param {object} deps.devices 设备绑定客户端（get()）
 * @param {object} deps.service 本机 Agent 服务（status()）
 * @param {Function} deps.invoke Tauri invoke
 * @param {boolean} [deps.reuseScan] 是否允许复用比特浏览器扫描结果（后台轮询用）
 * @returns {Promise<{cloudUser, localDevice, snapshot, binding, errors}>}
 */
export async function loadWorkEnvInputs({ session, devices, service, invoke, reuseScan = false }) {
  const [me, device, status, binding] = await Promise.allSettled([
    session.me(),
    invoke('local_device_identity'),
    service.status({ reuseScan }),
    devices.get(),
  ])
  const errors = [me, device, status, binding]
    .filter((r) => r.status === 'rejected')
    .map((r) => r.reason)
  return {
    cloudUser: me.status === 'fulfilled' ? me.value : null,
    localDevice: device.status === 'fulfilled' ? device.value : null,
    snapshot: status.status === 'fulfilled' ? status.value : null,
    binding: binding.status === 'fulfilled' ? binding.value : null,
    errors,
  }
}

/**
 * @param {object} input
 * @param {object|null} input.snapshot   本机 Agent 状态（Agent 上报的 snake_case 形状，
 *                                       即 normalizeLocalAgentStatus 的产物）
 * @param {object|null} input.cloudUser  当前登录用户（session.me() 的产物，可为 null）
 * @param {object|null} input.binding    云端设备绑定（deviceBinding.get() 的产物）
 * @param {object|null} input.localDevice 本机设备身份（invoke('local_device_identity') 的产物）
 */
export function aggregateWorkEnv({ snapshot, cloudUser, binding, localDevice }) {
  const state = localAgentStateFromStatus(snapshot)
  const page = createLocalAgentStatusPage(state, { cloudUser })
  const bound = !!binding?.bound
  const localDeviceId = localDevice?.device_id || ''
  const mismatched = bound && !!localDeviceId && binding.device_id !== localDeviceId
  const deviceReady = bound && !mismatched
  // 比特账号不一致（在比特浏览器客户端换过账号、系统还没跟上）也是「需检查」：
  // 此时本机浏览器操作会被 23002 挡住，提前在这里说出来。
  const accountDiffers = bitAccountDiffers(binding, state.mainUserId)
  const workable = page.bound && deviceReady && !accountDiffers

  // 阻断项按「设备 → 账号 → 本机」的优先级排，面板照此逐条展示。
  const blockers = []
  if (!bound) blockers.push('尚未绑定运营电脑，请在个人中心完成绑定')
  else if (mismatched) blockers.push('当前电脑与绑定设备不一致，请在个人中心确认')
  else if (accountDiffers) blockers.push('比特浏览器登录账号与绑定账号不一致，请在个人信息页点「比特账号绑定」确认')
  if (!page.bound) blockers.push(page.trustReason)

  return { page, bound, localDeviceId, mismatched, bitAccountDiffers: accountDiffers, deviceReady, workable, blockers }
}
