<script setup>
import { computed, onMounted, ref } from 'vue'
import { createUsersClient } from './usersApi.js'

const client = createUsersClient()
const games = ref([])
const users = ref([])
const loading = ref(true)
const error = ref('')
const filters = ref({ keyword: '', status: '' })
const pagination = ref({ current: 1, pageSize: 10 })
const dialogVisible = ref(false)
const editingGame = ref(null)
const gameForm = ref(emptyGameForm())
const saving = ref(false)

const pagedGames = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return games.value.slice(start, start + pagination.value.pageSize)
})

const columns = [
  { colKey: 'id', title: '游戏ID', width: 160 },
  { colKey: 'name', title: '游戏名称' },
  { colKey: 'user_count', title: '用户数', width: 100 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'remark', title: '备注' },
  { colKey: 'operations', title: '操作', width: 220 },
]

onMounted(loadGames)

function emptyGameForm() {
  return { id: '', name: '', status: 'enabled', remark: '' }
}

async function loadGames() {
  loading.value = true
  error.value = ''
  try {
    const [gameRows, userRows] = await Promise.all([client.listGames(filters.value), client.listUsers()])
    users.value = userRows || []
    games.value = (gameRows || []).map((game) => ({
      ...game,
      user_count: users.value.filter((user) => Array.isArray(user.game_ids) && user.game_ids.includes(game.id)).length,
    }))
    pagination.value.current = 1
  } catch (e) {
    error.value = e.message || '加载游戏失败'
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingGame.value = null
  gameForm.value = emptyGameForm()
  dialogVisible.value = true
}

function openEdit(game) {
  editingGame.value = game
  gameForm.value = { id: game.id, name: game.name, status: game.status, remark: game.remark || '' }
  dialogVisible.value = true
}

async function saveGame() {
  if (!editingGame.value && !/^[A-Za-z0-9]{1,32}$/.test(gameForm.value.id)) {
    error.value = '游戏ID仅支持32位以内字母或数字'
    return
  }
  saving.value = true
  error.value = ''
  try {
    if (editingGame.value) await client.updateGame(editingGame.value.id, gameForm.value)
    else await client.createGame(gameForm.value)
    dialogVisible.value = false
    await loadGames()
  } catch (e) {
    error.value = e.message || '保存游戏失败'
  } finally {
    saving.value = false
  }
}

async function toggleGame(game) {
  if (game.status === 'enabled' && game.user_count > 0) {
    error.value = `该游戏已分配给 ${game.user_count} 个用户，不能停用，请先调整用户游戏范围`
    return
  }
  try {
    await client.updateGame(game.id, {
      name: game.name,
      remark: game.remark,
      status: game.status === 'enabled' ? 'disabled' : 'enabled',
    })
    await loadGames()
  } catch (e) {
    error.value = e.message || '更新游戏状态失败'
  }
}

async function deleteGame(game) {
  if (game.user_count > 0) {
    error.value = `该游戏已分配给 ${game.user_count} 个用户，不能删除，请先调整用户游戏范围`
    return
  }
  if (!window.confirm(`确定删除游戏 ${game.name}？已有用户或业务引用时不能删除。`)) return
  try {
    await client.deleteGame(game.id)
    await loadGames()
  } catch (e) {
    error.value = e.message || '删除游戏失败'
  }
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <t-card title="游戏管理" :bordered="true">
      <template #actions>
        <t-button theme="primary" size="small" @click="openCreate">新建游戏</t-button>
        <t-button size="small" style="margin-left:8px" @click="loadGames">刷新</t-button>
      </template>
      <div class="filter-row">
        <t-input v-model="filters.keyword" placeholder="搜索游戏ID或名称" clearable />
        <t-select v-model="filters.status" placeholder="状态" clearable>
          <t-option value="enabled" label="启用" />
          <t-option value="disabled" label="停用" />
        </t-select>
        <t-button size="small" @click="loadGames">查询</t-button>
      </div>
      <t-table :data="pagedGames" :columns="columns" row-key="id" size="small" hover>
        <template #status="{ row }"><t-tag :theme="row.status === 'enabled' ? 'success' : 'danger'" size="small">{{ row.status === 'enabled' ? '启用' : '停用' }}</t-tag></template>
        <template #operations="{ row }">
          <t-space>
            <t-button size="small" variant="text" @click="openEdit(row)">编辑</t-button>
            <t-button size="small" variant="text" :disabled="row.status === 'enabled' && row.user_count > 0" @click="toggleGame(row)">{{ row.status === 'enabled' ? '停用' : '启用' }}</t-button>
            <t-button size="small" variant="text" theme="danger" :disabled="row.user_count > 0" @click="deleteGame(row)">删除</t-button>
          </t-space>
        </template>
      </t-table>
      <div class="pagination-row">
        <t-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="games.length" :page-size-options="[10, 20, 50]" />
      </div>
    </t-card>
    <t-dialog v-model:visible="dialogVisible" :header="editingGame ? '编辑游戏' : '新建游戏'" :confirm-btn="{ loading: saving }" @confirm="saveGame">
      <t-form>
        <t-form-item label="游戏ID"><t-input v-model="gameForm.id" :disabled="!!editingGame" placeholder="仅支持32位以内字母或数字，例如 naruto01" /></t-form-item>
        <t-form-item label="游戏名称"><t-input v-model="gameForm.name" placeholder="例如：火影忍者" /></t-form-item>
        <t-form-item v-if="editingGame" label="状态">
          <t-select v-model="gameForm.status">
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
          </t-select>
        </t-form-item>
        <t-form-item label="备注"><t-textarea v-model="gameForm.remark" /></t-form-item>
      </t-form>
    </t-dialog>
  </t-loading>
</template>

<style scoped>
.filter-row { display:grid; grid-template-columns:minmax(180px, 1fr) minmax(120px, 180px) auto; gap:8px; align-items:center; margin-bottom:16px; max-width:620px; }
.pagination-row { display:flex; justify-content:flex-end; margin-top:16px; }
</style>
