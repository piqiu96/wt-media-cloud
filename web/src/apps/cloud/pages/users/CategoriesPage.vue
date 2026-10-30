<script setup>
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
const referencesVisible = ref(false)
const referenceCategory = ref(null)
const references = ref({ users: [], media_accounts: [] })
const referencesLoading = ref(false)

const pagedCategories = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return categories.value.slice(start, start + pagination.value.pageSize)
})

// 高级运营能看全部状态，但只读，也不拿关联摘要（关联详情仍归管理员）。
const canManage = computed(() => isAdminRole(currentUser.value))

const columns = computed(() => {
  const base = [
    { colKey: 'id', title: '类目ID', width: 100 },
    { colKey: 'type', title: '一级类型', width: 110 },
    { colKey: 'name', title: '类目名称', width: 220 },
  ]
  if (canManage.value) base.push({ colKey: 'references', title: '关联情况', width: 190 })
  base.push({ colKey: 'enabled', title: '状态', width: 100 })
  base.push({ colKey: 'remark', title: '备注', width: 220 })
  if (canManage.value) base.push({ colKey: 'operations', title: '操作', width: 220, fixed: 'right' })
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
  loadCategories()
}

// quiet 由定时刷新传入：保留页码与筛选，失败沿用上一次读数。
async function loadCategories({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    // management：全部启停状态、不含关联摘要；all：管理员口径，带摘要。
    const scope = canManage.value ? 'all' : 'management'
    categories.value = await client.listCategories({ ...filters.value, scope }) || []
    if (!quiet) pagination.value.current = 1
  } catch (e) {
    if (!quiet) error.value = e.message || '加载类目失败'
  } finally {
    if (!quiet) loading.value = false
  }
}

useAutoRefresh(() => loadCategories({ quiet: true }), {
  shouldSkip: () => dialogVisible.value || saving.value || referencesVisible.value || referencesLoading.value,
})

function referenceSummary(category) {
  return category.reference_summary || { user_scope_count: 0, media_account_count: 0, total_count: 0 }
}

function hasReferences(category) {
  return referenceSummary(category).total_count > 0
}

async function openReferences(category) {
  if (!canManage.value) return
  referenceCategory.value = category
  references.value = { users: [], media_accounts: [] }
  referencesVisible.value = true
  referencesLoading.value = true
  try {
    references.value = await client.getCategoryReferences(category.id)
  } catch (e) {
    error.value = e.message || '加载关联详情失败'
  } finally {
    referencesLoading.value = false
  }
}

function openCreate() {
  editingCategory.value = null
  categoryForm.value = emptyCategoryForm()
  dialogVisible.value = true
}

function openEdit(category) {
  editingCategory.value = category
  categoryForm.value = { type: category.type, name: category.name, enabled: category.enabled, remark: category.remark || '' }
  dialogVisible.value = true
}

async function saveCategory() {
  if (!categoryForm.value.type || !categoryForm.value.name.trim()) {
    error.value = '请选择一级类型并填写类目名称'
    return
  }
  saving.value = true
  error.value = ''
  try {
    if (editingCategory.value) await client.updateCategory(editingCategory.value.id, categoryForm.value)
    else await client.createCategory(categoryForm.value)
    dialogVisible.value = false
    await loadCategories()
  } catch (e) {
    error.value = e.message || '保存类目失败'
  } finally {
    saving.value = false
  }
}

async function toggleCategory(category) {
  if (category.enabled && hasReferences(category)) {
    error.value = '该类目仍有关联，请先查看关联详情并解除用户授权或媒体账号引用'
    return
  }
  try {
    await client.updateCategory(category.id, {
      name: category.name,
      remark: category.remark,
      enabled: !category.enabled,
    })
    await loadCategories()
  } catch (e) {
    error.value = e.message || '更新类目状态失败'
  }
}

async function deleteCategory(category) {
  if (hasReferences(category)) {
    error.value = '该类目仍有关联，请先查看关联详情并解除用户授权或媒体账号引用'
    return
  }
  if (!await feedback.confirmWarning({ title: '删除类目', message: `确定删除类目 ${category.name}？已有用户或业务引用时不能删除。`, confirmText: '删除', danger: true })) return
  try {
    await client.deleteCategory(category.id)
    await loadCategories()
  } catch (e) {
    error.value = e.message || '删除类目失败'
  }
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <div class="wt-resource-page resource-visual-approved">
    <ResourcePageHeader title="类目管理" description="维护类目分类与关联范围">
      <template #actions>
        <t-button v-if="canManage" theme="primary" @click="openCreate">新增类目</t-button>
      </template>
    </ResourcePageHeader>
    <FeedbackAlert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <ResourceCard class="system-resource-card">
      <div class="filter-bar">
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="filters.keyword" placeholder="类目 ID / 名称" clearable @enter="loadCategories" /></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">一级类型</span><t-select v-model="filters.type" placeholder="全部" clearable>
          <t-option v-for="option in CATEGORY_TYPE_OPTIONS" :key="option.value" :value="option.value" :label="option.label" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.enabled" placeholder="全部" clearable>
          <t-option :value="true" label="启用" />
          <t-option :value="false" label="停用" />
        </t-select></label>
        <div class="wt-filter-actions"><t-button theme="primary" @click="loadCategories">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button></div>
      </div>
      <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="pagedCategories" :columns="columns" row-key="id" size="small" hover :scroll="tableScroll">
        <template #references="{ row }">
          <t-link v-if="hasReferences(row)" theme="primary" @click="openReferences(row)">用户 {{ referenceSummary(row).user_scope_count }} / 账号 {{ referenceSummary(row).media_account_count }}</t-link>
          <span v-else>无关联</span>
        </template>
        <template #type="{ row }">{{ categoryTypeLabel(row.type) }}</template>
        <template #enabled="{ row }"><ResourceStatusBadge :tone="row.enabled ? 'success' : 'neutral'" :label="row.enabled ? '启用' : '停用'" /></template>
        <template #operations="{ row }">
          <t-space>
            <t-button size="small" variant="text" @click="openEdit(row)">编辑</t-button>
            <t-button size="small" variant="text" :disabled="row.enabled && hasReferences(row)" @click="toggleCategory(row)">{{ row.enabled ? '停用' : '启用' }}</t-button>
            <t-button size="small" variant="text" theme="danger" :disabled="hasReferences(row)" @click="deleteCategory(row)">删除</t-button>
          </t-space>
        </template>
      </t-table></div>
      <div class="pagination-bar">
        <t-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="categories.length" :page-size-options="[10, 20, 50]" />
      </div>
    </ResourceCard>
    <t-dialog v-model:visible="dialogVisible" :header="editingCategory ? '编辑类目' : '新增类目'" :confirm-btn="{ loading: saving }" @confirm="saveCategory">
      <t-form>
        <t-form-item v-if="editingCategory" label="类目ID"><t-input :model-value="String(editingCategory.id)" disabled /></t-form-item>
        <t-form-item label="一级类型"><BusinessSelect kind="category" v-model="categoryForm.type" :disabled="!!editingCategory">
          <t-option v-for="option in CATEGORY_TYPE_OPTIONS" :key="option.value" :value="option.value" :label="option.label" />
        </BusinessSelect></t-form-item>
        <t-form-item label="类目名称"><t-input v-model="categoryForm.name" placeholder="例如：游戏.三角洲" /></t-form-item>
        <t-form-item v-if="editingCategory" label="状态"><t-switch v-model="categoryForm.enabled" :label="['停用', '启用']" /></t-form-item>
        <t-form-item label="备注"><t-textarea v-model="categoryForm.remark" /></t-form-item>
      </t-form>
    </t-dialog>
    <t-drawer :close-btn="true" v-model:visible="referencesVisible" :header="`${referenceCategory?.name || ''} 的关联详情`" size="min(92vw, 760px)" :footer="false">
      <t-loading :loading="referencesLoading">
        <div class="reference-section">用户授权</div>
        <t-table :data="references.users || []" size="small" row-key="user_id" :columns="[
          { colKey: 'user_id', title: 'UID', width: 80 }, { colKey: 'username', title: '用户名' }, { colKey: 'role', title: '角色' }, { colKey: 'team_name', title: '运营组' }
        ]" />
        <div class="reference-section">媒体账号</div>
        <t-table :data="references.media_accounts || []" size="small" row-key="account_id" :columns="[
          { colKey: 'account_id', title: '账号ID', width: 90 }, { colKey: 'name', title: '账号名称' }, { colKey: 'platform', title: '平台' }, { colKey: 'username', title: '归属用户' }
        ]">
          <template #platform="{ row }">{{ platformLabel(row.platform) }}</template>
        </t-table>
      </t-loading>
    </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.reference-section { margin: 16px 0 8px; font-weight: 600; }
</style>
