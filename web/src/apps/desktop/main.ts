import { createApp } from "vue"
import { createPinia } from "pinia"
import TDesign from "tdesign-vue-next"
import { createDesktopRouter } from "./router"
import App from "./App.vue"
import { canUseDesktop, canUseLocalAgent } from "../../utils.js"
import { cloudBaseUrl, startDesktopLocalAgent, stopDesktopLocalAgent } from "./features/local-agent/init.js"
import { setDesktopCloudOrigin } from "../../shared/api/http.js"
import { installWebviewErrorReporting } from "./webviewErrors.js"
import "tdesign-vue-next/es/style/index.css"
import "../../shared/styles/layout.css"
import "../../styles/design-token.css"
import "../../shared/styles/resource-module.css"

// 尽早装上报，才能覆盖后续初始化与首屏渲染期间的报错。
// 原先由 Rust 用 window.eval 注入，被 CSP 拦掉，从未生效。
installWebviewErrorReporting()

const app = createApp(App)
const pinia = createPinia()
const router = createDesktopRouter()
;(window as any).__WT_MEDIA_APP__ = "desktop"

app.use(pinia)
app.use(TDesign)
app.use(router)

// 本机 Agent 只在普通运营登录时启动。角色切换（含退出后换号）时重新同步一次，
// 避免高级运营／管理员继承上一段会话留下的 sidecar。
let syncedAgentUser = ""
async function syncLocalAgentFor(user: { id?: number; role?: string }) {
  const key = `${user?.id ?? ""}:${user?.role ?? ""}`
  if (key === syncedAgentUser) return
  syncedAgentUser = key
  if (canUseLocalAgent(user)) {
    await startDesktopLocalAgent({ tauri: true }).catch((error) => {
      console.warn("Local Agent startup failed; use the environment page to retry.", error)
    })
    return
  }
  await stopDesktopLocalAgent({ tauri: true }).catch((error) => {
    console.warn("Local Agent stop failed.", error)
  })
}

// Auth guard
router.beforeEach(async (to, from, next) => {
  if (to.path === "/login") {
    next()
    return
  }

  // Desktop 本地页面不需要认证
  if (to.name === "LocalLogs" || to.name === "LocalSettings") {
    next()
    return
  }

  try {
    const { createSessionClient } = await import("../../shared/api/session.js")
    const sessionClient = createSessionClient()
    const user = await sessionClient.me()
    if (!canUseDesktop(user)) {
      await sessionClient.logout().catch(() => {})
      next("/login?desktop_role_forbidden=1")
      return
    }
    void syncLocalAgentFor(user)
    // M2 本机资源（账号、开户、浏览器窗口、代理）只对普通运营开放。
    if (to.meta?.localOnly && !canUseLocalAgent(user)) {
      next("/content-pool")
      return
    }
    next()
  } catch {
    next("/login")
  }
})

// Resolve the Cloud origin from the bundled native config before the router or
// any API client makes its first request. An unavailable config remains empty;
// the HTTP client then reports that fact instead of guessing a loopback server.
void cloudBaseUrl().then((origin) => {
  setDesktopCloudOrigin(origin)
  app.mount("#app")
})
