import { createApp } from "vue"
import { createPinia } from "pinia"
import TDesign from "tdesign-vue-next"
import { createCloudRouter } from "./router"
import App from "../../App.vue"
import { isPublicCloudRoute } from "./publicRoutes.js"
import { isSystemManagementPath, canViewSystemPath } from "../../shared/utils/roles.js"
import "tdesign-vue-next/es/style/index.css"
import "../../shared/styles/layout.css"
import "../../styles/design-token.css"
import "../../shared/styles/resource-module.css"

const app = createApp(App)
const pinia = createPinia()
const router = createCloudRouter()
;(window as any).__WT_MEDIA_APP__ = "cloud"

app.use(pinia)
app.use(TDesign)
app.use(router)

// Auth guard
router.beforeEach(async (to, from, next) => {
  if (isPublicCloudRoute(to.path)) {
    next()
    return
  }

  try {
    const { createSessionClient } = await import("../../shared/api/session.js")
    const user = await createSessionClient().me()
    // 用户管理仅管理员；运营团队 / 游戏管理对高级运营只读开放。
    if (isSystemManagementPath(to.path) && !canViewSystemPath(user.role, to.path)) {
      next("/")
      return
    }
    next()
  } catch {
    next("/login")
  }
})

app.mount("#app")
