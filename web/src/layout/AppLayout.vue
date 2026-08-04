<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { isDesktop } from '../utils.js'
import { createSessionClient } from '../shared/api/session.js'

const route = useRoute()
const router = useRouter()
const collapsed = ref(false)
const sessionClient = createSessionClient()
const currentUser = ref(null)

const menuItems = [
  { title: '工作台', path: '/', icon: 'dashboard' },

  { group: '内容发现' },
  { title: '内容发现', path: '/discovery', icon: 'browse' },

  { group: '内容生产' },
  { title: '素材库', path: '/material-library', icon: 'gallery' },
  { title: '我的素材', path: '/my-material', icon: 'file-icon' },
  { title: '合成任务', path: '/compose', icon: 'play-circle' },
  { title: '成片管理', path: '/finished-media', icon: 'video' },

  { group: '运营执行' },
  { title: '发布管理', path: '/publish', icon: 'send' },
  { title: '互动管理', path: '/interact', icon: 'chat' },
  { title: '执行任务', path: '/execute-tasks', icon: 'check-circle' },

  { group: '数据分析' },
  { title: '数据统计', path: '/stats', icon: 'chart-bar' },

  { group: '资源管理' },
  { title: '社媒账号', path: '/accounts', icon: 'user' },
  { title: '账号开户', path: '/account-opening', icon: 'add' },
  { title: '代理管理', path: '/proxies', icon: 'link' },
  { title: '浏览器窗口', path: '/browser-windows', icon: 'desktop' },
  { title: '合成策略', path: '/compose-strategy', icon: 'setting' },
  { title: '评论模板', path: '/comment-templates', icon: 'comment' },

  { group: '系统' },
  { title: '用户管理', path: '/users', icon: 'user-setting' },
  { title: '运营分组', path: '/operation-teams', icon: 'organization' },
  { title: '游戏管理', path: '/games', icon: 'gamepad' },
]

const desktopItems = [
  { group: '本地环境' },
  { title: 'Agent 状态', path: '/agent', icon: 'server' },
  { title: '本地日志', path: '/logs', icon: 'file' },
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
  return removeEmptyGroups(sourceItems.filter((item) => !item.path || !hiddenPaths.has(item.path)))
})

function removeEmptyGroups(items) {
  const result = []
  for (let index = 0; index < items.length; index += 1) {
    const item = items[index]
    if (item.group && !item.path) {
      let hasVisibleChild = false
      for (let nextIndex = index + 1; nextIndex < items.length; nextIndex += 1) {
        const next = items[nextIndex]
        if (next.group && !next.path) break
        if (next.path) {
          hasVisibleChild = true
          break
        }
      }
      if (!hasVisibleChild) continue
    }
    result.push(item)
  }
  return result
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
      <div class="sidebar-header" @click="router.push('/')">
        <span v-if="!collapsed" class="sidebar-title">WT Media</span>
        <span v-else class="sidebar-title-mini">W</span>
      </div>
      <t-menu
        :value="route.path"
        :collapsed="collapsed"
        theme="light"
        @change="navigate"
      >
        <template v-for="item in allItems" :key="item.path || item.group">
          <t-menu-group v-if="!item.path && item.group" :title="collapsed ? '' : item.group" />
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
          <t-breadcrumb>
            <t-breadcrumb-item>{{ route.meta?.title || route.name }}</t-breadcrumb-item>
          </t-breadcrumb>
        </div>
        <div style="display:flex; gap:8px">
          <t-button variant="text" @click="logout">退出</t-button>
        </div>
      </t-header>

      <t-content class="content-area">
        <router-view />
      </t-content>
    </t-layout>
  </t-layout>
</template>

<style scoped>
.app-shell { height: 100%; }
.app-aside { height: 100%; overflow-y: auto; flex-shrink: 0; }
.app-main { height: 100%; min-width: 0; overflow: hidden; }
.sidebar-header {
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border-bottom: 1px solid var(--td-component-stroke);
}
.sidebar-title { font-weight: 700; font-size: 18px; letter-spacing: 0.04em; }
.sidebar-title-mini { font-weight: 700; font-size: 20px; }
.topbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 24px; background: var(--td-bg-color-container);
  border-bottom: 1px solid var(--td-component-stroke); height: 48px;
  flex-shrink: 0;
}
.content-area {
  flex: 1; overflow: auto; min-width: 0; min-height: 0;
  padding: 24px; background: var(--td-bg-color-page);
}
.content-area > * { min-width: 0; max-width: 100%; }
</style>
