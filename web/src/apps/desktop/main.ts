import { createApp } from "vue"
import { createPinia } from "pinia"
import TDesign from "tdesign-vue-next"
import { createDesktopRouter } from "./router"
import App from "./App.vue"
import "tdesign-vue-next/es/style/index.css"

const app = createApp(App)
const pinia = createPinia()
const router = createDesktopRouter()

app.use(pinia)
app.use(TDesign)
app.use(router)

// Auth guard
router.beforeEach(async (to, from, next) => {
  if (to.path === "/login") {
    next()
    return
  }

  // Desktop 本地页面不需要认证
  if (to.name === "AgentStatus" || to.name === "LocalLogs") {
    next()
    return
  }

  try {
    const { createSessionClient } = await import("../../shared/api/session.js")
    await createSessionClient().me()
    next()
  } catch {
    next("/login")
  }
})

app.mount("#app")
