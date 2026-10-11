import { createApp } from "vue"
import { createPinia } from "pinia"
import TDesign from "tdesign-vue-next"
import { createCloudRouter } from "./router"
import App from "../../App.vue"
import { isPublicCloudRoute } from "./publicRoutes.js"
import "tdesign-vue-next/es/style/index.css"
import "../../shared/styles/layout.css"
import "../../styles/design-token.css"
import "../../shared/styles/resource-module.css"

const app = createApp(App)
const pinia = createPinia()
const router = createCloudRouter()
const adminOnlyPaths = new Set(["/users", "/operation-teams", "/games"])
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
    if (adminOnlyPaths.has(to.path) && user.role !== "admin") {
      next("/")
      return
    }
    next()
  } catch {
    next("/login")
  }
})

app.mount("#app")
