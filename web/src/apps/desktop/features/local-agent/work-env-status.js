// 顶栏工作环境胶囊的聚合：把「本机 Agent + 云端设备绑定 + 当前设备身份」算成一个
// 二元结论（可工作 / 需检查），并给出每个阻断项的说明。
//
// 纯函数，便于 vitest 直测；组件（WorkEnvPill.vue）只负责取数、轮询和渲染。

import { createLocalAgentStatusPage } from './local-agent-status.js'
import { localAgentStateFromStatus } from './service.js'

/**
 * @param {object} input
 * @param {object|null} input.snapshot   本机 Agent 状态（Agent 上报的 snake_case 形状，
 *                                       即 normalizeLocalAgentStatus 的产物）
 * @param {object|null} input.cloudUser  当前登录用户（session.me() 的产物，可为 null）
 * @param {object|null} input.binding    云端设备绑定（deviceBinding.get() 的产物）
 * @param {object|null} input.localDevice 本机设备身份（invoke('local_device_identity') 的产物）
 */
export function aggregateWorkEnv({ snapshot, cloudUser, binding, localDevice }) {
  const page = createLocalAgentStatusPage(localAgentStateFromStatus(snapshot), { cloudUser })
  const bound = !!binding?.bound
  const localDeviceId = localDevice?.device_id || ''
  const mismatched = bound && !!localDeviceId && binding.device_id !== localDeviceId
  const deviceReady = bound && !mismatched
  const workable = page.bound && deviceReady

  // 阻断项按「设备 → 本机」的优先级排，面板照此逐条展示。
  const blockers = []
  if (!bound) blockers.push('尚未绑定运营电脑，请在个人中心完成绑定')
  else if (mismatched) blockers.push('当前电脑与绑定设备不一致，请在个人中心确认')
  if (!page.bound) blockers.push(page.trustReason)

  return { page, bound, localDeviceId, mismatched, deviceReady, workable, blockers }
}
