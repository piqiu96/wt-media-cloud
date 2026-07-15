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
    await loadTasks()
  } catch (requestError) {
    error.value = requestError.message
  }
}

async function loadTasks() {
  taskLoading.value = true
  error.value = ''
  try {
    const task = await taskClient.get('')
    // Single task lookup not available for listing; user creates and sees results.
  } catch {
    // Task list endpoint not available yet; created tasks shown from create response.
  } finally {
    taskLoading.value = false
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
</script>

<template>
  <main class="shell">
    <section v-if="loading" class="card compact">正在检查会话…</section>
    <section v-else-if="!user" class="card login-card">
      <p class="eyebrow">WT MEDIA CLOUD</p>
      <h1>登录运营平台</h1>
      <form @submit.prevent="login">
        <label>用户名<input v-model.trim="username" autocomplete="username" required /></label>
        <label>密码<input v-model="password" type="password" autocomplete="current-password" required /></label>
        <p v-if="error" class="error">{{ error }}</p>
        <button type="submit">登录</button>
      </form>
    </section>

    <section v-else class="workspace">
      <header class="topbar">
        <div>
          <p class="eyebrow">媒体账号工作台</p>
          <h1>{{ user.username }}</h1>
          <p class="muted">{{ user.role }} · {{ user.game_ids.length ? user.game_ids.join('、') : '全局范围' }}</p>
        </div>
        <button class="secondary" type="button" @click="logout">退出登录</button>
      </header>

      <p v-if="error" class="error banner">{{ error }}</p>

      <div class="columns">
        <section class="card create-card">
          <p class="eyebrow">新增账号</p>
          <h2>创建待识别记录</h2>
          <form @submit.prevent="createAccount">
            <label v-if="user.role === 'technician'">目标用户 ID<input v-model.trim="targetUserId" placeholder="留空则归当前用户" /></label>
            <label>游戏 ID<input v-model.trim="gameId" required /></label>
            <label>平台
              <select v-model="platform">
                <option value="douyin">抖音</option>
                <option value="bilibili">B站</option>
                <option value="baijiahao">百家号</option>
              </select>
            </label>
            <label>原始 Cookie（可选）<textarea v-model="originalCookie" rows="4" autocomplete="off" /></label>
            <p class="hint">Cookie 仅随本次请求提交，不在页面或浏览器存储中保留。</p>
            <button type="submit">创建账号</button>
          </form>
        </section>

        <section class="card list-card">
          <div class="section-heading">
            <div><p class="eyebrow">账号列表</p><h2>{{ accounts.length }} 个账号</h2></div>
            <button class="secondary" type="button" @click="loadAccounts">刷新</button>
          </div>

          <div class="tag-tools">
            <input v-model.trim="tagInput" placeholder="标签，多个用逗号分隔" />
            <button type="button" @click="changeTags(false)">添加标签</button>
            <button class="secondary" type="button" @click="changeTags(true)">移除标签</button>
          </div>

          <p v-if="accountLoading" class="muted">正在加载账号…</p>
          <p v-else-if="!accounts.length" class="empty">还没有媒体账号。先创建一条待识别记录。</p>
          <div v-else class="account-list">
            <label v-for="account in accounts" :key="account.id" class="account-row">
              <input v-model="selectedAccountIds" type="checkbox" :value="account.id" />
              <span class="account-main">
                <strong>{{ account.name || account.platform_account_id || '待识别账号' }}</strong>
                <small>{{ account.platform }} · {{ account.game_id }} · {{ account.user_id }}</small>
                <span class="badges">
                  <em>{{ account.identification_status }}</em>
                  <em>{{ account.business_status }}</em>
                  <em v-for="tag in account.tags" :key="tag" class="tag">{{ tag }}</em>
                </span>
              </span>
            </label>
          </div>
        </section>
      </div>

      <!-- Task management section -->
      <section class="card task-section">
        <div class="section-heading">
          <div><p class="eyebrow">任务管理</p><h2>创建和查看任务</h2></div>
        </div>
        <div class="task-tools">
          <select v-model="newTaskType">
            <option value="noop_task">验证任务 (noop)</option>
          </select>
          <button type="button" @click="createTask">创建任务</button>
          <button v-if="tasks.length" class="secondary" type="button" @click="cancelLastTask">取消最近任务</button>
        </div>
        <p v-if="taskLoading" class="muted">加载中…</p>
        <div v-else-if="!tasks.length" class="empty">还没有创建任务。点击"创建任务"开始。</div>
        <div v-else class="task-list">
          <div v-for="task in tasks" :key="task.task_id" class="task-row" :class="task.status">
            <span class="task-id">{{ task.task_id.slice(0, 20) }}…</span>
            <span class="task-type">{{ task.task_type }}</span>
            <span class="task-status" :class="task.status">{{ task.status }}</span>
            <span v-if="task.progress" class="task-progress">{{ task.progress }}%</span>
            <span v-if="task.message" class="task-message">{{ task.message }}</span>
          </div>
        </div>
      </section>
    </section>
  </main>
</template>

<style>
:root { font-family: Inter, ui-sans-serif, system-ui, sans-serif; color: #13231b; background: #edf3ef; }
* { box-sizing: border-box; }
body { margin: 0; }
button, input, select, textarea { font: inherit; }
.shell { min-height: 100vh; padding: 32px; }
.card { padding: 30px; border: 1px solid #c9d7cf; border-radius: 18px; background: #fff; box-shadow: 0 18px 50px rgba(26, 56, 39, .08); }
.compact, .login-card { width: min(440px, 100%); margin: 10vh auto 0; }
.workspace { width: min(1180px, 100%); margin: 0 auto; }
.topbar, .section-heading { display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.topbar { margin-bottom: 28px; }
.columns { display: grid; grid-template-columns: minmax(280px, 360px) minmax(0, 1fr); gap: 24px; align-items: start; }
.eyebrow { margin: 0 0 8px; color: #387255; font-size: 12px; font-weight: 750; letter-spacing: .14em; }
h1, h2 { margin: 0; }
h1 { font-size: 28px; }
h2 { font-size: 21px; }
form { display: grid; gap: 18px; margin-top: 24px; }
label { display: grid; gap: 8px; font-size: 14px; font-weight: 650; }
input, select, textarea { width: 100%; padding: 11px 13px; border: 1px solid #aebfb5; border-radius: 10px; background: #fff; color: inherit; }
textarea { resize: vertical; }
button { padding: 11px 16px; border: 0; border-radius: 10px; background: #1d6a45; color: #fff; font-weight: 700; cursor: pointer; }
.secondary { background: #e5eee9; color: #174b34; }
.muted, .hint, small { color: #617067; }
.muted { margin: 6px 0 0; }
.hint { margin: -8px 0 0; font-size: 12px; line-height: 1.5; }
.error { margin: 0; color: #a12626; font-size: 14px; }
.banner { margin-bottom: 18px; padding: 12px 14px; border-radius: 10px; background: #fff0f0; }
.tag-tools { display: grid; grid-template-columns: minmax(180px, 1fr) auto auto; gap: 10px; margin: 22px 0; }
.account-list { display: grid; gap: 10px; }
.account-row { grid-template-columns: 20px minmax(0, 1fr); align-items: start; padding: 14px; border: 1px solid #dce6e0; border-radius: 12px; }
.account-row > input { margin-top: 3px; }
.account-main { display: grid; gap: 5px; }
.badges { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 4px; }
.badges em { padding: 3px 7px; border-radius: 999px; background: #eef3f0; color: #4b5f54; font-size: 11px; font-style: normal; }
.badges .tag { background: #dff0e7; color: #195c3b; }
.empty { padding: 36px 18px; border: 1px dashed #b8c9bf; border-radius: 12px; color: #617067; text-align: center; }
.task-section { margin-top: 24px; }
.task-tools { display: flex; gap: 10px; margin: 18px 0; flex-wrap: wrap; }
.task-tools select { min-width: 180px; }
.task-list { display: grid; gap: 8px; }
.task-row { display: flex; gap: 12px; align-items: center; padding: 10px 14px; border: 1px solid #dce6e0; border-radius: 10px; font-size: 13px; }
.task-id { font-family: monospace; color: #387255; min-width: 120px; }
.task-type { color: #617067; min-width: 80px; }
.task-status { font-weight: 700; min-width: 80px; }
.task-status.pending { color: #b8860b; }
.task-status.leased { color: #2563eb; }
.task-status.running { color: #1d6a45; }
.task-status.succeeded { color: #166534; }
.task-status.failed { color: #a12626; }
.task-status.cancelled { color: #617067; }
.task-progress { min-width: 40px; color: #2563eb; font-weight: 650; }
.task-message { color: #617067; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

@media (max-width: 820px) {
  .shell { padding: 18px; }
  .columns { grid-template-columns: 1fr; }
  .tag-tools { grid-template-columns: 1fr; }
}
</style>
