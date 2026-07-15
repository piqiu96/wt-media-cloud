<script setup>
import { onMounted, ref } from 'vue'
import { createMediaAccountClient } from '../mediaAccounts.js'
import { createSessionClient } from '../session.js'

const sessionClient = createSessionClient()
const accountClient = createMediaAccountClient()
const user = ref(null)
const accounts = ref([])
const loading = ref(true)
const error = ref('')
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

async function loadAccounts() {
  error.value = ''
  try {
    accounts.value = await accountClient.list()
  } catch (e) {
    error.value = e.message
  }
}

async function createAccount() {
  error.value = ''
  try {
    await accountClient.create({
      userId: user.value?.role === 'technician' ? targetUserId.value : '',
      gameId: gameId.value,
      platform: platform.value,
      originalCookie: originalCookie.value,
    })
    originalCookie.value = ''
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  }
}

async function changeTags(remove = false) {
  const tags = tagInput.value.split(',').map(t => t.trim()).filter(Boolean)
  if (!selectedAccountIds.value.length || !tags.length) return
  error.value = ''
  try {
    if (remove) await accountClient.removeTags(selectedAccountIds.value, tags)
    else await accountClient.addTags(selectedAccountIds.value, tags)
    tagInput.value = ''
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  }
}

const columns = [
  { colKey: 'name', title: '账号', width: 200 },
  { colKey: 'platform', title: '平台', width: 100 },
  { colKey: 'game_id', title: '游戏', width: 120 },
  { colKey: 'identification_status', title: '识别状态', width: 120 },
  { colKey: 'business_status', title: '业务状态', width: 120 },
]
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />

    <div style="display:grid; grid-template-columns: 360px 1fr; gap:24px">
      <t-card title="新增账号" :bordered="true">
        <t-form @submit="createAccount">
          <t-form-item v-if="user?.role === 'technician'" label="目标用户 ID">
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
          :columns="columns"
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
  </t-loading>
</template>
