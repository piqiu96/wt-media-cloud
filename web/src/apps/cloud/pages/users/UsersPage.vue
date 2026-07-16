<script setup>
import { onMounted, ref } from 'vue'
import { createSessionClient } from '../../../../shared/api/session.js'

const sessionClient = createSessionClient()
const users = ref([])
const loading = ref(true)
const error = ref('')
const dialogVisible = ref(false)
const newUser = ref({ username: '', password: '', role: 'operator', game_ids: '' })

async function loadUsers() {
  error.value = ''
  try {
    const resp = await fetch('/api/v1/users', { credentials: 'include' })
    if (!resp.ok) throw new Error('加载失败')
    const body = await resp.json()
    users.value = body.data || body
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function createUser() {
  error.value = ''
  try {
    const resp = await fetch('/api/v1/users', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: newUser.value.username,
        password: newUser.value.password,
        role: newUser.value.role,
        game_ids: newUser.value.game_ids.split(',').map(s => s.trim()).filter(Boolean),
      }),
    })
    if (!resp.ok) throw new Error('创建失败')
    dialogVisible.value = false
    newUser.value = { username: '', password: '', role: 'operator', game_ids: '' }
    await loadUsers()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(loadUsers)

const columns = [
  { colKey: 'username', title: '用户名' },
  { colKey: 'role', title: '角色', width: 120 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'game_ids', title: '游戏范围' },
]
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <t-card title="用户管理" :bordered="true">
      <template #actions>
        <t-button theme="primary" size="small" @click="dialogVisible = true">新建用户</t-button>
        <t-button theme="default" size="small" style="margin-left:8px" @click="loadUsers">刷新</t-button>
      </template>
      <t-table :data="users" :columns="columns" size="small" hover row-key="id">
        <template #role="{ row }">
          <t-tag :theme="row.role === 'technician' ? 'warning' : row.role === 'senior_operator' ? 'primary' : 'default'" size="small">{{ row.role }}</t-tag>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'enabled' ? 'success' : 'danger'" size="small">{{ row.status }}</t-tag>
        </template>
        <template #game_ids="{ row }">
          <span>{{ row.game_ids?.length ? row.game_ids.join(', ') : '全局' }}</span>
        </template>
      </t-table>
    </t-card>

    <t-dialog v-model:visible="dialogVisible" header="新建用户" @confirm="createUser" :confirm-btn="{ theme: 'primary' }">
      <t-form>
        <t-form-item label="用户名">
          <t-input v-model="newUser.username" />
        </t-form-item>
        <t-form-item label="密码">
          <t-input v-model="newUser.password" type="password" />
        </t-form-item>
        <t-form-item label="角色">
          <t-select v-model="newUser.role">
            <t-option value="operator" label="运营" />
            <t-option value="senior_operator" label="高级运营" />
            <t-option value="technician" label="技术" />
          </t-select>
        </t-form-item>
        <t-form-item label="游戏范围">
          <t-input v-model="newUser.game_ids" placeholder="多个用逗号分隔，留空表示全局" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </t-loading>
</template>
