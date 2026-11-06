<script setup>
import ResourceFilterBar from '../../../../shared/ui/resource/ResourceFilterBar.vue'
import ResourceRowActions from '../../../../shared/ui/resource/ResourceRowActions.vue'
import ResourceOverlay from '../../../../shared/ui/resource/ResourceOverlay.vue'
import ResourceForm from '../../../../shared/ui/resource/ResourceForm.vue'
import { opColumnWidth } from '../../../../shared/ui/resource/opColumnWidth.js'
import { usePendingActions, useEditorClose } from './managementInteractions.js'

import FeedbackAlert from '../../../../shared/ui/feedback/FeedbackAlert.vue'
import { feedback } from '../../../../shared/ui/feedback/index.js'
import { computed, onMounted, ref } from 'vue'
import { createUsersClient } from './usersApi.js'
import { createSessionClient } from '../../../../shared/api/session.js'
import { isAdminRole } from '../../../../shared/utils/roles.js'
import { useAutoRefresh } from '../../../../shared/utils/useAutoRefresh.js'
import ResourceCard from '../../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../../shared/ui/resource/ResourcePageHeader.vue'

const client = createUsersClient()
const sessionClient = createSessionClient()
const teams = ref([])
const currentUser = ref(null)
const keyword = ref('')
const loading = ref(true)
const error = ref('')
const pagination = ref({ current: 1, pageSize: 10 })
const dialogVisible = ref(false)
const editingTeam = ref(null)
const teamName = ref('')
const saving = ref(false)
const formError = ref(''), nameError = ref('')
const { pending: pendingActions, run: runAction } = usePendingActions()
const editorClose = useEditorClose({ visible: dialogVisible, busy: saving, draft: () => teamName.value, confirm: feedback.confirmWarning })

const pagedTeams = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return teams.value.slice(start, start + pagination.value.pageSize)
})

// 高级运营能进这一页但只读；人数由服务端随团队一起返回，不再读用户列表
// （那条接口对高级运营是 403，人数会整列变成 0）。
const canManage = computed(() => isAdminRole(currentUser.value))

const columns = computed(() => {
  const base = [
    { colKey: 'id', title: '团队ID', width: 100 },
    { colKey: 'name', title: '团队名称', width: 240, ellipsis: true },
    { colKey: 'user_count', title: '用户数', width: 100 },
  ]
  if (canManage.value) base.push({ colKey: 'operations', title: '操作', width: opColumnWidth(['重命名', '删除'], { variant: 'text', loading: true }), fixed: 'right' })
  return base
})

const tableScroll = computed(() => ({ x: `${columns.value.reduce((sum, col) => sum + col.width, 0)}px` }))

onMounted(async () => {
  await loadSession()
  await loadAll()
})

async function loadSession() {
  try {
    currentUser.value = await sessionClient.me()
  } catch {
    // 读不到会话就按只读处理：写入口宁可少给，也不给错。
    currentUser.value = null
  }
}

// quiet 由定时刷新传入：保留页码与关键字，失败沿用上一次读数。
async function loadAll({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  if (!quiet) error.value = ''
  try {
    // 关键字（团队 ID 精确 / 名称片段）由服务端过滤，页面只做分页切片。
    const teamRows = await client.listTeams({ keyword: keyword.value.trim() })
    teams.value = (teamRows || []).map((team) => ({ ...team, user_count: team.user_count ?? 0 }))
    pagination.value.current = Math.min(pagination.value.current, Math.max(1, Math.ceil(teams.value.length / pagination.value.pageSize)))
  } catch (e) {
    if (!quiet) error.value = e.message || '加载运营团队失败'
  } finally {
    if (!quiet) loading.value = false
  }
}

useAutoRefresh(() => loadAll({ quiet: true }), {
  shouldSkip: () => dialogVisible.value || saving.value || pendingActions.value.size > 0,
})

function search() {
  pagination.value.current = 1
  loadAll()
}

function resetFilters() {
  keyword.value = ''
  search()
}

function openCreate() {
  editingTeam.value = null
  teamName.value = ''
  formError.value = ''; nameError.value = ''
  editorClose.capture()
  dialogVisible.value = true
}

function openRename(team) {
  editingTeam.value = team
  teamName.value = team.name
  formError.value = ''; nameError.value = ''
  editorClose.capture()
  dialogVisible.value = true
}

async function saveTeam() {
  if (saving.value) return
  nameError.value = !teamName.value.trim() ? '请输入团队名称' : [...teamName.value.trim()].length > 64 ? '团队名称最多64个字符' : ''
  if (nameError.value) return
  saving.value = true
  formError.value = ''
  try {
    if (editingTeam.value) await client.renameTeam(editingTeam.value.id, teamName.value.trim())
    else await client.createTeam(teamName.value.trim())
    dialogVisible.value = false
    feedback.toast.success('已保存团队')
    await loadAll()
  } catch (e) {
    formError.value = e.message || '保存运营团队失败'
  } finally {
    saving.value = false
  }
}

async function deleteTeam(team) {
  return runAction(team.id, async () => {
    if (team.user_count > 0) {
      error.value = `该团队下还有 ${team.user_count} 个用户，不能删除，请先转移用户`
      return
    }
    if (!await feedback.confirmWarning({ title: '删除运营团队', message: `确定删除运营团队 ${team.name}？有用户或历史业务数据的团队不能删除。`, confirmText: '删除', danger: true })) return
    try {
      await client.deleteTeam(team.id)
      await loadAll()
    } catch (e) {
      error.value = e.message || '删除运营团队失败'
    }
  })
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <div class="wt-resource-page">
    <ResourcePageHeader title="运营团队" description="管理运营人员的团队归属">
      <template #actions>
        <t-button v-if="canManage" theme="primary" @click="openCreate">新增团队</t-button>
      </template>
    </ResourcePageHeader>
    <FeedbackAlert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <ResourceCard class="system-resource-card">
      <ResourceFilterBar>
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="keyword" placeholder="团队 ID / 名称" clearable @enter="search" /></label>
        <div class="wt-filter-actions"><t-button theme="primary" @click="search">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button></div>
      </ResourceFilterBar>
      <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="pagedTeams" :columns="columns" row-key="id" size="small" hover :scroll="tableScroll">
        <template #operations="{ row }">
          <ResourceRowActions>
            <t-button size="small" variant="text" :disabled="pendingActions.has(row.id)" @click="openRename(row)">重命名</t-button>
            <t-tooltip :content="row.user_count > 0 ? '团队内仍有用户，暂不可删除' : '有历史业务数据的团队不能删除'">
              <span><t-button size="small" variant="text" class="is-danger" :loading="pendingActions.has(row.id)" :disabled="row.user_count > 0 || pendingActions.has(row.id)" @click="deleteTeam(row)">删除</t-button></span>
            </t-tooltip>
          </ResourceRowActions>
        </template>
      </t-table></div>
      <div class="pagination-bar">
        <t-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="teams.length" :page-size-options="[10, 20, 50]" />
      </div>
    </ResourceCard>
    <ResourceOverlay :visible="dialogVisible" :title="editingTeam ? '重命名团队' : '新增团队'" :busy="saving" @update:visible="value => { if (!value) editorClose.close() }">
      <FeedbackAlert style="margin-bottom:16px" v-if="formError" :message="formError" theme="error" />
      <ResourceForm @submit="saveTeam">
        <t-form-item label="团队名称" required-mark :status="nameError ? 'error' : undefined" :tips="nameError">
          <t-input v-model="teamName" placeholder="输入团队名称" :maxlength="64" :disabled="saving" @enter="saveTeam" />
        </t-form-item>
      </ResourceForm>
      <template #footer><t-button variant="outline" :disabled="saving" @click="editorClose.close">取消</t-button><t-button theme="primary" :loading="saving" @click="saveTeam">{{ editingTeam ? '保存' : '创建' }}</t-button></template>
    </ResourceOverlay>
    </div>
  </t-loading>
</template>
