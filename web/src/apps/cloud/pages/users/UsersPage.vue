<script setup>
import { computed, onMounted, ref } from 'vue'
import { createUsersClient } from './usersApi.js'

const client = createUsersClient()
const users = ref([])
const teams = ref([])
const games = ref([])
const loading = ref(true)
const error = ref('')

const filters = ref({ uid: '', username: '', role: '', teamId: '', status: '', gameId: '' })
const pagination = ref({ current: 1, pageSize: 10 })
const userDialogVisible = ref(false)
const editingUser = ref(null)
const userForm = ref(emptyUserForm())
const savingUser = ref(false)

const passwordDialogVisible = ref(false)
const oneTimePassword = ref('')
const passwordTarget = ref(null)
const newPassword = ref('')

const roleLabels = { admin: '管理员', senior_operator: '高级运营', operator: '普通运营' }
const enabledGames = computed(() => games.value.filter((game) => game.status === 'enabled'))
const gameNameMap = computed(() => new Map(games.value.map((game) => [game.id, game.name])))
const requiresScope = computed(() => userForm.value.role !== 'admin')
const pagedUsers = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return users.value.slice(start, start + pagination.value.pageSize)
})

const columns = [
  { colKey: 'id', title: 'UID', width: 90 },
  { colKey: 'username', title: '用户名', width: 150 },
  { colKey: 'role', title: '角色', width: 120 },
  { colKey: 'team_name', title: '运营分组', width: 140 },
  { colKey: 'game_ids', title: '游戏范围' },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'operations', title: '操作', width: 280, fixed: 'right' },
]

onMounted(loadAll)

function emptyUserForm() {
  return { username: '', password: '', role: 'operator', teamId: '', status: 'enabled', gameIds: [] }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [userRows, teamRows, gameRows] = await Promise.all([
      client.listUsers(filters.value),
      client.listTeams(),
      client.listGames(),
    ])
    users.value = userRows || []
    teams.value = teamRows || []
    games.value = gameRows || []
    pagination.value.current = 1
  } catch (e) {
    error.value = friendlyError(e)
  } finally {
    loading.value = false
  }
}

async function applyFilters() {
  loading.value = true
  error.value = ''
  try {
    users.value = await client.listUsers(filters.value) || []
    pagination.value.current = 1
  } catch (e) {
    error.value = friendlyError(e)
  } finally {
    loading.value = false
  }
}

function clearFilters() {
  filters.value = { uid: '', username: '', role: '', teamId: '', status: '', gameId: '' }
  applyFilters()
}

function openCreateUser() {
  editingUser.value = null
  userForm.value = emptyUserForm()
  userDialogVisible.value = true
}

function openEditUser(user) {
  editingUser.value = user
  userForm.value = {
    username: user.username,
    password: '',
    role: user.role,
    teamId: user.team_id || '',
    status: user.status,
    gameIds: [...(user.game_ids || [])],
  }
  userDialogVisible.value = true
}

async function saveUser() {
  if (!userForm.value.role) {
    error.value = '请选择用户角色'
    return
  }
  if (!editingUser.value && userForm.value.password.length < 6) {
    error.value = '密码至少需要6位'
    return
  }
  if (requiresScope.value && !userForm.value.teamId) {
    error.value = '普通运营和高级运营必须选择运营分组'
    return
  }
  if (requiresScope.value && !userForm.value.gameIds.length) {
    error.value = '普通运营和高级运营必须选择至少一个游戏'
    return
  }
  savingUser.value = true
  error.value = ''
  try {
    if (editingUser.value) {
      await client.updateUser(editingUser.value.id, userForm.value)
    } else {
      const result = await client.createUser(userForm.value)
      showOneTimePassword(result.one_time_password)
    }
    userDialogVisible.value = false
    await loadAll()
  } catch (e) {
    error.value = friendlyError(e)
  } finally {
    savingUser.value = false
  }
}

async function toggleUser(user) {
  try {
    await client.updateUser(user.id, {
      role: user.role,
      teamId: user.team_id,
      status: user.status === 'enabled' ? 'disabled' : 'enabled',
      gameIds: user.game_ids || [],
    })
    await loadAll()
  } catch (e) {
    error.value = friendlyError(e)
  }
}

async function deleteUser(user) {
  if (!window.confirm(`确定删除用户 ${user.username}？有历史记录的用户将被阻止删除。`)) return
  try {
    await client.deleteUser(user.id)
    await loadAll()
  } catch (e) {
    error.value = friendlyError(e)
  }
}

async function clearBitBrowserBinding(user) {
  const confirmed = window.confirm(`确定解除 ${user.username} 的比特浏览器账号绑定？\n\n解除后，该用户需要在 Desktop 重新绑定正确的比特浏览器账号。\n系统不会删除已有浏览器窗口、媒体账号和历史记录。`)
  if (!confirmed) return
  try {
    await client.clearBitBrowserBinding(user.id)
    await loadAll()
  } catch (e) {
    error.value = friendlyError(e)
  }
}

function openPasswordReset(user) {
  passwordTarget.value = user
  newPassword.value = ''
  passwordDialogVisible.value = true
}

async function resetPassword() {
  if (newPassword.value.length < 6) {
    error.value = '密码至少需要6位'
    return
  }
  try {
    const result = await client.resetPassword(passwordTarget.value.id, newPassword.value)
    passwordDialogVisible.value = false
    showOneTimePassword(result.one_time_password)
  } catch (e) {
    error.value = friendlyError(e)
  }
}

function showOneTimePassword(password) {
  oneTimePassword.value = password || ''
}

function clearOneTimePassword() {
  oneTimePassword.value = ''
  userForm.value.password = ''
  newPassword.value = ''
}

async function copyPassword() {
  await navigator.clipboard.writeText(oneTimePassword.value)
}

function friendlyError(e) {
  return e?.message || '操作失败，请检查输入后重试'
}

function gameScopeText(gameIds) {
  gameIds = Array.isArray(gameIds) ? gameIds : []
  if (!gameIds.length) return '全部游戏'
  return gameIds.map((id) => gameNameMap.value.get(id) || id).join(', ')
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />

    <t-card title="用户管理" :bordered="true">
      <template #actions>
        <t-button theme="primary" size="small" @click="openCreateUser">新建用户</t-button>
        <t-button theme="default" size="small" style="margin-left:8px" @click="loadAll">刷新</t-button>
      </template>
      <div class="filter-row">
        <label class="filter-field"><span>UID</span><t-input v-model="filters.uid" placeholder="UID" clearable style="width:120px" /></label>
        <label class="filter-field"><span>用户名</span><t-input v-model="filters.username" placeholder="用户名" clearable style="width:140px" /></label>
        <label class="filter-field"><span>角色</span><t-select v-model="filters.role" placeholder="全部" clearable style="width:140px">
          <t-option value="admin" label="管理员" />
          <t-option value="senior_operator" label="高级运营" />
          <t-option value="operator" label="普通运营" />
        </t-select></label>
        <label class="filter-field"><span>运营分组</span><t-select v-model="filters.teamId" placeholder="全部" clearable style="width:160px">
          <t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" />
        </t-select></label>
        <label class="filter-field"><span>游戏</span><t-select v-model="filters.gameId" placeholder="全部" clearable style="width:160px">
          <t-option v-for="game in games" :key="game.id" :value="game.id" :label="`${game.name}（${game.id}）`" />
        </t-select></label>
        <label class="filter-field"><span>状态</span><t-select v-model="filters.status" placeholder="全部" clearable style="width:120px">
          <t-option value="enabled" label="启用" />
          <t-option value="disabled" label="停用" />
        </t-select></label>
        <t-button theme="primary" size="small" @click="applyFilters">查询</t-button>
        <t-button class="wt-secondary-button" size="small" variant="outline" @click="clearFilters">重置</t-button>
      </div>

      <t-table :data="pagedUsers" :columns="columns" size="small" hover row-key="id">
        <template #role="{ row }"><t-tag size="small">{{ roleLabels[row.role] || row.role }}</t-tag></template>
        <template #team_name="{ row }">{{ row.team_name || '全局' }}</template>
        <template #game_ids="{ row }">{{ gameScopeText(row.game_ids) }}</template>
        <template #status="{ row }"><t-tag :theme="row.status === 'enabled' ? 'success' : 'danger'" size="small">{{ row.status === 'enabled' ? '启用' : '停用' }}</t-tag></template>
        <template #operations="{ row }">
          <t-space>
            <t-button size="small" variant="text" @click="openEditUser(row)">编辑用户</t-button>
            <t-button size="small" variant="text" @click="openPasswordReset(row)">重置密码</t-button>
            <t-button size="small" variant="text" theme="warning" @click="clearBitBrowserBinding(row)">解除比特绑定</t-button>
            <t-button size="small" variant="text" @click="toggleUser(row)">{{ row.status === 'enabled' ? '停用' : '启用' }}</t-button>
            <t-button size="small" variant="text" theme="danger" @click="deleteUser(row)">删除</t-button>
          </t-space>
        </template>
      </t-table>
      <div class="pagination-row">
        <t-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="users.length" :page-size-options="[10, 20, 50]" />
      </div>
    </t-card>

    <t-dialog v-model:visible="userDialogVisible" :header="editingUser ? '编辑用户' : '新建用户'" :confirm-btn="{ loading: savingUser }" @confirm="saveUser">
      <t-form>
        <t-form-item label="用户名"><t-input v-model="userForm.username" :disabled="!!editingUser" /></t-form-item>
        <t-form-item v-if="!editingUser" label="初始密码"><t-input v-model="userForm.password" type="password" /></t-form-item>
        <t-form-item label="角色">
          <t-select v-model="userForm.role">
            <t-option value="admin" label="管理员" />
            <t-option value="senior_operator" label="高级运营" />
            <t-option value="operator" label="普通运营" />
          </t-select>
        </t-form-item>
        <t-form-item v-if="requiresScope" label="运营分组">
          <t-select v-model="userForm.teamId"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></t-select>
        </t-form-item>
        <t-form-item v-if="requiresScope" label="游戏范围">
          <t-select v-model="userForm.gameIds" multiple filterable placeholder="请选择已启用游戏">
            <t-option v-for="game in enabledGames" :key="game.id" :value="game.id" :label="`${game.name}（${game.id}）`" />
          </t-select>
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog v-model:visible="passwordDialogVisible" header="重置密码" @confirm="resetPassword">
      <t-form><t-form-item label="新密码"><t-input v-model="newPassword" type="password" /></t-form-item></t-form>
    </t-dialog>

    <t-dialog :visible="!!oneTimePassword" header="一次性密码" :cancel-btn="null" @confirm="clearOneTimePassword" @close="clearOneTimePassword">
      <t-alert theme="warning" message="该密码仅本次显示，关闭后无法再次查看。" style="margin-bottom:12px" />
      <t-input :value="oneTimePassword" readonly>
        <template #suffix><t-button size="small" variant="text" @click="copyPassword">复制</t-button></template>
      </t-input>
    </t-dialog>
  </t-loading>
</template>

<style scoped>
.filter-row { display:flex; flex-wrap:wrap; gap:12px; align-items:center; margin-bottom:16px; }
.filter-field { display:flex; align-items:center; gap:8px; color:var(--wt-text-secondary); font-size:13px; font-weight:500; white-space:nowrap; }
.pagination-row { display:flex; justify-content:flex-end; margin-top:16px; }
</style>
