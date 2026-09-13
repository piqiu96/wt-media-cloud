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
  return true
}

function cloudBaseUrl() {
  if (typeof window === "undefined") return "http://127.0.0.1:18080"
  const origin = window.location?.origin || "http://127.0.0.1:18080"
  // Packaged Desktop runs on http://tauri.localhost, which is NOT the Cloud
  // API host; the local Cloud server is always the API base.
  return origin.startsWith("http://127.0.0.1:18080") ? origin : "http://127.0.0.1:18080"
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

export async function bindTrustedLocalAgent() {
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
      throw new Error("当前比特浏览器登录账号与系统绑定账号不一致，已阻止本机浏览器相关操作。请切换回已绑定的比特浏览器账号后重新检测；如果系统绑定错误，请联系管理员解除绑定后重新绑定。")
    }
    throw error
  }
  const status = await service.status()
  if (status.node_id) {
    try {
      await service.refreshRuntime({ cloudBaseUrl: cloudBaseUrl() })
      return createDesktopStatusPreview()
    } catch (error) {
      console.warn("刷新本机可信状态失败，将重新确认并绑定当前Desktop执行凭证", error)
    }
  }
  const ticket = await createRuntimeBindingClient().createBindingTicket()
  await service.bindSession({
    bindingTicket: ticket.binding_token,
    cloudBaseUrl: cloudBaseUrl(),
  })
  return createDesktopStatusPreview()
}
