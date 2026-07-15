import { createApp } from 'vue'
import { createPinia } from 'pinia'
import TDesign from 'tdesign-vue-next'
import router from './router/index.js'

import App from './App.vue'
import 'tdesign-vue-next/es/style/index.css'

const app = createApp(App)
app.use(createPinia())
app.use(TDesign)
app.use(router)

// Global auth guard
router.beforeEach(async (to, from, next) => {
  if (to.path === '/login') {
    next()
    return
  }

  // Desktop-only route guard
  if (to.meta?.desktopOnly && typeof window !== 'undefined' && !window.__TAURI_INTERNALS__) {
    next('/')
    return
  }

  // Auth check: if not logged in, redirect to login
  try {
    const { createSessionClient } = await import('./session.js')
    await createSessionClient().me()
    next()
  } catch {
    next('/login')
  }
})

app.mount('#app')
