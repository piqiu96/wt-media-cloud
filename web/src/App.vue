<script setup>
import { onMounted, ref } from 'vue'

import { createMediaAccountClient } from './mediaAccounts.js'
import { createSessionClient } from './session.js'
import { createTaskClient } from './tasks.js'

const sessionClient = createSessionClient()
const accountClient = createMediaAccountClient()
const taskClient = createTaskClient()
const username = ref('')
const password = ref('')
const user = ref(null)
const loading = ref(true)
const error = ref('')
const accounts = ref([])
const accountLoading = ref(false)
const gameId = ref('')
const targetUserId = ref('')
const platform = ref('douyin')
const originalCookie = ref('')
const selectedAccountIds = ref([])
const tagInput = ref('')

onMounted(async () => {
  try {
    user.value = await sessionClient.me()
    gameId.value = (user.value.game_ids || [])[0] || ''
    await loadAccounts()
  } catch {
    user.value = null
  } finally {
    loading.value = false
  }
})

async function login() {
  error.value = ''
  try {
    user.value = await sessionClient.login(username.value, password.value)
    password.value = ''
    gameId.value = (user.value.game_ids || [])[0] || ''
    await loadAccounts()
  } catch (requestError) {
    error.value = requestError.message
  }
}

async function logout() {
  await sessionClient.logout()
  user.value = null
  accounts.value = []
  selectedAccountIds.value = []
}

async function loadAccounts() {
  accountLoading.value = true
  error.value = ''
  try {
    accounts.value = await accountClient.list()
  } catch (requestError) {
    error.value = requestError.message
  } finally {
    accountLoading.value = false
  }
}

async function createAccount() {
  error.value = ''
  try {
    await accountClient.create({
      userId: user.value.role === 'technician' ? targetUserId.value : '',
      gameId: gameId.value,
      platform: platform.value,
      originalCookie: originalCookie.value,
    })
    originalCookie.value = ''
    await loadAccounts()
  } catch (requestError) {
    error.value = requestError.message
  }
}

async function changeTags(remove = false) {
  const tags = tagInput.value.split(',').map((tag) => tag.trim()).filter(Boolean)
  if (!selectedAccountIds.value.length || !tags.length) return
  error.value = ''
  try {
    if (remove) await accountClient.removeTags(selectedAccountIds.value, tags)
    else await accountClient.addTags(selectedAccountIds.value, tags)
    tagInput.value = ''
    await loadAccounts()
  } catch (requestError) {
    error.value = requestError.message
  }
}

// Task management
const tasks = ref([])
const taskLoading = ref(false)
const newTaskType = ref('noop_task')

async function createTask() {
  error.value = ''
  try {
    const task = await taskClient.create(newTaskType.value)
    tasks.value.unshift(task)
  } catch (requestError) {
    error.value = requestError.message
  }
}

async function cancelLastTask() {
  if (!tasks.value.length) return
  error.value = ''
  try {
    await taskClient.cancel(tasks.value[0].task_id, 'operator cancelled from web')
    tasks.value[0].status = 'cancelled'
  } catch (requestError) {
    error.value = requestError.message
  }
}

const statusMap = {
  pending: { label: '待执行', theme: 'warning' },
  leased: { label: '已领取', theme: 'primary' },
  running: { label: '执行中', theme: 'primary' },
  succeeded: { label: '已完成', theme: 'success' },
  failed: { label: '失败', theme: 'danger' },
  cancelled: { label: '已取消', theme: 'default' },
}

const accountColumns = [
  { colKey: 'name', title: '账号', width: 200 },
  { colKey: 'platform', title: '平台', width: 100 },
  { colKey: 'game_id', title: '游戏', width: 120 },
  { colKey: 'identification_status', title: '识别状态', width: 120 },
  { colKey: 'business_status', title: '业务状态', width: 120 },
]

const taskColumns = [
  { colKey: 'task_id', title: '任务 ID', width: 200 },
  { colKey: 'task_type', title: '类型', width: 100 },
  { colKey: 'status', title: '状态', width: 120 },
  { colKey: 'progress', title: '进度', width: 100 },
  { colKey: 'message', title: '消息' },
]
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <div v-if="!user" class="login-wrapper">
      <t-card :bordered="true" class="login-card">
        <template #title>
          <span style="font-weight: 700; letter-spacing: 0.04em">WT MEDIA CLOUD</span>
        </template>
        <template #subtitle>
          <span style="color: var(--td-text-color-secondary)">登录运营平台</span>
        </template>
        <t-form @submit="login">
          <t-form-item label="用户名">
            <t-input v-model="username" placeholder="请输入用户名" autocomplete="username" />
          </t-form-item>
          <t-form-item label="密码">
            <t-input v-model="password" type="password" placeholder="请输入密码" autocomplete="current-password" />
          </t-form-item>
          <t-form-item v-if="error">
            <t-alert :message="error" theme="error" />
          </t-form-item>
          <t-form-item>
            <t-button type="submit" theme="primary" block>登录</t-button>
          </t-form-item>
        </t-form>
      </t-card>
    </div>

    <div v-else class="workspace-wrapper">
      <t-layout>
        <t-header class="topbar">
          <div>
            <h2 style="margin:0">{{ user.username }}</h2>
            <p style="margin:4px 0 0; color: var(--td-text-color-secondary); font-size: 12px">
              {{ user.role }}
              <template v-if="user.game_ids?.length"> · {{ user.game_ids.join('、') }}</template>
              <template v-else> · 全局范围</template>
            </p>
          </div>
          <div style="display:flex; gap:8px">
            <t-button theme="default" @click="createTask">创建测试任务</t-button>
            <t-button theme="default" @click="logout">退出登录</t-button>
          </div>
        </t-header>

        <t-content style="padding: 24px">
          <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />

          <t-tabs default-value="accounts" style="margin-bottom:24px">
            <t-tab-panel value="accounts" label="账号管理" destroy-on-hide>
              <div style="display:grid; grid-template-columns: 360px 1fr; gap:24px">
                <t-card title="新增账号" :bordered="true">
                  <t-form @submit="createAccount">
                    <t-form-item v-if="user.role === 'technician'" label="目标用户 ID">
                      <t-input v-model="targetUserId" placeholder="留空则归当前用户" />
                    </t-form-item>
                    <t-form-item label="游戏 ID">
                      <t-input v-model="gameId" />
                    </t-form-item>
                    <t-form-item label="平台">
                      <t-select v-model="platform">
                        <t-option value="douyin" label="抖音" />
                        <t-option value="bilibili" label="B站" />
                        <t-option value="baijiahao" label="百家号" />
                      </t-select>
                    </t-form-item>
                    <t-form-item label="原始 Cookie（可选）">
                      <t-textarea v-model="originalCookie" :rows="3" placeholder="Cookie 仅随本次请求提交" />
                    </t-form-item>
                    <t-form-item>
                      <t-button type="submit" theme="primary">创建账号</t-button>
                    </t-form-item>
                  </t-form>
                </t-card>

                <t-card :title="`账号列表 (${accounts.length})`" :bordered="true">
                  <template #actions>
                    <t-button theme="default" size="small" @click="loadAccounts">刷新</t-button>
                  </template>
                  <div style="display:flex; gap:8px; margin-bottom:16px">
                    <t-input v-model="tagInput" placeholder="标签，多个用逗号分隔" />
                    <t-button size="small" @click="changeTags(false)">添加标签</t-button>
                    <t-button size="small" theme="default" @click="changeTags(true)">移除标签</t-button>
                  </div>
                  <t-table
                    v-if="accounts.length"
                    :data="accounts"
                    :columns="accountColumns"
                    :selected-row-keys="selectedAccountIds"
                    :row-key="(r) => r.id"
                    @select-change="(keys) => { selectedAccountIds = keys }"
                    size="small"
                    hover
                  >
                    <template #name="{ row }">
                      <div>
                        <div>{{ row.name || row.platform_account_id || '待识别' }}</div>
                        <div style="display:flex; gap:4px; margin-top:4px">
                          <t-tag v-for="tag in row.tags || []" :key="tag" size="small" theme="primary" variant="light">{{ tag }}</t-tag>
                        </div>
                      </div>
                    </template>
                    <template #identification_status="{ row }">
                      <t-tag :theme="row.identification_status === 'identified' ? 'success' : 'warning'" size="small">{{ row.identification_status }}</t-tag>
                    </template>
                    <template #business_status="{ row }">
                      <t-tag :theme="row.business_status === 'active' ? 'success' : 'default'" size="small">{{ row.business_status }}</t-tag>
                    </template>
                  </t-table>
                  <t-empty v-else description="还没有媒体账号" />
                </t-card>
              </div>
            </t-tab-panel>

            <t-tab-panel value="tasks" label="任务管理" destroy-on-hide>
              <t-card title="创建和查看任务" :bordered="true">
                <div style="display:flex; gap:12px; margin-bottom:16px">
                  <t-select v-model="newTaskType" style="width:200px">
                    <t-option value="noop_task" label="验证任务 (noop)" />
                  </t-select>
                  <t-button @click="createTask">创建任务</t-button>
                  <t-button v-if="tasks.length" theme="default" @click="cancelLastTask">取消最近任务</t-button>
                </div>

                <t-table
                  v-if="tasks.length"
                  :data="tasks"
                  :columns="taskColumns"
                  size="small"
                  hover
                >
                  <template #status="{ row }">
                    <t-tag :theme="(statusMap[row.status] || {}).theme || 'default'" size="small">
                      {{ (statusMap[row.status] || {}).label || row.status }}
                    </t-tag>
                  </template>
                  <template #progress="{ row }">
                    <t-progress v-if="row.progress" :percentage="row.progress" :stroke-width="8" />
                    <span v-else style="color:var(--td-text-color-secondary)">-</span>
                  </template>
                </t-table>
                <t-empty v-else description="还没有创建任务" />
              </t-card>
            </t-tab-panel>
          </t-tabs>
        </t-content>
      </t-layout>
    </div>
  </t-loading>
</template>

<style>
body { margin: 0; background: var(--td-bg-color-page); }
.login-wrapper { display: flex; justify-content: center; align-items: center; min-height: 100vh; padding: 24px; }
.login-card { width: 400px; }
.workspace-wrapper { min-height: 100vh; }
.topbar { display: flex; justify-content: space-between; align-items: center; padding: 16px 24px; background: var(--td-bg-color-container); border-bottom: 1px solid var(--td-component-stroke); }
</style>
