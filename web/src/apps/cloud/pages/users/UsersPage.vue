<script setup>
import ResourceFilterBar from '../../../../shared/ui/resource/ResourceFilterBar.vue'
import ResourceRowActions from '../../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceOverlay from '../../../../shared/ui/resource/ResourceOverlay.vue'
import ResourceForm from '../../../../shared/ui/resource/ResourceForm.vue'
import { opColumnWidth } from '../../../../shared/ui/resource/opColumnWidth.js'
import { usePendingActions, useEditorClose } from './managementInteractions.js'

import BusinessSelect from "../../../../shared/ui/fields/BusinessSelect.vue"
import FeedbackAlert from '../../../../shared/ui/feedback/FeedbackAlert.vue'
import { feedback } from '../../../../shared/ui/feedback/index.js'
import { computed, onMounted, ref } from 'vue'
import { createUsersClient } from './usersApi.js'
import ResourceCard from '../../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../../shared/ui/resource/ResourceStatusBadge.vue'
import { enabledCategories } from '../../../../shared/utils/categories.js'
import { createSessionClient } from '../../../../shared/api/session.js'
import ResourceDescriptions from '../../../../shared/ui/resource/ResourceDescriptions.vue'
import { useAutoRefresh } from '../../../../shared/utils/useAutoRefresh.js'

const client = createUsersClient()
const sessionClient = createSessionClient(), currentUser = ref(null)
const formError = ref(''), fieldErrors = ref({}), passwordError = ref(''), savingPassword = ref(false)
const { pending: pendingActions, run: runAction } = usePendingActions()
const users = ref([])
const teams = ref([])
const categories = ref([])
const loading = ref(true)
const error = ref('')

const filters = ref({ uid: '', username: '', role: '', teamId: '', status: '', categoryId: '' })
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
const enabledCategoryOptions = computed(() => enabledCategories(categories.value))
const categoryNameMap = computed(() => new Map(categories.value.map((category) => [category.id, category.name])))
const requiresScope = computed(() => userForm.value.role !== 'admin')
const pagedUsers = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return users.value.slice(start, start + pagination.value.pageSize)
})

const columns = [
  { colKey: 'id', title: 'UID', width: 80 },
  { colKey: 'username', title: '用户名', width: 140, ellipsis: true },
  { colKey: 'role', title: '角色', width: 100 },
  { colKey: 'team_name', title: '运营团队', width: 130, ellipsis: true },
  { colKey: 'category_ids', title: '类目范围', width: 170, ellipsis: true },
  { colKey: 'status', title: '状态', width: 80 },
  { colKey: 'operations', title: '操作', width: opColumnWidth(['编辑', '重置密码', '停用', '删除'], { variant: 'text', loading: true }), fixed: 'right' },
]

const tableScroll = computed(() => ({ x: `${columns.reduce((sum, col) => sum + col.width, 0)}px` }))

onMounted(async () => {
  try { currentUser.value = await sessionClient.me() } catch { currentUser.value = null }
  await loadAll()
})
const editorClose = useEditorClose({ visible: userDialogVisible, busy: savingUser, draft: () => userForm.value, confirm: feedback.confirmWarning })
const passwordClose = useEditorClose({ visible: passwordDialogVisible, busy: savingPassword, draft: () => newPassword.value, confirm: feedback.confirmWarning })
async function closeEditor() { await editorClose.close(); if (!userDialogVisible.value) userForm.value.password = '' }
async function closePassword() { await passwordClose.close(); if (!passwordDialogVisible.value) newPassword.value = '' }
function isSelf(user) { return !currentUser.value || Number(user.id) === Number(currentUser.value.id) }

function emptyUserForm() {
  return { username: '', password: '', role: 'operator', teamId: '', status: 'enabled', categoryIds: [] }
}

// quiet 由定时刷新传入：保留页码与筛选，失败沿用上一次读数。
async function loadAll({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    const [userRows, teamRows, categoryRows] = await Promise.all([
      client.listUsers(filters.value),
      client.listTeams(),
      client.listCategories(),
    ])
    users.value = userRows || []
    teams.value = teamRows || []
    categories.value = categoryRows || []
    pagination.value.current = Math.min(pagination.value.current, Math.max(1, Math.ceil(users.value.length / pagination.value.pageSize)))
  } catch (e) {
    if (!quiet) error.value = friendlyError(e)
  } finally {
    if (!quiet) loading.value = false
  }
}

useAutoRefresh(() => loadAll({ quiet: true }), {
  shouldSkip: () => userDialogVisible.value || savingUser.value || passwordDialogVisible.value || !!oneTimePassword.value || pendingActions.value.size > 0,
})

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
  filters.value = { uid: '', username: '', role: '', teamId: '', status: '', categoryId: '' }
  applyFilters()
}

function openCreateUser() {
  editingUser.value = null
  userForm.value = emptyUserForm()
  formError.value = ''; fieldErrors.value = {}
  editorClose.capture()
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
    categoryIds: [...(user.category_ids || [])],
  }
  formError.value = ''; fieldErrors.value = {}
  editorClose.capture()
  userDialogVisible.value = true
}

async function saveUser() {
  if (savingUser.value) return
  const errors = {}, form = userForm.value
  if (!form.username.trim()) errors.username = '请输入用户名'
  if (!editingUser.value && form.password.length < 6) errors.password = '密码至少6位'
  if (!form.role) errors.role = '请选择角色'
  if (requiresScope.value && !form.teamId) errors.teamId = '请选择运营团队'
  if (requiresScope.value && !form.categoryIds.length) errors.categoryIds = '至少选择一个类目'
  fieldErrors.value = errors
  if (Object.keys(errors).length) return
  savingUser.value = true; formError.value = ''
  try {
    if (editingUser.value) {
      await client.updateUser(editingUser.value.id, { ...form, status: undefined })
      feedback.toast.success('已保存用户')
    } else {
      const result = await client.createUser({ ...form, username: form.username.trim() })
      showOneTimePassword(result.one_time_password)
    }
    userDialogVisible.value = false
    userForm.value.password = ''
    await loadAll()
  } catch (e) { formError.value = friendlyError(e) }
  finally { savingUser.value = false }
}

async function toggleUser(user) {
  if (isSelf(user)) return
  return runAction(user.id, async () => {
    const disabling = user.status === 'enabled'
    if (!await feedback.confirmWarning({ title: disabling ? '停用用户' : '启用用户', message: disabling ? `停用 ${user.username} 后，该用户将立即退出全部登录会话。` : `启用 ${user.username} 后，该用户可重新登录。`, confirmText: disabling ? '停用' : '启用', danger: disabling })) return
    try {
      await client.updateUser(user.id, { role: user.role, teamId: user.team_id, status: disabling ? 'disabled' : 'enabled', categoryIds: user.category_ids || [] })
      feedback.toast.success(disabling ? '已停用用户' : '已启用用户')
      await loadAll()
    } catch (e) { error.value = friendlyError(e) }
  })
}

async function deleteUser(user) {
  if (isSelf(user)) return
  return runAction(user.id, async () => {
    if (!await feedback.confirmWarning({ title: '删除用户', message: `确定删除 ${user.username}？有历史记录的用户不能删除。`, confirmText: '删除', danger: true })) return
    try { await client.deleteUser(user.id); feedback.toast.success('已删除用户'); await loadAll() }
    catch (e) { error.value = friendlyError(e) }
  })
}

function openPasswordReset(user) {
  passwordTarget.value = user
  newPassword.value = ''; passwordError.value = ''
  passwordClose.capture()
  passwordDialogVisible.value = true
}

async function resetPassword() {
  if (savingPassword.value) return
  passwordError.value = newPassword.value.length < 6 ? '密码至少6位' : ''
  if (passwordError.value) return
  savingPassword.value = true
  try {
    const result = await client.resetPassword(passwordTarget.value.id, newPassword.value)
    passwordDialogVisible.value = false
    newPassword.value = ''
    showOneTimePassword(result.one_time_password)
  } catch (e) { passwordError.value = friendlyError(e) }
  finally { savingPassword.value = false }
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
  try { await navigator.clipboard.writeText(oneTimePassword.value); feedback.toast.success('已复制密码') }
  catch { feedback.toast.error('复制失败，请手动复制') }
}

function friendlyError(e) {
  return e?.message || '操作失败，请检查输入后重试'
}

function categoryScopeText(categoryIds, role) {
  categoryIds = Array.isArray(categoryIds) ? categoryIds : []
  if (role === 'admin') return '全部类目'
  if (!categoryIds.length) return '未分配类目'
  return categoryIds.map((id) => categoryNameMap.value.get(id) || id).join(', ')
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <div class="wt-resource-page">
    <ResourcePageHeader title="用户管理" description="管理用户权限、运营团队与类目范围">
      <template #actions>
        <t-button theme="primary" @click="openCreateUser">新增用户</t-button>
      </template>
    </ResourcePageHeader>
    <FeedbackAlert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <ResourceCard class="system-resource-card">
      <ResourceFilterBar>
        <label class="wt-filter-field"><span class="wt-filter-field__label">UID</span><t-input v-model="filters.uid" placeholder="UID" clearable /></label>
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">用户名</span><t-input v-model="filters.username" placeholder="用户名" clearable /></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">角色</span><t-select v-model="filters.role" placeholder="全部" clearable>
          <t-option value="admin" label="管理员" />
          <t-option value="senior_operator" label="高级运营" />
          <t-option value="operator" label="普通运营" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">运营团队</span><BusinessSelect kind="team" filter v-model="filters.teamId" clearable>
          <t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" />
        </BusinessSelect></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">类目</span><BusinessSelect kind="category" filter v-model="filters.categoryId" clearable>
          <t-option v-for="category in enabledCategoryOptions" :key="category.id" :value="category.id" :label="`${category.name}（${category.id}）`" />
        </BusinessSelect></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" placeholder="全部" clearable>
          <t-option value="enabled" label="启用" />
          <t-option value="disabled" label="停用" />
        </t-select></label>
        <div class="wt-filter-actions"><t-button theme="primary" @click="applyFilters">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="clearFilters">重置</t-button></div>
      </ResourceFilterBar>

      <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="pagedUsers" :columns="columns" size="small" hover row-key="id" :scroll="tableScroll">
        <template #role="{ row }"><t-tag size="small">{{ roleLabels[row.role] || row.role }}</t-tag></template>
        <template #team_name="{ row }">{{ row.role === 'admin' ? '全局' : (row.team_name || '未分配团队') }}</template>
        <template #category_ids="{ row }">{{ categoryScopeText(row.category_ids, row.role) }}</template>
        <template #status="{ row }"><ResourceStatusBadge :tone="row.status === 'enabled' ? 'success' : 'neutral'" :label="row.status === 'enabled' ? '启用' : '停用'" /></template>
        <template #operations="{ row }">
          <ResourceRowActions>
            <t-button size="small" variant="text" :disabled="pendingActions.has(row.id)" @click="openEditUser(row)">编辑</t-button>
            <t-button size="small" variant="text" :disabled="pendingActions.has(row.id)" @click="openPasswordReset(row)">重置密码</t-button>
            <t-tooltip :content="isSelf(row) ? '不能修改自己的启停状态' : (row.status === 'enabled' ? '停用后，该用户将立即退出全部登录会话' : '启用后可重新登录')"><span><t-button size="small" variant="text" :disabled="isSelf(row)" :loading="pendingActions.has(row.id)" @click="toggleUser(row)">{{ row.status === 'enabled' ? '停用' : '启用' }}</t-button></span></t-tooltip>
            <t-tooltip :content="isSelf(row) ? '不能删除当前登录用户' : '有历史记录的用户不能删除'"><span><t-button size="small" variant="text" class="is-danger" :disabled="isSelf(row) || pendingActions.has(row.id)" @click="deleteUser(row)">删除</t-button></span></t-tooltip>
          </ResourceRowActions>
        </template>
      </t-table></div>
      <div class="pagination-bar">
        <t-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="users.length" :page-size-options="[10, 20, 50]" />
      </div>
    </ResourceCard>

    <ResourceOverlay :visible="userDialogVisible" :title="editingUser ? '编辑用户' : '新增用户'" :busy="savingUser" @update:visible="value => { if (!value) closeEditor() }">
      <FeedbackAlert style="margin-bottom:16px" v-if="formError" theme="error" :message="formError" />
      <ResourceForm @submit="saveUser">
        <t-form-item v-if="editingUser" label="UID"><t-input :value="String(editingUser.id)" disabled /></t-form-item>
        <t-form-item label="用户名" required-mark :status="fieldErrors.username ? 'error' : undefined" :tips="fieldErrors.username"><t-input v-model="userForm.username" placeholder="输入用户名" :disabled="!!editingUser || savingUser" /></t-form-item>
        <t-form-item v-if="!editingUser" label="初始密码" required-mark :status="fieldErrors.password ? 'error' : undefined" :tips="fieldErrors.password"><t-input v-model="userForm.password" type="password" placeholder="至少6位" :disabled="savingUser" /></t-form-item>
        <t-form-item label="角色" required-mark :status="fieldErrors.role ? 'error' : undefined" :tips="fieldErrors.role">
          <t-select v-model="userForm.role" :disabled="savingUser"><t-option value="admin" label="管理员" /><t-option value="senior_operator" label="高级运营" /><t-option value="operator" label="普通运营" /></t-select>
        </t-form-item>
        <t-form-item v-if="requiresScope" label="运营团队" required-mark :status="fieldErrors.teamId ? 'error' : undefined" :tips="fieldErrors.teamId">
          <BusinessSelect kind="team" v-model="userForm.teamId" :disabled="savingUser"><t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" /></BusinessSelect>
        </t-form-item>
        <t-form-item v-if="requiresScope" label="类目范围" required-mark :status="fieldErrors.categoryIds ? 'error' : undefined" :tips="fieldErrors.categoryIds">
          <BusinessSelect kind="category" v-model="userForm.categoryIds" multiple filterable :disabled="savingUser"><t-option v-for="category in enabledCategoryOptions" :key="category.id" :value="category.id" :label="`${category.name}（${category.id}）`" /></BusinessSelect>
        </t-form-item>
      </ResourceForm>
      <template #footer><t-button variant="outline" :disabled="savingUser" @click="closeEditor">取消</t-button><t-button theme="primary" :loading="savingUser" @click="saveUser">{{ editingUser ? '保存' : '创建' }}</t-button></template>
    </ResourceOverlay>
    <ResourceOverlay :visible="passwordDialogVisible" title="重置密码" :busy="savingPassword" @update:visible="value => { if (!value) closePassword() }">
      <ResourceDescriptions><t-descriptions-item label="用户">{{ passwordTarget?.username }}</t-descriptions-item><t-descriptions-item label="UID">{{ passwordTarget?.id }}</t-descriptions-item></ResourceDescriptions>
      <FeedbackAlert theme="warning" message="重置后，该用户将立即退出全部登录会话。" style="margin:16px 0" />
      <FeedbackAlert v-if="passwordError" theme="error" :message="passwordError" />
      <ResourceForm @submit="resetPassword"><t-form-item label="新密码" required-mark><t-input v-model="newPassword" type="password" placeholder="至少6位" :disabled="savingPassword" /></t-form-item></ResourceForm>
      <template #footer><t-button variant="outline" :disabled="savingPassword" @click="closePassword">取消</t-button><t-button theme="primary" :loading="savingPassword" @click="resetPassword">重置密码</t-button></template>
    </ResourceOverlay>
    <ResourceOverlay :visible="!!oneTimePassword" title="本次密码" @update:visible="value => { if (!value) clearOneTimePassword() }">
      <FeedbackAlert theme="warning" message="密码仅本次显示，请复制保存。" style="margin-bottom:16px" />
      <t-input :value="oneTimePassword" readonly><template #suffix><t-button size="small" variant="text" @click="copyPassword">复制</t-button></template></t-input>
      <template #footer><t-button theme="primary" @click="clearOneTimePassword">完成</t-button></template>
    </ResourceOverlay>
    </div>
  </t-loading>
</template>

<style scoped>
</style>
