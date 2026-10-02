// Desktop 本地 Agent 服务初始化
// 分离自 main.ts，避免模块间循环引用

import { createLocalAgentService, createMockLocalAgentService } from "./service.js"
import { createLocalAgentStore } from "./store.js"
import { createLocalAgentStatusPage } from "./local-agent-status.js"
import { createSessionClient } from "../../../../shared/api/session.js"
import { createProfileBindingClient } from "../../../../shared/api/profileBindings.js"
import { createRuntimeBindingClient } from "../../../../shared/api/runtimeBinding.js"
// Static import: a dynamic import() of a node_modules chunk does not resolve in
// the packaged Tauri WebView ("Module name ... does not resolve to a valid file").
import { invoke } from "@tauri-apps/api/core"

function isTauri() {
  return typeof window !== "undefined" && window.__TAURI_INTERNALS__ !== undefined
}

export async function startDesktopLocalAgent({ tauri = isTauri(), invokeImpl = invoke } = {}) {
  if (!tauri) return false
  await createLocalAgentService({ invoke: invokeImpl }).start()
  // A restart clears the Agent's in-memory node credential. The same bound
  // installation may renew it using the current login, without manual binding.
  void ensureTrustedLocalAgent().catch((error) => {
    console.info('本机执行授权将在登录或下一次执行前恢复', error?.message || error)
  })
  return true
}

// The command that answers "where is Cloud". One definition for the WebView
// side; the Rust side spells it in `commands/public_config.rs`.
export const PUBLIC_CONFIG_COMMAND = "get_public_config"

// The last answer `get_public_config` managed to give.
//
// At module scope because the answer outlives one call: binding and runtime
// refresh are pressed at different moments, and the second press must not lose
// the address the first one learned.
//
// It starts empty, and stays empty if the native side never answers. Empty is
// the honest failure value — the callers already report "Cloud地址为空" — and it
// is never a loopback literal. That literal was the old behaviour: it is wrong
// in every packaged build (their origin is `tauri.localhost`, not the Cloud API
// host), and the whole point of asking the native side is that only it knows.
let lastKnownCloudBaseUrl = ""

export async function cloudBaseUrl({ invokeImpl = invoke } = {}) {
  try {
    const config = await invokeImpl(PUBLIC_CONFIG_COMMAND)
    // A successful answer replaces the cache even when it is empty: the native
    // side is authoritative, and "Cloud is not configured" is an answer rather
    // than a failure to get one.
    lastKnownCloudBaseUrl =
      typeof config?.cloud_base_url === "string" ? config.cloud_base_url.trim() : ""
  } catch {
    // Keep the last known good address. Nothing is reported here on purpose:
    // an empty address already surfaces to the user as the existing
    // "Cloud地址为空" error, and a second message would say the same thing twice.
  }
  return lastKnownCloudBaseUrl
}

async function createDesktopStatusContext() {
  const tauri = isTauri()
  let service

  if (tauri) {
    service = createLocalAgentService({ invoke })
  } else {
    // Browser previews never contact a local Agent port. Only the Tauri Rust
    // bridge can reach the Local Agent; non-Desktop contexts use mock facts.
    service = createMockLocalAgentService()
  }

  const store = createLocalAgentStore({ service })
  await store.refresh()
  const snapshot = store.snapshot()
  let cloudUser = null
  try {
    cloudUser = await createSessionClient().me()
  } catch {
    cloudUser = null
  }
  return { service, store, snapshot, cloudUser, tauri }
}

export async function createDesktopStatusPreview() {
  const { snapshot, cloudUser, tauri } = await createDesktopStatusContext()
  console.info(`wt-media-desktop local agent: ${snapshot.status}`)
  return createLocalAgentStatusPage(snapshot, { cloudUser, canBindTrustedNode: tauri })
}

export async function bindTrustedLocalAgent({ bindDevice = false } = {}) {
  const { service, cloudUser, tauri } = await createDesktopStatusContext()
  if (!tauri) {
    throw new Error("请在Desktop应用内刷新本机可信状态")
  }
  if (!cloudUser) {
    throw new Error("请先登录运营平台")
  }
  const localSnapshot = await service.profileScan()
  const mainUserId = localSnapshot?.main_user_id || ""
  if (!mainUserId) {
    throw new Error("未读取到比特浏览器账号，请确认比特浏览器已登录后重新检测本机环境。")
  }
  try {
    await createProfileBindingClient().confirmMainIdentityDirect(mainUserId)
  } catch (error) {
    if (error?.errcode === 23002) {
      throw new Error("当前比特浏览器登录账号与系统绑定账号不一致，已阻止本机浏览器相关操作。请切换回已绑定的比特浏览器账号后重新检测；如果确实要改用当前账号，可在「个人信息 → 设备与本机环境」点「以当前环境为准」自助确认。")
    }
    throw error
  }
  // Asked once, here, rather than at each call site: both paths below need it,
  // and reading it twice would let the two disagree if the config changed
  // between them.
  const address = await cloudBaseUrl()
  const status = await service.status()
  if (status.node_id) {
    try {
      await service.refreshRuntime({ cloudBaseUrl: address })
      return createDesktopStatusPreview()
    } catch (error) {
      console.warn("刷新本机可信状态失败，将重新确认并绑定当前Desktop执行凭证", error)
    }
  }
  const ticket = await createRuntimeBindingClient().createBindingTicket()
  await service.bindSession({
    bindingTicket: ticket.binding_token,
    cloudBaseUrl: address,
    bindDevice,
  })
  return createDesktopStatusPreview()
}

let pendingRenewal = null
export function ensureTrustedLocalAgent() {
  if (!isTauri()) return Promise.resolve(null)
  if (!pendingRenewal) {
    pendingRenewal = bindTrustedLocalAgent().finally(() => { pendingRenewal = null })
  }
  return pendingRenewal
}
