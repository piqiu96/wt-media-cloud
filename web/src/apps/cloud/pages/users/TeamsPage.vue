<script setup>
import { computed, onMounted, ref } from 'vue'
import { createUsersClient } from './usersApi.js'

const client = createUsersClient()
const teams = ref([])
const users = ref([])
const keyword = ref('')
const loading = ref(true)
const error = ref('')
const pagination = ref({ current: 1, pageSize: 10 })
const dialogVisible = ref(false)
const editingTeam = ref(null)
const teamName = ref('')
const saving = ref(false)

const filteredTeams = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return teams.value
  return teams.value.filter((team) => String(team.id).includes(kw) || team.name.toLowerCase().includes(kw))
})
const pagedTeams = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return filteredTeams.value.slice(start, start + pagination.value.pageSize)
})

const columns = [
  { colKey: 'id', title: '分组ID', width: 100 },
  { colKey: 'name', title: '分组名称' },
  { colKey: 'user_count', title: '用户数', width: 100 },
  { colKey: 'operations', title: '操作', width: 180 },
]

onMounted(loadAll)

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [teamRows, userRows] = await Promise.all([client.listTeams(), client.listUsers()])
    users.value = userRows || []
    teams.value = (teamRows || []).map((team) => ({
      ...team,
      user_count: users.value.filter((user) => user.team_id === team.id).length,
    }))
    pagination.value.current = 1
  } catch (e) {
    error.value = e.message || '加载运营分组失败'
  } finally {
    loading.value = false
  }
}

function search() {
  pagination.value.current = 1
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
    error.value = e.message || '保存运营分组失败'
  } finally {
    saving.value = false
  }
}

async function deleteTeam(team) {
  if (team.user_count > 0) {
    error.value = `该分组下还有 ${team.user_count} 个用户，不能删除，请先转移用户`
    return
  }
  if (!window.confirm(`确定删除运营分组 ${team.name}？只有空分组可以删除。`)) return
  try {
    await client.deleteTeam(team.id)
    await loadAll()
  } catch (e) {
    error.value = e.message || '删除运营分组失败'
  }
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <t-card title="运营分组" :bordered="true">
      <template #actions>
        <t-button theme="primary" size="small" @click="openCreate">新建分组</t-button>
        <t-button size="small" style="margin-left:8px" @click="loadAll">刷新</t-button>
      </template>
      <div class="filter-row">
        <t-input v-model="keyword" placeholder="搜索分组ID或名称" clearable @change="search" />
        <t-button size="small" @click="search">查询</t-button>
      </div>
      <t-table :data="pagedTeams" :columns="columns" row-key="id" size="small" hover>
        <template #operations="{ row }">
          <t-space>
            <t-button size="small" variant="text" @click="openRename(row)">重命名</t-button>
            <t-button size="small" variant="text" theme="danger" :disabled="row.user_count > 0" @click="deleteTeam(row)">删除</t-button>
          </t-space>
        </template>
      </t-table>
      <div class="pagination-row">
        <t-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="filteredTeams.length" :page-size-options="[10, 20, 50]" />
      </div>
    </t-card>
    <t-dialog v-model:visible="dialogVisible" :header="editingTeam ? '重命名运营分组' : '新建运营分组'" :confirm-btn="{ loading: saving }" @confirm="saveTeam">
      <t-form><t-form-item label="分组名称"><t-input v-model="teamName" /></t-form-item></t-form>
    </t-dialog>
  </t-loading>
</template>

<style scoped>
.filter-row { display:flex; gap:8px; align-items:center; margin-bottom:16px; max-width:420px; }
.pagination-row { display:flex; justify-content:flex-end; margin-top:16px; }
</style>
