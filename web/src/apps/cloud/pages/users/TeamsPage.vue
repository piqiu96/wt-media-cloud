<script setup>
import FeedbackAlert from '../../../../shared/ui/feedback/FeedbackAlert.vue'
import { feedback } from '../../../../shared/ui/feedback/index.js'
import { computed, onMounted, ref } from 'vue'
import { createUsersClient } from './usersApi.js'
import { createSessionClient } from '../../../../shared/api/session.js'
import { isAdminRole } from '../../../../shared/utils/roles.js'
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
    { colKey: 'name', title: '团队名称', width: 240 },
    { colKey: 'user_count', title: '用户数', width: 100 },
  ]
  if (canManage.value) base.push({ colKey: 'operations', title: '操作', width: 180, fixed: 'right' })
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

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    // 关键字（团队 ID 精确 / 名称片段）由服务端过滤，页面只做分页切片。
    const teamRows = await client.listTeams({ keyword: keyword.value.trim() })
    teams.value = (teamRows || []).map((team) => ({ ...team, user_count: team.user_count ?? 0 }))
    pagination.value.current = 1
  } catch (e) {
    error.value = e.message || '加载运营团队失败'
  } finally {
    loading.value = false
  }
}

function search() {
  loadAll()
}

function resetFilters() {
  keyword.value = ''
  loadAll()
}

function openCreate() {
  editingTeam.value = null
  teamName.value = ''
  dialogVisible.value = true
}

function openRename(team) {
  editingTeam.value = team
  teamName.value = team.name
  dialogVisible.value = true
}

async function saveTeam() {
  saving.value = true
  error.value = ''
  try {
    if (editingTeam.value) await client.renameTeam(editingTeam.value.id, teamName.value)
    else await client.createTeam(teamName.value)
    dialogVisible.value = false
    await loadAll()
  } catch (e) {
    error.value = e.message || '保存运营团队失败'
  } finally {
    saving.value = false
  }
}

async function deleteTeam(team) {
  if (team.user_count > 0) {
    error.value = `该团队下还有 ${team.user_count} 个用户，不能删除，请先转移用户`
    return
  }
  if (!await feedback.confirmWarning({ title: '删除运营团队', message: `确定删除运营团队 ${team.name}？只有空团队可以删除。`, confirmText: '删除', danger: true })) return
  try {
    await client.deleteTeam(team.id)
    await loadAll()
  } catch (e) {
    error.value = e.message || '删除运营团队失败'
  }
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <div class="wt-resource-page">
    <ResourcePageHeader title="运营团队" description="管理运营人员的团队归属">
      <template #actions>
        <t-button v-if="canManage" theme="primary" @click="openCreate">新增团队</t-button>
        <t-button class="wt-secondary-button" variant="outline" @click="loadAll">刷新</t-button>
      </template>
    </ResourcePageHeader>
    <FeedbackAlert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <ResourceCard class="system-resource-card">
      <div class="filter-bar">
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="keyword" placeholder="团队 ID / 名称" clearable @enter="search" /></label>
        <div class="wt-filter-actions"><t-button theme="primary" @click="search">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button></div>
      </div>
      <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="pagedTeams" :columns="columns" row-key="id" size="small" hover :scroll="tableScroll">
        <template #operations="{ row }">
          <t-space>
            <t-button size="small" variant="text" @click="openRename(row)">重命名</t-button>
            <t-button size="small" variant="text" theme="danger" :disabled="row.user_count > 0" @click="deleteTeam(row)">删除</t-button>
          </t-space>
        </template>
      </t-table></div>
      <div class="pagination-bar">
        <t-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="teams.length" :page-size-options="[10, 20, 50]" />
      </div>
    </ResourceCard>
    <t-dialog v-model:visible="dialogVisible" :header="editingTeam ? '重命名运营团队' : '新增运营团队'" :confirm-btn="{ loading: saving }" @confirm="saveTeam">
      <t-form><t-form-item label="团队名称"><t-input v-model="teamName" /></t-form-item></t-form>
    </t-dialog>
    </div>
  </t-loading>
</template>

<style scoped>
</style>
