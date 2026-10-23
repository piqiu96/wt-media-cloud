<script setup>
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { feedback } from '../../../shared/ui/feedback/index.js'
import { computed, onMounted, ref } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { createSessionClient } from '../../../shared/api/session.js'
import { createBitAccountClient } from '../../../shared/api/bitAccount.js'
import { createUsersClient } from '../../../apps/cloud/pages/users/usersApi.js'
import { bitAccountDiffers as computeBitAccountDiffers } from '../../../apps/desktop/features/local-agent/work-env-status.js'
import { avatarOptions } from '../../../shared/ui/systemAvatars.js'
import SystemAvatar from '../../../shared/ui/SystemAvatar.vue'

const session = createSessionClient()
const bitAccounts = createBitAccountClient()
// 分组/游戏范围只做展示，取与素材库页同一个 games 端点把游戏 ID 映射成名称；
// 拉不到时退化为显示 ID，不阻塞整页加载。
const usersClient = createUsersClient()
const games = ref([])
const user = ref(null)
const nickname = ref('')
const avatarId = ref('sky')
const binding = ref(null)
const localAgent = ref(null)
const loading = ref(true)
const saving = ref(false)
const bindingBusy = ref(false)
// 本机比特账号变化后，用户必须主动确认换用当前账号。
const confirmRebind = ref(false)
const error = ref('')
const inDesktop = typeof window !== 'undefined' && window.__TAURI_INTERNALS__ !== undefined
// 管理员不受分组/游戏限制；运营展示自己被授权的游戏名称（找不到名称时退化为 ID）。
const gameScopeLabel = computed(() => {
  if (user.value?.role === 'admin') return '全部游戏'
  const ids = user.value?.game_ids || []
  if (!ids.length) return '—'
  return ids.map((id) => games.value.find((game) => game.id === id)?.name || id).join('、')
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
const bindingTitle = computed(() => (binding.value?.bound ? `已绑定 ${binding.value.main_user_id_masked || '比特浏览器账号'}` : '尚未绑定比特浏览器账号'))
const bindingSummary = computed(() => {
  if (!binding.value?.bound) return inDesktop ? '由本机比特浏览器读取后绑定' : '绑定需在 Desktop 应用中操作'
  if (bitAccountDiffers.value) return '当前比特浏览器登录的是另一个账号'
  if (localAccountMasked.value) return '当前比特浏览器已登录绑定账号'
  return '尚未读取到本机比特浏览器账号'
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [currentUser, currentBinding, gameRows] = await Promise.all([session.me(), bitAccounts.get(), usersClient.listGames({ status: 'enabled' }).catch(() => [])])
    user.value = currentUser
    nickname.value = currentUser.nickname || currentUser.username
    avatarId.value = currentUser.avatar_id || 'sky'
    games.value = Array.isArray(gameRows) ? gameRows : []
    binding.value = currentBinding
    if (inDesktop) {
      try { localAgent.value = await invoke('local_agent_status') }
      catch { localAgent.value = null }
    }
  } catch (e) { error.value = e?.message || '无法加载个人信息' }
  finally { loading.value = false }
}
onMounted(load)

async function saveProfile() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    const updated = await session.updateProfile(nickname.value, avatarId.value)
    user.value = updated
    nickname.value = updated.nickname
    window.dispatchEvent(new Event('wt-media:profile-updated'))
    feedback.toast.success('个人信息已保存')
  } catch (e) { feedback.toast.error(e?.message || '保存失败') }
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
    <header class="page-heading"><div><p class="eyebrow">ACCOUNT &amp; BITBROWSER</p><h1>个人信息</h1><p>管理你的展示资料与比特浏览器账号绑定。</p></div></header>
    <t-loading :loading="loading" size="large">
      <div class="personal-grid">
        <section class="profile-card">
          <h2>基本信息</h2>
          <div class="identity-summary"><SystemAvatar :avatar-id="avatarId" :size="64" /><div><strong>{{ user?.nickname || user?.username }}</strong><span>用户 UID {{ user?.id || '—' }}</span></div></div>
          <div class="static-details"><span>用户名</span><strong>{{ user?.username || '—' }}</strong><span>角色</span><strong>{{ user?.role === 'operator' ? '普通运营' : user?.role === 'admin' ? '管理员' : '高级运营' }}</strong><span>所属分组</span><strong>{{ user?.team_name || '—' }}</strong><span>游戏范围</span><strong>{{ gameScopeLabel }}</strong></div>
          <label class="form-label" for="profile-nickname">昵称</label>
          <t-input id="profile-nickname" v-model="nickname" maxlength="64" placeholder="请输入昵称" />
          <p class="field-help">默认与用户名一致，可随时修改。</p>
          <p class="form-label">选择系统头像</p>
          <div class="avatar-options" role="group" aria-label="系统头像">
            <button v-for="option in avatarOptions" :key="option.id" type="button" class="avatar-choice" :class="{ selected: avatarId === option.id }" :aria-pressed="avatarId === option.id" @click="avatarId = option.id">
              <SystemAvatar :avatar-id="option.id" :size="42" /><span>{{ option.label }}</span>
            </button>
          </div>
          <t-button theme="primary" :loading="saving" @click="saveProfile">保存</t-button>
        </section>

        <section class="device-card">
          <div class="section-title"><div><h2>比特浏览器</h2><p>每个账号绑定一个比特浏览器主账号，只有该账号下的窗口能执行任务。</p></div><span class="status-pill" :class="binding?.bound ? 'is-bound' : 'is-unbound'">{{ binding?.bound ? '已绑定' : '未绑定' }}</span></div>
          <div class="device-hero"><span class="device-icon"><t-icon name="user-circle" /></span><div><strong>{{ bindingTitle }}</strong><span>{{ bindingSummary }}</span></div></div>
          <dl class="device-details">
            <div><dt>绑定账号</dt><dd>{{ binding?.main_user_id_masked || '—' }}</dd></div>
            <div><dt>绑定时间</dt><dd>{{ binding?.bound_at ? new Date(binding.bound_at).toLocaleString() : '—' }}</dd></div>
            <div><dt>最近验证</dt><dd>{{ binding?.last_verified_at ? new Date(binding.last_verified_at).toLocaleString() : '—' }}</dd></div>
            <div v-if="inDesktop"><dt>当前比特浏览器</dt><dd>{{ currentAccountText }}</dd></div>
          </dl>
          <FeedbackAlert v-if="bitAccountDiffers" theme="warning" class="device-alert" message="当前比特浏览器登录账号与系统绑定账号不一致，任务会被拦下。如确需更换，请点击「换绑为当前账号」。" />
          <p v-if="!binding?.bound" class="device-note">首次绑定由本机 Agent 直接读取比特浏览器当前登录的主账号，不需要手工输入；其他账号下的窗口不会出现在可用列表中。</p>
          <div class="device-actions">
            <t-button v-if="!binding?.bound && inDesktop" theme="primary" :loading="bindingBusy" @click="bindAccount()">绑定比特浏览器账号</t-button>
            <t-button v-if="binding?.bound && inDesktop && bitAccountDiffers" variant="outline" :loading="bindingBusy" @click="confirmRebind = true">换绑为当前账号</t-button>
          </div>
          <p v-if="!inDesktop" class="device-note">Cloud 网页可查看绑定状态；绑定与换绑需在 Desktop 应用中操作。</p>
        </section>
      </div>
      <FeedbackAlert v-if="error" class="feedback" :message="error" theme="error" />
    </t-loading>

    <t-dialog v-model:visible="confirmRebind" header="换绑为当前比特浏览器账号" :footer="false" :close-on-overlay-click="false">
      <p class="dialog-note">把当前比特浏览器登录的账号（{{ localAccountMasked || '未读取到' }}）设为绑定账号后：</p>
      <ul class="dialog-list">
        <li>这台电脑的比特浏览器窗口按新账号归属，只有新账号下的窗口能执行任务；</li>
        <li>原账号下的窗口记录不会删除，切回原账号即可恢复；</li>
        <li>正在执行的任务不受影响。</li>
      </ul>
      <p class="dialog-note">请确认比特浏览器已登录你要使用的账号。</p>
      <div class="dialog-actions"><t-button variant="outline" @click="confirmRebind = false">取消</t-button><t-button theme="primary" :loading="bindingBusy" @click="bindAccount({ rebind: true })">确认换绑</t-button></div>
    </t-dialog>
  </main>
</template>

<style scoped>
.personal-page { max-width: 1140px; margin: 0 auto; color: #193250; }.page-heading { margin-bottom: 25px; }.eyebrow { margin: 0 0 7px; color: #3b86e9; font-size: 11px; font-weight: 700; letter-spacing: .16em; }.page-heading h1 { margin: 0 0 6px; font-size: 28px; }.page-heading p { margin: 0; color: #7b8ba2; }
.personal-grid { display: grid; grid-template-columns: minmax(330px, 1fr) minmax(340px, 1.1fr); gap: 20px; }.profile-card,.device-card { padding: 28px; border: 1px solid #e7edf5; border-radius: 14px; background: #fff; box-shadow: 0 8px 24px #1733570a; }.profile-card h2,.device-card h2 { margin: 0; font-size: 20px; }.identity-summary { display: flex; align-items: center; gap: 14px; margin: 24px 0; }.identity-summary div { display: flex; flex-direction: column; gap: 5px; }.identity-summary strong { font-size: 19px; }.identity-summary span { color: #8294ab; font-size: 13px; }.static-details { display: grid; grid-template-columns: 75px 1fr; gap: 10px; padding: 16px 0; border-top: 1px solid #eef2f6; border-bottom: 1px solid #eef2f6; }.static-details span { color: #8a9aaf; }.static-details strong { font-weight: 500; }.form-label { display: block; margin: 22px 0 9px; font-weight: 600; }.field-help { margin: 6px 0 0; color: #94a3b8; font-size: 12px; }.avatar-options { display: flex; flex-wrap: wrap; gap: 9px; margin-bottom: 20px; }.avatar-choice { display: flex; flex-direction: column; align-items: center; gap: 5px; min-width: 66px; padding: 8px 5px; border: 2px solid transparent; border-radius: 10px; background: #f7f9fc; cursor: pointer; color: #637995; font-size: 12px; }.avatar-choice.selected { border-color: #2984fa; background: #eef6ff; color: #1465d2; }
.section-title { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }.section-title p { margin: 7px 0 0; color: #8d9db0; font-size: 13px; }.status-pill { padding: 5px 10px; border-radius: 20px; font-size: 12px; }.is-bound { color: #167e5c; background: #e7f7f0; }.is-unbound { color: #8f6f1c; background: #fff5da; }.device-hero { display: flex; gap: 14px; align-items: center; margin: 24px 0; padding: 18px; border-radius: 10px; background: #f3f8ff; }.device-icon { display: grid; place-items: center; width: 48px; height: 48px; border-radius: 10px; background: #deedff; color: #2075e8; font-size: 24px; }.device-hero div { display: flex; flex-direction: column; gap: 5px; }.device-hero span:not(.device-icon) { color: #7890ae; font-size: 13px; }.device-details { margin: 0; }.device-details div { display: flex; justify-content: space-between; gap: 14px; padding: 12px 0; border-bottom: 1px solid #eef2f6; }.device-details dt { color: #8a9bb0; }.device-details dd { margin: 0; text-align: right; overflow-wrap: anywhere; }.device-alert { margin-top: 19px; }.device-note { margin: 18px 0 0; color: #8294a9; font-size: 13px; line-height: 1.6; }.device-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 20px; }.feedback { margin-top: 18px; }.unbind-warning { padding: 15px; border-radius: 9px; color: #81421a; background: #fff3e8; line-height: 1.7; }.unbind-warning p { margin: 7px 0 0; }.dialog-note { margin: 0; color: #7b8ba2; line-height: 1.7; }.dialog-list { margin: 10px 0; padding-left: 20px; color: #5b6d85; line-height: 1.9; }.dialog-list li + li { margin-top: 2px; }.acknowledge { display: flex; align-items: flex-start; gap: 8px; margin: 19px 0; cursor: pointer; line-height: 1.5; }.acknowledge input { margin-top: 4px; }.dialog-actions { display: flex; justify-content: flex-end; gap: 9px; }
@media (max-width: 830px) { .personal-grid { grid-template-columns: 1fr; } }
@media (max-width: 480px) { .profile-card,.device-card { padding: 20px; } }
</style>
