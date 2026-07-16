// Desktop 路由 — 含业务页面 + Desktop 独有页面，不含 Cloud 管理页面
import { createRouter, createWebHistory } from "vue-router"

export function createDesktopRouter() {
  const routes = [
    { path: "/login", name: "Login", component: () => import("../../modules/auth/pages/LoginPage.vue") },
    {
      path: "/",
      component: () => import("../../layout/AppLayout.vue"),
      children: [
        { path: "", name: "Dashboard", component: () => import("../../modules/dashboard/pages/DashboardPage.vue") },
        { path: "discovery", name: "Discovery", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "material-library", name: "MaterialLibrary", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "my-material", name: "MyMaterial", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "compose", name: "Compose", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "finished-media", name: "FinishedMedia", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "publish", name: "Publish", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "interact", name: "Interact", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "execute-tasks", name: "ExecuteTasks", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "stats", name: "Stats", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "accounts", name: "Accounts", component: () => import("../../modules/accounts/pages/AccountsPage.vue") },
        { path: "account-opening", name: "AccountOpening", component: () => import("../../modules/accounts/pages/AccountOpeningPage.vue") },
        { path: "browser-users", name: "BrowserUsers", component: () => import("../../modules/profiles/pages/ProfilesPage.vue") },
        { path: "compose-strategy", name: "ComposeStrategy", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "comment-templates", name: "CommentTemplates", component: () => import("../../shared/ui/ComingSoon.vue") },
        { path: "proxies", name: "Proxies", component: () => import("../../modules/proxy/pages/ProxyPage.vue") },
        // Desktop 独有页面
        { path: "agent", name: "AgentStatus", component: () => import("./features/local-agent/AgentStatusPage.vue") },
        { path: "logs", name: "LocalLogs", component: () => import("./features/local-logs/LocalLogsPage.vue") },
        // 不包含 /users — Cloud 管理页面
      ],
    },
  ]

  const router = createRouter({
    history: createWebHistory(),
    routes,
  })

  return router
}
