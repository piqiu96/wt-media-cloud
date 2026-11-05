<script setup>
import ResourceFilterBar from '../../../../shared/ui/resource/ResourceFilterBar.vue'
import ResourceRowActions from '../../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceOverlay from '../../../../shared/ui/resource/ResourceOverlay.vue'
import ResourceForm from '../../../../shared/ui/resource/ResourceForm.vue'
import { opColumnWidth } from '../../../../shared/ui/resource/opColumnWidth.js'
import ResourceSection from '../../../../shared/ui/resource/ResourceSection.vue'
import ResourceEmptyState from '../../../../shared/ui/resource/ResourceEmptyState.vue'
import { usePendingActions, useEditorClose, useReferenceDetails } from './managementInteractions.js'

import BusinessSelect from "../../../../shared/ui/fields/BusinessSelect.vue"
import FeedbackAlert from '../../../../shared/ui/feedback/FeedbackAlert.vue'
import { feedback } from '../../../../shared/ui/feedback/index.js'
import { computed, onMounted, ref } from 'vue'
import { createUsersClient } from './usersApi.js'
import { createSessionClient } from '../../../../shared/api/session.js'
import { isAdminRole } from '../../../../shared/utils/roles.js'
import { platformLabel } from '../../../../shared/utils/platforms.js'
import { CATEGORY_TYPE_OPTIONS, categoryTypeLabel } from '../../../../shared/utils/categories.js'
import { useAutoRefresh } from '../../../../shared/utils/useAutoRefresh.js'
import ResourceCard from '../../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../../shared/ui/resource/ResourceStatusBadge.vue'

const client = createUsersClient()
const sessionClient = createSessionClient()
const currentUser = ref(null)
const categories = ref([])
const loading = ref(true)
const error = ref('')
const filters = ref({ keyword: '', type: '', enabled: '' })
const pagination = ref({ current: 1, pageSize: 10 })
const dialogVisible = ref(false)
const editingCategory = ref(null)
const categoryForm = ref(emptyCategoryForm())
const saving = ref(false)
const referenceDetails = useReferenceDetails(id => client.getCategoryReferences(id))
const { visible: referencesVisible, category: referenceCategory, data: references, loading: referencesLoading, error: referencesError } = referenceDetails
const formError = ref(''), fieldErrors = ref({})
const { pending: pendingActions, run: runAction } = usePendingActions()
const editorClose = useEditorClose({ visible: dialogVisible, busy: saving, draft: () => categoryForm.value, confirm: feedback.confirmWarning })
const roleLabels = { admin: '管理员', senior_operator: '高级运营', operator: '普通运营' }
const userReferenceColumns = [
  { colKey: 'user_id', title: 'UID', width: 80 }, { colKey: 'username', title: '用户名', width: 150 },
  { colKey: 'role', title: '角色', width: 100 }, { colKey: 'team_name', title: '运营团队', width: 180 },
]
const accountReferenceColumns = [
  { colKey: 'account_id', title: '账号ID', width: 90 }, { colKey: 'name', title: '账号名称', width: 160 },
  { colKey: 'platform', title: '平台', width: 100 }, { colKey: 'username', title: '归属用户', width: 160 },
]

const pagedCategories = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return categories.value.slice(start, start + pagination.value.pageSize)
})

// 高级运营能看全部状态，但只读，也不拿关联摘要（关联详情仍归管理员）。
const canManage = computed(() => isAdminRole(currentUser.value))

const columns = computed(() => {
  const base = [
    { colKey: 'id', title: '类目ID', width: 80 },
    { colKey: 'name', title: '类目名称', width: 180 },
    { colKey: 'type', title: '一级分类', width: 100 },
    { colKey: 'remark', title: '备注', width: 180, ellipsis: true },
  ]
  if (canManage.value) base.push({ colKey: 'references', title: '用户／账号关联', width: 170 })
  base.push({ colKey: 'enabled', title: '状态', width: 100 })
  if (canManage.value) base.push({ colKey: 'operations', title: '操作', width: opColumnWidth(['编辑', '删除'], { variant: 'text', loading: true }), fixed: 'right' })
  return base
})

const tableScroll = computed(() => ({ x: `${columns.value.reduce((sum, col) => sum + col.width, 0)}px` }))

onMounted(async () => {
  await loadSession()
  await loadCategories()
})

async function loadSession() {
  try {
    currentUser.value = await sessionClient.me()
  } catch {
    // 读不到会话按只读处理：senior 的 management 视图不比管理员视图多给任何写入口。
    currentUser.value = null
  }
}

function emptyCategoryForm() {
  return { type: 'game', name: '', enabled: true, remark: '' }
}

function resetFilters() {
  filters.value = { keyword: '', type: '', enabled: '' }
  search()
}

// quiet 由定时刷新传入：保留页码与筛选，失败沿用上一次读数。
async function loadCategories({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    // management：全部启停状态、不含关联摘要；all：管理员口径，带摘要。
    const scope = canManage.value ? 'all' : 'management'
    categories.value = await client.listCategories({ ...filters.value, scope }) || []
    pagination.value.current = Math.min(pagination.value.current, Math.max(1, Math.ceil(categories.value.length / pagination.value.pageSize)))
  } catch (e) {
    if (!quiet) error.value = e.message || '加载类目失败'
  } finally {
    if (!quiet) loading.value = false
  }
}

useAutoRefresh(() => loadCategories({ quiet: true }), {
  shouldSkip: () => dialogVisible.value || saving.value || referencesVisible.value || referencesLoading.value || pendingActions.value.size > 0,
})

function referenceSummary(category) {
  return category.reference_summary || { user_scope_count: 0, media_account_count: 0, total_count: 0 }
}

function hasReferences(category) {
  return referenceSummary(category).total_count > 0
}

function openReferences(category) {
  if (!canManage.value) return
  return referenceDetails.open(category)
}

function openCreate() {
  editingCategory.value = null
  categoryForm.value = emptyCategoryForm()
  formError.value = ''; fieldErrors.value = {}
  editorClose.capture()
  dialogVisible.value = true
}

function openEdit(category) {
  editingCategory.value = category
  categoryForm.value = { type: category.type, name: category.name, enabled: category.enabled, remark: category.remark || '' }
  formError.value = ''; fieldErrors.value = {}
  editorClose.capture()
  dialogVisible.value = true
}

async function saveCategory() {
  if (saving.value) return
  const errors = {}
  if (!categoryForm.value.type) errors.type = '请选择一级分类'
  if (!categoryForm.value.name.trim()) errors.name = '请输入类目名称'
  if ([...categoryForm.value.name.trim()].length > 128) errors.name = '类目名称最多128个字符'
  if ([...categoryForm.value.remark].length > 255) errors.remark = '备注最多255个字符'
  fieldErrors.value = errors
  if (Object.keys(errors).length) return
  saving.value = true; formError.value = ''
  try {
    const input = { ...categoryForm.value, name: categoryForm.value.name.trim(), remark: categoryForm.value.remark.trim() }
    if (editingCategory.value) await client.updateCategory(editingCategory.value.id, input)
    else await client.createCategory(input)
    dialogVisible.value = false
    feedback.toast.success('已保存类目')
    await loadCategories()
  } catch (e) { formError.value = e.message || '保存类目失败' }
  finally { saving.value = false }
}

async function toggleCategory(category) {
  if (!canManage.value || (category.enabled && hasReferences(category))) return
  return runAction(category.id, async () => {
    if (!await feedback.confirmWarning({ title: category.enabled ? '停用类目' : '启用类目', message: category.enabled ? `停用 ${category.name} 后，新业务将不能选择该类目。` : `确定启用 ${category.name}？`, confirmText: category.enabled ? '停用' : '启用' })) return
    try {
      await client.updateCategory(category.id, { name: category.name, remark: category.remark, enabled: !category.enabled })
      feedback.toast.success(category.enabled ? '已停用类目' : '已启用类目')
      await loadCategories()
    } catch (e) { error.value = e.message || '更新类目状态失败' }
  })
}

async function deleteCategory(category) {
  if (!canManage.value || hasReferences(category)) return
  return runAction(category.id, async () => {
    if (!await feedback.confirmWarning({ title: '删除类目', message: `确定删除 ${category.name}？有用户授权、账号或历史业务引用时不能删除。`, confirmText: '删除', danger: true })) return
    try { await client.deleteCategory(category.id); feedback.toast.success('已删除类目'); await loadCategories() }
    catch (e) { error.value = e.message || '删除类目失败' }
  })
}

function search() { pagination.value.current = 1; loadCategories() }
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <div class="wt-resource-page">
    <ResourcePageHeader title="类目管理" description="维护类目分类与关联范围">
      <template #actions>
        <t-button v-if="canManage" theme="primary" @click="openCreate">新增类目</t-button>
      </template>
    </ResourcePageHeader>
    <FeedbackAlert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <ResourceCard class="system-resource-card">
      <ResourceFilterBar>
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="filters.keyword" placeholder="类目 ID / 名称" clearable @enter="search" /></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">一级分类</span><BusinessSelect kind="category" filter v-model="filters.type">
          <t-option v-for="option in CATEGORY_TYPE_OPTIONS" :key="option.value" :value="option.value" :label="option.label" />
        </BusinessSelect></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.enabled" placeholder="全部" clearable>
          <t-option :value="true" label="启用" />
          <t-option :value="false" label="停用" />
        </t-select></label>
        <div class="wt-filter-actions"><t-button theme="primary" @click="search">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button></div>
      </ResourceFilterBar>
      <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="pagedCategories" :columns="columns" row-key="id" size="small" hover :scroll="tableScroll">
        <template #references="{ row }">
          <t-link v-if="hasReferences(row)" theme="primary" @click="openReferences(row)">用户 {{ referenceSummary(row).user_scope_count }} / 账号 {{ referenceSummary(row).media_account_count }}</t-link>
          <span v-else>无用户／账号关联</span>
        </template>
        <template #type="{ row }">{{ categoryTypeLabel(row.type) }}</template>
        <template #enabled="{ row }">
          <t-tooltip v-if="canManage" :content="row.enabled && hasReferences(row) ? '仍有用户授权或账号关联，暂不可停用' : (row.enabled ? '停用后，新业务将不能选择' : '启用类目')">
            <span><t-switch :model-value="row.enabled" :label="['启用', '停用']" :loading="pendingActions.has(row.id)" :disabled="pendingActions.has(row.id) || (row.enabled && hasReferences(row))" @change="toggleCategory(row)" /></span>
          </t-tooltip>
          <ResourceStatusBadge v-else :tone="row.enabled ? 'success' : 'neutral'" :label="row.enabled ? '启用' : '停用'" />
        </template>
        <template #operations="{ row }">
          <ResourceRowActions>
            <t-button size="small" variant="text" :disabled="pendingActions.has(row.id)" @click="openEdit(row)">编辑</t-button>
            <t-tooltip :content="hasReferences(row) ? '仍有用户授权或账号关联，暂不可删除' : '有历史业务引用的类目不能删除'"><span><t-button size="small" variant="text" class="is-danger" :disabled="hasReferences(row) || pendingActions.has(row.id)" @click="deleteCategory(row)">删除</t-button></span></t-tooltip>
          </ResourceRowActions>
        </template>
      </t-table></div>
      <div class="pagination-bar">
        <t-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="categories.length" :page-size-options="[10, 20, 50]" />
      </div>
    </ResourceCard>
    <ResourceOverlay :visible="dialogVisible" :title="editingCategory ? '编辑类目' : '新增类目'" :busy="saving" @update:visible="value => { if (!value) editorClose.close() }">
      <FeedbackAlert style="margin-bottom:16px" v-if="formError" theme="error" :message="formError" />
      <ResourceForm @submit="saveCategory">
        <t-form-item v-if="editingCategory" label="类目ID"><t-input :value="String(editingCategory.id)" disabled /></t-form-item>
        <t-form-item label="一级分类" required-mark :status="fieldErrors.type ? 'error' : undefined" :tips="fieldErrors.type"><BusinessSelect kind="category" v-model="categoryForm.type" placeholder="选择一级分类" :clearable="false" :disabled="!!editingCategory || saving"><t-option v-for="option in CATEGORY_TYPE_OPTIONS" :key="option.value" :value="option.value" :label="option.label" /></BusinessSelect></t-form-item>
        <t-form-item label="类目名称" required-mark :status="fieldErrors.name ? 'error' : undefined" :tips="fieldErrors.name"><t-input v-model="categoryForm.name" placeholder="输入类目名称" :maxlength="128" :disabled="saving" /></t-form-item>
        <t-form-item label="备注" :status="fieldErrors.remark ? 'error' : undefined" :tips="fieldErrors.remark"><t-textarea v-model="categoryForm.remark" placeholder="填写备注（选填）" :maxlength="255" :disabled="saving" /></t-form-item>
      </ResourceForm>
      <template #footer><t-button variant="outline" :disabled="saving" @click="editorClose.close">取消</t-button><t-button theme="primary" :loading="saving" @click="saveCategory">{{ editingCategory ? '保存' : '创建' }}</t-button></template>
    </ResourceOverlay>
    <ResourceOverlay v-model:visible="referencesVisible" :title="`${referenceCategory?.name || ''} · 关联信息`" kind="drawer" :width="800" :footer="false">
      <t-loading v-if="referencesLoading" :loading="true" text="加载关联信息…" style="display:block;min-height:180px" />
      <template v-else-if="referencesError"><FeedbackAlert theme="error" :message="referencesError" /><ResourceEmptyState title="关联信息加载失败" description="重试后查看用户授权和账号关联。"><t-button variant="outline" @click="referenceDetails.reload">重试</t-button></ResourceEmptyState></template>
      <template v-else-if="references">
        <ResourceSection title="用户授权"><div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="references.users || []" :columns="userReferenceColumns" :scroll="{ x: '510px' }" row-key="user_id" size="small"><template #role="{ row }">{{ roleLabels[row.role] || row.role }}</template><template #team_name="{ row }">{{ row.team_name || '—' }}</template><template #empty><ResourceEmptyState title="暂无用户授权" /></template></t-table></div></ResourceSection>
        <ResourceSection title="媒体账号"><div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="references.media_accounts || []" :columns="accountReferenceColumns" :scroll="{ x: '510px' }" row-key="account_id" size="small"><template #platform="{ row }">{{ platformLabel(row.platform) }}</template><template #name="{ row }">{{ row.name || row.account_id }}</template><template #empty><ResourceEmptyState title="暂无账号关联" /></template></t-table></div></ResourceSection>
      </template>
    </ResourceOverlay>
    </div>
  </t-loading>
</template>
