import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue') },
  {
    path: '/',
    component: () => import('../layout/AppLayout.vue'),
    children: [
      { path: '', name: 'Dashboard', component: () => import('../views/Dashboard.vue') },
      { path: 'discovery', name: 'Discovery', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'material-library', name: 'MaterialLibrary', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'my-material', name: 'MyMaterial', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'compose', name: 'Compose', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'finished-media', name: 'FinishedMedia', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'publish', name: 'Publish', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'interact', name: 'Interact', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'execute-tasks', name: 'ExecuteTasks', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'stats', name: 'Stats', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'accounts', name: 'Accounts', component: () => import('../views/Accounts.vue') },
      { path: 'browser-users', name: 'BrowserUsers', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'compose-strategy', name: 'ComposeStrategy', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'comment-templates', name: 'CommentTemplates', component: () => import('../views/placeholders/ComingSoon.vue') },
      { path: 'users', name: 'Users', component: () => import('../views/Users.vue') },
      { path: 'agent', name: 'AgentStatus', component: () => import('../views/AgentStatus.vue'), meta: { desktopOnly: true } },
      { path: 'logs', name: 'LocalLogs', component: () => import('../views/LocalLogs.vue'), meta: { desktopOnly: true } },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Redirect /login → / if already authenticated
router.beforeEach((to, from, next) => {
  if (to.path === '/login') {
    // Let login page handle its own auth check
  }
  next()
})

export default router
