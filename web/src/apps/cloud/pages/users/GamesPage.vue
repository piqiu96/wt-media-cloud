<script setup>
import FeedbackAlert from '../../../../shared/ui/feedback/FeedbackAlert.vue'
import { feedback } from '../../../../shared/ui/feedback/index.js'
import { computed, onMounted, ref } from 'vue'
import { createUsersClient } from './usersApi.js'
import { createSessionClient } from '../../../../shared/api/session.js'
import { isAdminRole } from '../../../../shared/utils/roles.js'
import { platformLabel } from '../../../../shared/utils/platforms.js'
import ResourceCard from '../../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../../shared/ui/resource/ResourceStatusBadge.vue'

const client = createUsersClient()
const sessionClient = createSessionClient()
const currentUser = ref(null)
const games = ref([])
const loading = ref(true)
const error = ref('')
const filters = ref({ keyword: '', status: '' })
const pagination = ref({ current: 1, pageSize: 10 })
const dialogVisible = ref(false)
const editingGame = ref(null)
const gameForm = ref(emptyGameForm())
const saving = ref(false)
const referencesVisible = ref(false)
const referenceGame = ref(null)
const references = ref({ users: [], media_accounts: [] })
const referencesLoading = ref(false)

const pagedGames = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return games.value.slice(start, start + pagination.value.pageSize)
})

// 高级运营能看全部状态，但只读，也不拿关联摘要（关联详情仍归管理员）。
const canManage = computed(() => isAdminRole(currentUser.value))

const columns = computed(() => {
  const base = [
    { colKey: 'id', title: '游戏ID', width: 160 },
    { colKey: 'name', title: '游戏名称', width: 200 },
  ]
  if (canManage.value) base.push({ colKey: 'references', title: '关联情况', width: 190 })
  base.push({ colKey: 'status', title: '状态', width: 100 })
  base.push({ colKey: 'remark', title: '备注', width: 220 })
  if (canManage.value) base.push({ colKey: 'operations', title: '操作', width: 220, fixed: 'right' })
  return base
})

const tableScroll = computed(() => ({ x: `${columns.value.reduce((sum, col) => sum + col.width, 0)}px` }))

onMounted(async () => {
  await loadSession()
  await loadGames()
})

async function loadSession() {
  try {
    currentUser.value = await sessionClient.me()
  } catch {
    // 读不到会话按只读处理：senior 的 management 视图不比管理员视图多给任何写入口。
    currentUser.value = null
  }
}

function emptyGameForm() {
  return { id: '', name: '', status: 'enabled', remark: '' }
}

function resetFilters() {
  filters.value = { keyword: '', status: '' }
  loadGames()
}

async function loadGames() {
  loading.value = true
  error.value = ''
  try {
    // management：全部状态、不含关联摘要；all：管理员口径，带摘要。
    const scope = canManage.value ? 'all' : 'management'
    games.value = await client.listGames({ ...filters.value, scope }) || []
    pagination.value.current = 1
  } catch (e) {
    error.value = e.message || '加载游戏失败'
  } finally {
    loading.value = false
  }
}

function referenceSummary(game) {
  return game.reference_summary || { user_scope_count: 0, media_account_count: 0, total_count: 0 }
}

function hasReferences(game) {
  return referenceSummary(game).total_count > 0
}

async function openReferences(game) {
  if (!canManage.value) return
  referenceGame.value = game
  references.value = { users: [], media_accounts: [] }
  referencesVisible.value = true
  referencesLoading.value = true
  try {
    references.value = await client.getGameReferences(game.id)
  } catch (e) {
    error.value = e.message || '加载关联详情失败'
  } finally {
    referencesLoading.value = false
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
  if (game.status === 'enabled' && hasReferences(game)) {
    error.value = '该游戏仍有关联，请先查看关联详情并解除用户授权或媒体账号引用'
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
  if (hasReferences(game)) {
    error.value = '该游戏仍有关联，请先查看关联详情并解除用户授权或媒体账号引用'
    return
  }
  if (!await feedback.confirmWarning({ title: '删除游戏', message: `确定删除游戏 ${game.name}？已有用户或业务引用时不能删除。`, confirmText: '删除', danger: true })) return
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
    <div class="wt-resource-page">
    <ResourcePageHeader title="游戏管理" description="维护游戏分类与关联范围">
      <template #actions>
        <t-button v-if="canManage" theme="primary" @click="openCreate">新增游戏</t-button>
        <t-button class="wt-secondary-button" variant="outline" @click="loadGames">刷新</t-button>
      </template>
    </ResourcePageHeader>
    <FeedbackAlert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <ResourceCard class="system-resource-card">
      <div class="filter-bar">
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="filters.keyword" placeholder="游戏 ID / 名称" clearable @enter="loadGames" /></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">状态</span><t-select v-model="filters.status" placeholder="全部" clearable>
          <t-option value="enabled" label="启用" />
          <t-option value="disabled" label="停用" />
        </t-select></label>
        <div class="wt-filter-actions"><t-button theme="primary" @click="loadGames">查询</t-button><t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button></div>
      </div>
      <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="pagedGames" :columns="columns" row-key="id" size="small" hover :scroll="tableScroll">
        <template #references="{ row }">
          <t-link v-if="hasReferences(row)" theme="primary" @click="openReferences(row)">用户 {{ referenceSummary(row).user_scope_count }} / 账号 {{ referenceSummary(row).media_account_count }}</t-link>
          <span v-else>无关联</span>
        </template>
        <template #status="{ row }"><ResourceStatusBadge :tone="row.status === 'enabled' ? 'success' : 'neutral'" :label="row.status === 'enabled' ? '启用' : '停用'" /></template>
        <template #operations="{ row }">
          <t-space>
            <t-button size="small" variant="text" @click="openEdit(row)">编辑</t-button>
            <t-button size="small" variant="text" :disabled="row.status === 'enabled' && hasReferences(row)" @click="toggleGame(row)">{{ row.status === 'enabled' ? '停用' : '启用' }}</t-button>
            <t-button size="small" variant="text" theme="danger" :disabled="hasReferences(row)" @click="deleteGame(row)">删除</t-button>
          </t-space>
        </template>
      </t-table></div>
      <div class="pagination-bar">
        <t-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="games.length" :page-size-options="[10, 20, 50]" />
      </div>
    </ResourceCard>
    <t-dialog v-model:visible="dialogVisible" :header="editingGame ? '编辑游戏' : '新增游戏'" :confirm-btn="{ loading: saving }" @confirm="saveGame">
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
    <t-drawer :close-btn="true" v-model:visible="referencesVisible" :header="`${referenceGame?.name || ''} 的关联详情`" size="min(92vw, 760px)" :footer="false">
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
