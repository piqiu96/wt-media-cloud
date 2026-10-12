// Desktop 路由 — 含业务页面 + Desktop 独有页面，不含 Cloud 管理页面
import { createRouter, createWebHistory } from "vue-router"

// The table is exported rather than built inside `createDesktopRouter`, because
// building a router needs a browser history and the routes do not. A page's
// wiring — its path, its name, and that its component file is where it says it
// is — is then checkable without a DOM, which is the only environment the test
// runner here has (`localSettingsWiring.test.js`).
export const desktopRoutes = [
  { path: "/login", name: "Login", component: () => import("../../modules/auth/pages/LoginPage.vue") },
  {
    path: "/",
    component: () => import("../../layout/AppLayout.vue"),
    children: [
      { path: "", name: "Dashboard", component: () => import("../../modules/dashboard/pages/DashboardPage.vue") },
      { path: "personal-info", name: "PersonalInfo", component: () => import("../../modules/auth/pages/PersonalInfoPage.vue"), meta: { title: '个人信息' } },
      { path: "discovery", redirect: "/content-pool" },
      { path: "content-pool", name: "ContentPool", component: () => import("../../modules/contentpool/pages/ContentPoolPage.vue") },
      { path: "discovery-strategies", name: "DiscoveryStrategies", component: () => import("../../modules/contentpool/pages/DiscoveryStrategiesPage.vue") },
      { path: "crawl-tasks", name: "CrawlTasks", component: () => import("../../modules/contentpool/pages/CrawlTasksPage.vue") },
      { path: "material-library", name: "MaterialLibrary", component: () => import("../../modules/materials/pages/MaterialLibraryPage.vue") },
      { path: "my-material", name: "MyMaterial", component: () => import("../../modules/materials/pages/MyMaterialsPage.vue") },
      { path: "compose-strategy", name: "ComposeStrategy", component: () => import("../../shared/ui/ComingSoon.vue") },
      { path: "compose", name: "Compose", component: () => import("../../shared/ui/ComingSoon.vue") },
      { path: "finished-media", name: "FinishedMedia", component: () => import("../../shared/ui/ComingSoon.vue") },
      { path: "publish", name: "Publish", component: () => import("../../shared/ui/ComingSoon.vue") },
      { path: "interact", name: "Interact", component: () => import("../../shared/ui/ComingSoon.vue") },
      { path: "comment-templates", name: "CommentTemplates", component: () => import("../../shared/ui/ComingSoon.vue") },
      { path: "stats", name: "Stats", component: () => import("../../shared/ui/ComingSoon.vue") },
      { path: "accounts", name: "Accounts", component: () => import("../../modules/accounts/pages/AccountsPage.vue") },
      { path: "account-opening", name: "AccountOpening", component: () => import("../../modules/accounts/pages/AccountOpeningPage.vue") },
      { path: "browser-windows", name: "BrowserWindows", component: () => import("../../modules/profiles/pages/ProfilesPage.vue") },
      { path: "proxies", name: "Proxies", component: () => import("../../modules/proxy/pages/ProxyPage.vue") },
      // Desktop 独有页面
      { path: "logs", name: "LocalLogs", component: () => import("./features/local-logs/LocalLogsPage.vue") },
      { path: "settings", name: "LocalSettings", component: () => import("./features/local-settings/LocalSettingsPage.vue") },
      // 不包含 /users — Cloud 管理页面
    ],
  },
]

export function createDesktopRouter() {
  const router = createRouter({
    history: createWebHistory(),
    routes: desktopRoutes,
  })

  return router
}
