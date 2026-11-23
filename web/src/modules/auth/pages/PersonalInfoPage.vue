<script setup>
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { feedback } from '../../../shared/ui/feedback/index.js'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { createSessionClient } from '../../../shared/api/session.js'
import { createBitAccountClient } from '../../../shared/api/bitAccount.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import { bitAccountDiffers as computeBitAccountDiffers } from '../../../apps/desktop/features/local-agent/work-env-status.js'
import { createLocalAgentStatusPage } from '../../../apps/desktop/features/local-agent/local-agent-status.js'
import { localAgentStateFromStatus } from '../../../apps/desktop/features/local-agent/service.js'
import { canRunLocalAgent } from '../../../utils.js'
import { avatarOptions } from '../../../shared/ui/systemAvatars.js'
import SystemAvatar from '../../../shared/ui/SystemAvatar.vue'

import { onBeforeRouteLeave } from 'vue-router'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourceSection from '../../../shared/ui/resource/ResourceSection.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'

const editing = ref(false)
const avatarVisible = ref(false)
const profileError = ref('')
const bindingError = ref('')
const dirty = computed(() => editing.value && (nickname.value !== (user.value?.nickname || user.value?.username || '') || avatarId.value !== (user.value?.avatar_id || 'sky')))
function startEdit() {
  nickname.value = user.value.nickname || user.value.username
  avatarId.value = user.value.avatar_id || 'sky'
  profileError.value = ''
  editing.value = true
}
function cancelEdit() {
  editing.value = false
  avatarVisible.value = false
  profileError.value = ''
}
onBeforeRouteLeave(async (to) => {
  if (to.path === '/login') return true
  if (saving.value || bindingBusy.value) return false
  if (!dirty.value) return true
  return feedback.confirmWarning({title:'放弃未保存的修改？',message:'离开后，本次昵称和头像修改将不保存。',confirmText:'放弃修改'})
})
function beforeUnload(event) {
  if (dirty.value || saving.value) { event.preventDefault(); event.returnValue = '' }
}
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onUnmounted(() => window.removeEventListener('beforeunload', beforeUnload))
const session = createSessionClient()
const bitAccounts = createBitAccountClient()
// 团队/类目范围只做展示，取与素材库页同一个 categories 端点把类目 ID 映射成名称；
// 拉不到时退化为显示 ID，不阻塞整页加载。
const usersClient = createUsersClient()
const categories = ref([])
const user = ref(null)
const nickname = ref('')
const avatarId = ref('sky')
const binding = ref(null)
const localAgent = ref(null)
const deviceId = ref('')
const loading = ref(true)
const saving = ref(false)
const bindingBusy = ref(false)
// 本机比特账号变化后，用户必须主动确认换用当前账号。
const confirmRebind = ref(false)
const error = ref('')
const inDesktop = typeof window !== 'undefined' && window.__TAURI_INTERNALS__ !== undefined
const localAgentAllowed = computed(() => canRunLocalAgent(user.value))
// 管理员不受团队/类目限制；运营展示自己被授权的类目名称（找不到名称时退化为 ID）。
const categoryScopeLabel = computed(() => {
  if (user.value?.role === 'admin') return '全部类目'
  const ids = user.value?.category_ids || []
  if (!ids.length) return '—'
  return ids.map((id) => categories.value.find((category) => category.id === id)?.name || id).join('、')
})
// 本机账号与绑定账号一致时没有可绑定的东西，入口不出现；不一致才是
// 「客户端换过账号、系统待确认」。判定与顶栏胶囊同源（work-env-status），
// 两处只会同时变绿或同时变橙。
const bitAccountDiffers = computed(() => computeBitAccountDiffers(binding.value, localAgent.value?.main_user_id_masked))
// 本机读到的账号已由 Tauri 按 Cloud 同一规则掩掉，页面拿不到完整主账号。
const localAccountMasked = computed(() => localAgent.value?.main_user_id_masked || '')
const currentAccountText = computed(() => {
  if (!localAccountMasked.value) return '未读取到（请确认已登录比特浏览器）'
  return bitAccountDiffers.value ? `${localAccountMasked.value}（与绑定账号不一致）` : localAccountMasked.value
})
const bindingTitle = computed(() => (binding.value?.bound ? '已绑定比特浏览器账号' : '尚未绑定比特浏览器账号'))
const bindingSummary = computed(() => {
  if (!binding.value?.bound) return inDesktop ? '由本机比特浏览器读取后绑定' : '绑定需在 Desktop 应用中操作'
  if (bitAccountDiffers.value) return '当前比特浏览器登录的是另一个账号'
  if (localAccountMasked.value) return '当前比特浏览器已登录绑定账号'
  return '尚未读取到本机比特浏览器账号'
})
const agentStatusText = computed(() => localAgent.value
  ? createLocalAgentStatusPage(localAgentStateFromStatus(localAgent.value)).primaryStatus
  : '无法连接')
const OPERATING_SYSTEM_LABELS = { macos: 'macOS', windows: 'Windows', linux: 'Linux' }
const systemLabel = computed(() => {
  const os = localAgent.value?.operating_system || ''
  const parts = [OPERATING_SYSTEM_LABELS[os] || os, localAgent.value?.cpu_architecture || ''].filter(Boolean)
  return parts.length ? parts.join(' · ') : '未读取到'
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [currentUser, categoryRows] = await Promise.all([session.me(), usersClient.listCategories({ enabled: true }).catch(() => [])])
    user.value = currentUser
    if (!editing.value) {
      nickname.value = currentUser.nickname || currentUser.username
      avatarId.value = currentUser.avatar_id || 'sky'
    }
    categories.value = Array.isArray(categoryRows) ? categoryRows : []
    // 本机 Agent 与比特账号绑定给运营与高级运营；管理员不读取这类本机信息。
    if (!canRunLocalAgent(currentUser)) return
    bindingError.value = ''
    try { binding.value = await bitAccounts.get() }
    catch (e) { bindingError.value = e?.message || '无法读取绑定状态' }
    if (inDesktop) {
      const [agentResult, deviceResult] = await Promise.allSettled([
        invoke('local_agent_status'),
        invoke('local_device_id'),
      ])
      localAgent.value = agentResult.status === 'fulfilled' ? agentResult.value : null
      deviceId.value = deviceResult.status === 'fulfilled' ? deviceResult.value : ''
    }
  } catch (e) { error.value = e?.message || '无法加载账号信息' }
  finally { loading.value = false }
}
onMounted(load)

async function saveProfile() {
  if (saving.value) return
  if (!nickname.value.trim()) { profileError.value = '请输入昵称'; return }
  saving.value = true
  profileError.value = ''
  try {
    const updated = await session.updateProfile(nickname.value.trim(), avatarId.value)
    user.value = updated
    nickname.value = updated.nickname || updated.username
    avatarId.value = updated.avatar_id || 'sky'
    editing.value = false
    window.dispatchEvent(new Event('wt-media:profile-updated'))
    feedback.toast.success('账号资料已保存')
  } catch (e) { profileError.value = e?.message || '保存失败，请重试' }
  finally { saving.value = false }
}

// 绑定与换绑是同一个入口：主账号由本机 Agent 实读，页面只决定 `rebind`。
// 失败按 `code` 分支，尤其「账号不一致」要给出去换绑的下一步，而不是让用户
// 以为重试就能过。
const REFUSAL_TEXT = {
  BIT_MAIN_USER_MISMATCH: '当前比特浏览器登录账号与系统绑定账号不一致；如确需更换，请点击「换绑为当前账号」。',
  bit_browser_offline: '比特浏览器未运行或不可达，请先启动比特浏览器后重试。',
  BIT_BROWSER_OFFLINE: '比特浏览器未运行或不可达，请先启动比特浏览器后重试。',
  agent_unreachable: '本机服务不可达，请重启应用后重试。',
  cloud_request_failed: '请求未能完成，请稍后重试。',
  DESKTOP_LOGIN_REQUIRED: '登录状态已失效，请重新登录后再绑定。',
  AUTH_DESKTOP_REPLACED: '你的账号已在另一台电脑登录，请重新登录。',
  AUTH_AGENT_TOKEN_INVALID: '本机执行凭证无效，请重新登录后再绑定。',
  AUTH_AGENT_TOKEN_EXPIRED: '本机执行凭证已过期，请重新登录后再绑定。',
}

async function bindAccount({ rebind = false } = {}) {
  if (!inDesktop || bindingBusy.value) return
  bindingBusy.value = true
  error.value = ''
  try {
    const { bindBitBrowser } = await import('../../../apps/desktop/features/local-agent/init.js')
    await bindBitBrowser({ rebind })
    confirmRebind.value = false
    await load()
    feedback.toast.success(rebind ? '已换用当前比特浏览器账号；原有窗口记录保留，切回原账号即可恢复。' : '比特浏览器账号已绑定')
  } catch (e) {
    feedback.toast.error(REFUSAL_TEXT[e?.code] || e?.message || String(e))
  } finally { bindingBusy.value = false }
}
</script>

<template>
  <main class="personal-page">
    <ResourcePageHeader title="我的账号" />
    <FeedbackAlert v-if="error" theme="error" :message="error"><template #operation><t-button variant="text" @click="load">重试</t-button></template></FeedbackAlert>
    <t-loading :loading="loading" size="large">
      <div v-if="user" class="account-sections">
        <ResourceCard>
          <ResourceSection title="账号资料">
            <div class="identity-summary">
              <SystemAvatar :avatar-id="editing ? avatarId : user.avatar_id || 'sky'" :size="48" />
              <div class="identity-name"><strong>{{ user.nickname || user.username }}</strong><span>用户 UID {{ user.id }}</span></div>
              <t-button v-if="!editing" variant="text" theme="primary" class="edit-action" @click="startEdit">编辑</t-button>
            </div>
            <div v-if="editing" class="profile-editor">
              <t-form label-align="top" @submit.prevent="saveProfile">
                <t-form-item label="昵称" name="nickname"><t-input v-model="nickname" aria-label="昵称" maxlength="64" :disabled="saving" placeholder="请输入昵称" /></t-form-item>
              </t-form>
              <t-button variant="outline" :disabled="saving" @click="avatarVisible = true">更换头像</t-button>
              <FeedbackAlert v-if="profileError" theme="error" :message="profileError" />
              <div class="profile-actions"><t-button theme="primary" :loading="saving" :disabled="!dirty" @click="saveProfile">保存</t-button><t-button variant="outline" :disabled="saving" @click="cancelEdit">取消</t-button></div>
            </div>
            <dl class="profile-details">
              <div><dt>用户名</dt><dd>{{ user.username }}</dd></div>
              <div><dt>角色</dt><dd>{{ user.role === 'operator' ? '普通运营' : user.role === 'admin' ? '管理员' : '高级运营' }}</dd></div>
              <div><dt>所属团队</dt><dd>{{ user.team_name || '—' }}</dd></div>
              <div><dt>类目范围</dt><dd>{{ categoryScopeLabel }}</dd></div>
            </dl>
          </ResourceSection>
        </ResourceCard>
        <ResourceCard v-if="localAgentAllowed">
          <ResourceSection title="比特浏览器">
            <template #title-extra><t-tag class="binding-tag" :theme="binding?.bound ? 'success' : 'default'" variant="light">{{ bindingError ? '读取失败' : binding?.bound ? '已绑定' : '未绑定' }}</t-tag></template>
            <FeedbackAlert v-if="bindingError" theme="error" :message="bindingError"><template #operation><t-button variant="text" @click="load">重试</t-button></template></FeedbackAlert>
            <template v-else>
              <div class="device-hero"><span class="device-icon"><t-icon name="user-circle" /></span><div><strong>{{ bindingTitle }}</strong><span>{{ bindingSummary }}</span></div></div>
              <dl class="device-details">
                <div><dt>绑定比特账号</dt><dd>{{ binding?.main_user_id_masked || '—' }}</dd></div>
                <div><dt>绑定时间</dt><dd>{{ binding?.bound_at ? new Date(binding.bound_at).toLocaleString() : '—' }}</dd></div>
                <div><dt>最近验证</dt><dd>{{ binding?.last_verified_at ? new Date(binding.last_verified_at).toLocaleString() : '—' }}</dd></div>
                <div v-if="inDesktop && bitAccountDiffers"><dt>当前比特账号</dt><dd>{{ currentAccountText }}</dd></div>
              </dl>
              <FeedbackAlert v-if="bitAccountDiffers" theme="warning" message="当前比特浏览器登录账号与绑定账号不一致，请核对后换绑或切回原账号。" />
              <div v-if="inDesktop" class="device-actions">
                <t-button v-if="!binding?.bound" theme="primary" :loading="bindingBusy" :disabled="editing" @click="bindAccount()">绑定比特浏览器账号</t-button>
                <t-button v-if="binding?.bound && bitAccountDiffers" variant="outline" :loading="bindingBusy" :disabled="editing" @click="confirmRebind = true">换绑为当前账号</t-button>
              </div>
              <p v-else class="section-note">绑定和换绑请在桌面应用中操作。</p>
            </template>
          </ResourceSection>
          <ResourceSection v-if="inDesktop" title="本机" variant="form" class="machine-section">
            <dl class="device-details">
              <div><dt>设备 ID</dt><dd>{{ deviceId || '无法读取' }}</dd></div>
              <div><dt>系统</dt><dd>{{ systemLabel }}</dd></div>
              <div><dt>Agent 状态</dt><dd>{{ agentStatusText }}</dd></div>
              <div><dt>Agent 版本</dt><dd>{{ localAgent?.agent_version || '未读取到' }}</dd></div>
            </dl>
          </ResourceSection>
        </ResourceCard>
      </div>
    </t-loading>
    <ResourceOverlay v-model:visible="avatarVisible" title="选择头像" :busy="saving">
      <div class="avatar-options" role="group" aria-label="系统头像">
        <button v-for="option in avatarOptions" :key="option.id" type="button" class="avatar-choice" :class="{ selected: avatarId === option.id }" :aria-pressed="avatarId === option.id" @click="avatarId = option.id">
          <SystemAvatar :avatar-id="option.id" :size="42" /><span>{{ option.label }}</span>
        </button>
      </div>
      <template #footer><t-button theme="primary" @click="avatarVisible = false">完成</t-button></template>
    </ResourceOverlay>
    <ResourceOverlay v-model:visible="confirmRebind" title="换绑为当前比特浏览器账号" :busy="bindingBusy">
      <p>确认将当前比特账号 {{ localAccountMasked || '未读取到' }} 设为绑定账号？</p>
      <ul class="dialog-list"><li>只有新账号下的窗口能执行任务。</li><li>原账号窗口记录保留，切回原账号即可恢复。</li><li>正在执行的任务不受影响。</li></ul>
      <template #footer><t-button variant="outline" :disabled="bindingBusy" @click="confirmRebind = false">取消</t-button><t-button theme="primary" :loading="bindingBusy" @click="bindAccount({ rebind: true })">确认换绑</t-button></template>
    </ResourceOverlay>
  </main>
</template>

<style scoped>
.personal-page { min-width: 0; color: var(--wt-text-secondary); }
.account-sections { display: grid; gap: 16px; }
.identity-summary { display:flex; align-items:center; gap:16px; margin-bottom:24px; }
.identity-name { display:grid; gap:4px; }.identity-name strong { font-size:16px; font-weight:500; color:var(--wt-text-primary); }.identity-name span,.section-note { color:var(--wt-text-tertiary); font-size:13px; line-height:22px; }
.edit-action,.binding-tag { margin-left:auto; flex-shrink:0; white-space:nowrap; }
.profile-details { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); column-gap:24px; row-gap:16px; margin:0; }
.profile-details > div,.device-details > div { display:grid; grid-template-columns:112px minmax(0,1fr); gap:16px; align-items:start; }
dt { color:var(--wt-text-tertiary); }dd { margin:0; color:var(--wt-text-secondary); overflow-wrap:anywhere; line-height:24px; }
.profile-editor { display:grid; gap:16px; max-width:480px; margin-bottom:24px; }.profile-editor > .t-button { justify-self:start; }.profile-actions,.device-actions { display:flex; gap:8px; flex-wrap:wrap; }.device-actions:empty { display:none; }
.section-note { margin:0 0 16px; }.device-hero { display:flex; align-items:center; gap:16px; padding:20px 24px; background:var(--wt-primary-light,#eaf2ff); border-radius:6px; margin-bottom:16px; }.device-icon { display:grid; place-items:center; flex:none; width:48px; height:48px; border-radius:6px; background:#dceaff; color:var(--wt-primary); font-size:24px; }.device-hero > div { display:grid; gap:6px; }.device-hero strong { font-weight:500; color:var(--wt-text-primary); }.device-hero span { color:var(--wt-text-tertiary); }
.device-details { margin:0; }.device-details > div { padding:8px 0; }.device-actions { margin-top:16px; }.machine-section { margin-top:24px !important; padding-top:24px; border-top:1px solid var(--wt-border); }
.avatar-options { display:flex; flex-wrap:wrap; gap:12px; }.avatar-choice { display:grid; justify-items:center; gap:8px; padding:12px; background:var(--wt-bg-page); border:1px solid transparent; border-radius:6px; color:var(--wt-text-secondary); cursor:pointer; }.avatar-choice.selected { border-color:var(--wt-primary); background:var(--wt-primary-light,#eaf2ff); color:var(--wt-primary); }.avatar-choice:focus-visible { outline:2px solid var(--wt-primary); }.dialog-list { padding-left:20px; }
@media (max-width:900px) { .profile-details { grid-template-columns:1fr; } }
</style>
