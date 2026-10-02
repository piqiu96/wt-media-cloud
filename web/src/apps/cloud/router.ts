// Cloud 路由 — 不含 Desktop 独有页面
import { createRouter, createWebHistory } from "vue-router"

// 路由表导出而不是在 `createCloudRouter` 里现搭，理由与 Desktop 那张表相同：搭路由需要一个
// 浏览器 history，而路由表本身不需要（`apps/desktop/router.ts` 早就这么做并写了理由）。
//
// Cloud 这一侧的额外收益是「Cloud Web 与 Desktop WebView 复用同一批页面」这条跨端不变量
// 终于可查了 —— 它是靠导入两张表比对出来的，而不是靠两句注释各说一遍
// （`src/apps/routerParity.test.js`）。两棵树如果各自指向不同的组件文件，那句话就是假的，
// 而在此之前没有任何东西会发现。
export const cloudRoutes = [
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
      { path: "users", name: "Users", component: () => import("./pages/users/UsersPage.vue") },
      { path: "operation-teams", name: "OperationTeams", component: () => import("./pages/users/TeamsPage.vue") },
      { path: "games", name: "Games", component: () => import("./pages/users/GamesPage.vue") },
      // 不包含 /logs, /settings — 这些是 Desktop-only
    ],
  },
]

export function createCloudRouter() {
  const router = createRouter({
    history: createWebHistory(),
    routes: cloudRoutes,
  })

  return router
}
