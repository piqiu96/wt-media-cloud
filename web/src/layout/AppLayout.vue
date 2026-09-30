<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { isDesktop } from '../utils.js'
import { createSessionClient } from '../shared/api/session.js'
import DownloadCentreDrawer from '../modules/transfer/DownloadCentreDrawer.vue'
import { useDownloadCentre } from '../modules/transfer/downloadCentre.js'

const route = useRoute()
const router = useRouter()
const downloadCentre = useDownloadCentre()
const collapsed = ref(false)
const SIDEBAR_EXPANDED_GROUPS_KEY = 'wt-media:sidebar-expanded-groups'
const expandedGroups = ref(readExpandedGroups())
const sessionClient = createSessionClient()
const currentUser = ref(null)

const menuItems = [
  { title: '工作台', path: '/', icon: 'dashboard' },
  { value: 'discovery', title: '内容挖掘', icon: 'browse', children: [
    { title: '内容池', path: '/content-pool', icon: 'browse' },
    { title: '挖掘策略', path: '/discovery-strategies', icon: 'setting' },
    { title: '挖掘任务', path: '/crawl-tasks', icon: 'time' },
  ] },
  { value: 'production', title: '内容生产', icon: 'browse-gallery', children: [
    { title: '素材库', path: '/material-library', icon: 'browse-gallery' },
    { title: '我的素材', path: '/my-material', icon: 'file-icon' },
    { title: '合成策略', path: '/compose-strategy', icon: 'setting' },
    { title: '合成任务', path: '/compose', icon: 'play-circle' },
    { title: '成片管理', path: '/finished-media', icon: 'video' },
  ] },
  { value: 'operations', title: '运营执行', icon: 'send', children: [
    { title: '发布管理', path: '/publish', icon: 'send' },
    { title: '互动管理', path: '/interact', icon: 'chat' },
    { title: '评论模板', path: '/comment-templates', icon: 'chat-bubble' },
  ] },
  { value: 'analytics', title: '数据分析', icon: 'chart-bar', children: [
    { title: '数据统计', path: '/stats', icon: 'chart-bar' },
  ] },
  { value: 'operation-resources', title: '运营资源', icon: 'folder', children: [
    { title: '浏览器窗口', path: '/browser-windows', icon: 'desktop' },
    { title: '代理管理', path: '/proxies', icon: 'link' },
    { title: '社媒账号', path: '/accounts', icon: 'user' },
  ] },
  { value: 'system', title: '系统', icon: 'setting', children: [
    { title: '用户管理', path: '/users', icon: 'user-setting' },
    { title: '运营分组', path: '/operation-teams', icon: 'view-organization' },
    { title: '游戏管理', path: '/games', icon: 'gamepad' },
  ] },
]

const desktopItems = [
  { value: 'local-environment', title: '本地环境', icon: 'server', children: [
    { title: 'Agent 状态', path: '/agent', icon: 'server' },
    { title: '本地日志', path: '/logs', icon: 'file' },
    { title: '本机设置', path: '/settings', icon: 'setting' },
  ] },
]

const adminOnlyPaths = new Set(['/users', '/operation-teams', '/games'])
const desktopHiddenPaths = new Set(['/users', '/operation-teams', '/games'])

onMounted(async () => {
  try {
    currentUser.value = await sessionClient.me()
  } catch {
    currentUser.value = null
  }
})

const allItems = computed(() => {
  const hiddenPaths = new Set()
  if (isDesktop()) {
    for (const path of desktopHiddenPaths) hiddenPaths.add(path)
  }
  if (currentUser.value?.role !== 'admin') {
    for (const path of adminOnlyPaths) hiddenPaths.add(path)
  }
  const sourceItems = isDesktop() ? [...menuItems, ...desktopItems] : menuItems
  return sourceItems
    .map((item) => {
      if (!item.children) return hiddenPaths.has(item.path) ? null : item
      const children = item.children.filter((child) => !hiddenPaths.has(child.path))
      return children.length > 0 ? { ...item, children } : null
    })
    .filter(Boolean)
})

const breadcrumbItems = computed(() => {
  const group = allItems.value.find((item) => item.children?.some((child) => child.path === route.path))
  if (group) {
    const child = group.children.find((item) => item.path === route.path)
    return child ? [group.title, child.title] : [group.title]
  }
  const item = allItems.value.find((candidate) => candidate.path === route.path)
  return [route.meta?.title || item?.title || String(route.name || '')]
})

watch([() => route.path, allItems], ([path, items]) => {
  const activeGroup = items.find((item) => item.children?.some((child) => child.path === path))
  if (activeGroup && !expandedGroups.value.includes(activeGroup.value)) {
    expandedGroups.value = [...expandedGroups.value, activeGroup.value]
  }
}, { immediate: true })

watch(expandedGroups, (groups) => {
  try {
    localStorage.setItem(SIDEBAR_EXPANDED_GROUPS_KEY, JSON.stringify(groups))
  } catch {
    // Keep navigation usable if browser storage is unavailable.
  }
}, { deep: true })

function readExpandedGroups() {
  try {
    const storedGroups = JSON.parse(localStorage.getItem(SIDEBAR_EXPANDED_GROUPS_KEY) || '[]')
    return Array.isArray(storedGroups) ? storedGroups.filter((group) => typeof group === 'string') : []
  } catch {
    return []
  }
}

function navigate(path) {
  if (path) router.push(path)
}

async function logout() {
  try {
    await sessionClient.logout()
  } finally {
    router.push('/login')
  }
}
</script>

<template>
  <t-layout class="app-shell">
    <t-aside class="app-aside" :width="collapsed ? '64px' : '232px'">
      <div class="sidebar-header" :class="{ 'is-collapsed': collapsed }" @click="router.push('/')">
        <span class="brand-mark" aria-hidden="true">W</span>
        <div v-if="!collapsed" class="brand-copy">
          <span class="brand-kicker">内容运营平台</span>
          <span class="sidebar-title">WT Media</span>
        </div>
      </div>
      <t-menu
        :value="route.path"
        :collapsed="collapsed"
        theme="light"
        v-model:expanded="expandedGroups"
        @change="navigate"
      >
        <template v-for="item in allItems" :key="item.path || item.value">
          <t-submenu v-if="item.children" :value="item.value" :title="item.title">
            <template #icon>
              <t-icon :name="item.icon" />
            </template>
            <t-menu-item v-for="child in item.children" :key="child.path" :value="child.path">
              <template #icon>
                <t-icon :name="child.icon" />
              </template>
              {{ child.title }}
            </t-menu-item>
          </t-submenu>
          <t-menu-item v-else :value="item.path">
            <template #icon>
              <t-icon :name="item.icon" />
            </template>
            {{ item.title }}
          </t-menu-item>
        </template>
      </t-menu>
    </t-aside>

    <t-layout class="app-main">
      <t-header class="topbar">
        <div style="display:flex; align-items:center; gap:12px">
          <t-button variant="text" @click="collapsed = !collapsed">
            <t-icon :name="collapsed ? 'menu-unfold' : 'menu-fold'" />
          </t-button>
          <t-breadcrumb aria-label="当前页面位置">
            <t-breadcrumb-item v-for="item in breadcrumbItems" :key="item">{{ item }}</t-breadcrumb-item>
          </t-breadcrumb>
        </div>
        <div style="display:flex; gap:8px; align-items:center">
          <!--
            下载中心挂在顶栏而不是素材库里：一条下载可以来自素材库、我的素材或日后的
            成片导出，它不属于其中任何一页。面板本身只有一份（modules/transfer），
            页面用 `useDownloadCentre().open()` 把它叫出来。
          -->
          <t-button variant="text" @click="downloadCentre.open">
            <t-icon name="download" />
            下载中心
          </t-button>
          <t-button variant="text" @click="logout">退出</t-button>
        </div>
      </t-header>

      <t-content class="content-area">
        <router-view />
      </t-content>

      <DownloadCentreDrawer />
    </t-layout>
  </t-layout>
</template>

<style scoped>
.app-shell { height: 100%; }
.app-aside {
  height: 100%;
  overflow-y: auto;
  flex-shrink: 0;
  background: #FAFAF8;
  border-right: 1px solid rgba(0, 0, 0, 0.04);
  font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans SC", sans-serif;
  transition: width 0.2s ease;
}
.app-main { height: 100%; min-width: 0; overflow: hidden; }
.sidebar-header {
  height: 76px;
  padding: 0 20px;
  display: flex;
  align-items: center;
  gap: 10px;
  box-sizing: border-box;
  cursor: pointer;
}
.sidebar-header.is-collapsed { justify-content: center; padding: 0; }
.brand-mark {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 28px;
  border-radius: 8px;
  color: #F8FAFC;
  background: #243B53;
  font-family: Georgia, "Songti SC", serif;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.08em;
}
.brand-copy { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.brand-kicker { color: #9CA3AF; font-size: 11px; line-height: 1; letter-spacing: 0.08em; }
.sidebar-title { color: #273142; font-size: 15px; line-height: 1.2; font-weight: 600; letter-spacing: 0.04em; }

:deep(.t-default-menu) {
  width: 100%;
  padding: 4px 0 24px;
  background: transparent;
  font-family: inherit;
  transition: width 0.2s ease;
}
:deep(.t-default-menu .t-menu__item) {
  height: 44px;
  margin: 1px 10px;
  padding: 0 20px;
  border-radius: 8px;
  color: #4B5563;
  font-size: 15px;
  font-weight: 500;
  line-height: 44px;
  transition: background-color 0.2s ease, color 0.2s ease;
}
:deep(.t-default-menu .t-menu__item .t-icon) {
  width: 18px;
  height: 18px;
  margin-right: 10px;
  color: currentColor;
  font-size: 18px;
}
:deep(.t-default-menu .t-menu__item:hover:not(.t-is-active):not(.t-is-disabled)) {
  background: rgba(0, 0, 0, 0.03);
}
:deep(.t-default-menu .t-submenu > .t-menu__item.t-is-opened),
:deep(.t-menu .t-submenu.t-is-active > .t-menu__item),
:deep(.t-menu .t-submenu.t-is-active > .t-menu__item .t-icon) {
  color: #4B5563;
  background: transparent;
}
:deep(.t-default-menu .t-submenu > .t-menu__item:hover) { background: rgba(0, 0, 0, 0.03); }
:deep(.t-default-menu .t-menu__sub) {
  position: relative;
  margin: 2px 20px 7px 30px;
  padding: 2px 0;
  overflow: hidden;
  transition: height 0.2s ease;
}
:deep(.t-default-menu .t-menu__sub::before) {
  content: '';
  position: absolute;
  top: 6px;
  bottom: 6px;
  left: 8px;
  width: 1px;
  background: rgba(75, 85, 99, 0.18);
}
:deep(.t-default-menu .t-menu__sub .t-menu__item) {
  height: 40px;
  margin: 1px 0;
  padding: 0 12px 0 20px;
  color: #6B7280;
  font-size: 14px;
  font-weight: 400;
  line-height: 40px;
}
:deep(.t-default-menu .t-menu__sub .t-menu__item .t-icon) {
  width: 16px;
  height: 16px;
  margin-right: 10px;
  font-size: 16px;
}
:deep(.t-default-menu .t-menu__sub .t-menu__item.t-is-active:not(.t-is-opened)) {
  padding-left: 17px;
  border-left: 3px solid #2563EB;
  border-radius: 8px;
  color: #1D4ED8;
  background: rgba(30, 64, 175, 0.06);
}
:deep(.t-default-menu .t-menu__sub .t-menu__item.t-is-active .t-icon) { color: #1D4ED8; }
:deep(.t-default-menu .t-submenu .t-submenu-icon) {
  width: 14px;
  height: 14px;
  opacity: 0.5;
  transition: transform 0.2s ease;
}
:deep(.t-default-menu .t-submenu.t-is-opened .t-submenu-icon) { transform: rotate(180deg); }
:deep(.t-default-menu.t-is-collapsed .t-menu .t-menu__item) { margin: 1px 10px; padding: 0; }
:deep(.t-default-menu.t-is-collapsed .t-menu__item .t-icon) { margin-right: 0; }
.topbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 24px; background: #FFFDFC;
  border-bottom: 1px solid rgba(0, 0, 0, 0.04); height: 52px;
  flex-shrink: 0;
}
.content-area {
  flex: 1; overflow: auto; min-width: 0; min-height: 0;
  padding: 24px; background: var(--td-bg-color-page);
}
.content-area > * { min-width: 0; max-width: 100%; }
</style>
