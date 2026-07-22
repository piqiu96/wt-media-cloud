<script setup>
import { computed, onMounted, ref } from 'vue'
import { createUsersClient } from './usersApi.js'

const client = createUsersClient()
const users = ref([])
const teams = ref([])
const loading = ref(true)
const error = ref('')

const filters = ref({ uid: '', username: '', role: '', teamId: '', status: '', gameId: '' })
const userDialogVisible = ref(false)
const editingUser = ref(null)
const userForm = ref(emptyUserForm())
const savingUser = ref(false)

const teamDialogVisible = ref(false)
const editingTeam = ref(null)
const teamName = ref('')
const savingTeam = ref(false)

const passwordDialogVisible = ref(false)
const oneTimePassword = ref('')
const passwordTarget = ref(null)
const newPassword = ref('')

const roleLabels = { admin: '管理员', senior_operator: '高级运营', operator: '普通运营' }
const columns = [
  { colKey: 'id', title: 'UID', width: 90 },
  { colKey: 'username', title: '用户名', width: 150 },
  { colKey: 'role', title: '角色', width: 120 },
  { colKey: 'team_name', title: '运营分组', width: 140 },
  { colKey: 'game_ids', title: '游戏范围' },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'operations', title: '操作', width: 280, fixed: 'right' },
]

const requiresScope = computed(() => userForm.value.role !== 'admin')

onMounted(loadAll)

function emptyUserForm() {
  return { username: '', password: '', role: 'operator', teamId: '', status: 'enabled', gameIds: [] }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [userRows, teamRows] = await Promise.all([client.listUsers(filters.value), client.listTeams()])
    users.value = userRows || []
    teams.value = teamRows || []
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function applyFilters() {
  loading.value = true
  error.value = ''
  try {
    users.value = await client.listUsers(filters.value) || []
  } catch (e) {
    error.value = e.message
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
  if (!userForm.value.role || (requiresScope.value && (!userForm.value.teamId || !userForm.value.gameIds.length))) {
    error.value = '普通运营和高级运营必须选择运营分组及至少一个游戏'
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
    error.value = e.message
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
    error.value = e.message
  }
}

async function deleteUser(user) {
  if (!window.confirm(`确定删除用户 ${user.username}？有历史记录的用户将被阻止删除。`)) return
  try {
    await client.deleteUser(user.id)
    await loadAll()
  } catch (e) {
    error.value = e.message
  }
}

function openPasswordReset(user) {
  passwordTarget.value = user
  newPassword.value = ''
  passwordDialogVisible.value = true
}

async function resetPassword() {
  try {
    const result = await client.resetPassword(passwordTarget.value.id, newPassword.value)
    passwordDialogVisible.value = false
    showOneTimePassword(result.one_time_password)
  } catch (e) {
    error.value = e.message
  }
}

function showOneTimePassword(password) {
  oneTimePassword.value = password || ''
}

function clearOneTimePassword() {
  oneTimePassword.value = ''
}

async function copyPassword() {
  await navigator.clipboard.writeText(oneTimePassword.value)
}

function openCreateTeam() {
  editingTeam.value = null
  teamName.value = ''
  teamDialogVisible.value = true
}

function openRenameTeam(team) {
  editingTeam.value = team
  teamName.value = team.name
  teamDialogVisible.value = true
}

async function saveTeam() {
  savingTeam.value = true
  try {
    if (editingTeam.value) await client.renameTeam(editingTeam.value.id, teamName.value)
    else await client.createTeam(teamName.value)
    teamDialogVisible.value = false
    await loadAll()
  } catch (e) {
    error.value = e.message
  } finally {
    savingTeam.value = false
  }
}

async function deleteTeam(team) {
  if (!window.confirm(`确定删除运营分组 ${team.name}？`)) return
  try {
    await client.deleteTeam(team.id)
    await loadAll()
  } catch (e) {
    error.value = e.message
  }
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />

    <t-card title="用户与权限" :bordered="true" style="margin-bottom:16px">
      <template #actions>
        <t-button theme="primary" size="small" @click="openCreateUser">新建用户</t-button>
        <t-button theme="default" size="small" style="margin-left:8px" @click="loadAll">刷新</t-button>
      </template>
      <div class="filter-row">
        <span class="filter-title">筛选</span>
        <t-input v-model="filters.uid" placeholder="UID" clearable />
        <t-input v-model="filters.username" placeholder="用户名" clearable />
        <t-select v-model="filters.role" placeholder="角色" clearable>
          <t-option value="admin" label="管理员" />
          <t-option value="senior_operator" label="高级运营" />
          <t-option value="operator" label="普通运营" />
        </t-select>
        <t-select v-model="filters.teamId" placeholder="运营分组" clearable>
          <t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" />
        </t-select>
        <t-input v-model="filters.gameId" placeholder="游戏 ID" clearable />
        <t-select v-model="filters.status" placeholder="状态" clearable>
          <t-option value="enabled" label="启用" />
          <t-option value="disabled" label="停用" />
        </t-select>
        <t-button size="small" @click="applyFilters">查询</t-button>
        <t-button size="small" variant="text" @click="clearFilters">清空</t-button>
      </div>

      <t-table :data="users" :columns="columns" size="small" hover row-key="id">
        <template #role="{ row }"><t-tag size="small">{{ roleLabels[row.role] || row.role }}</t-tag></template>
        <template #team_name="{ row }">{{ row.team_name || '全局' }}</template>
        <template #game_ids="{ row }">{{ row.game_ids?.length ? row.game_ids.join(', ') : '全部游戏' }}</template>
        <template #status="{ row }"><t-tag :theme="row.status === 'enabled' ? 'success' : 'danger'" size="small">{{ row.status === 'enabled' ? '启用' : '停用' }}</t-tag></template>
        <template #operations="{ row }">
          <t-space>
            <t-button size="small" variant="text" @click="openEditUser(row)">编辑用户</t-button>
            <t-button size="small" variant="text" @click="openPasswordReset(row)">重置密码</t-button>
            <t-button size="small" variant="text" @click="toggleUser(row)">{{ row.status === 'enabled' ? '停用' : '启用' }}</t-button>
            <t-button size="small" variant="text" theme="danger" @click="deleteUser(row)">删除</t-button>
          </t-space>
        </template>
      </t-table>
    </t-card>

    <t-card title="运营分组" :bordered="true">
      <template #actions><t-button size="small" @click="openCreateTeam">新建分组</t-button></template>
      <t-list :split="true">
        <t-list-item v-for="team in teams" :key="team.id">
          <span>#{{ team.id }} {{ team.name }}</span>
          <template #action>
            <t-button size="small" variant="text" @click="openRenameTeam(team)">重命名</t-button>
            <t-button size="small" variant="text" theme="danger" @click="deleteTeam(team)">删除</t-button>
          </template>
        </t-list-item>
      </t-list>
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
          <t-tag-input v-model="userForm.gameIds" placeholder="输入游戏 ID 后回车" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog v-model:visible="teamDialogVisible" :header="editingTeam ? '重命名运营分组' : '新建运营分组'" :confirm-btn="{ loading: savingTeam }" @confirm="saveTeam">
      <t-form><t-form-item label="分组名称"><t-input v-model="teamName" /></t-form-item></t-form>
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
.filter-row { display:grid; grid-template-columns:auto repeat(6, minmax(110px, 1fr)) auto auto; gap:8px; align-items:center; margin-bottom:16px; }
.filter-title { color:var(--td-text-color-secondary); }
@media (max-width: 1200px) { .filter-row { grid-template-columns:repeat(3, minmax(140px, 1fr)); } }
</style>
