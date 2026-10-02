<script setup>
import { computed, onMounted, ref } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { createSessionClient } from '../../../shared/api/session.js'
import { createDeviceBindingClient } from '../../../shared/api/deviceBinding.js'
import { createProfileBindingClient } from '../../../shared/api/profileBindings.js'
import { bitAccountDiffers as computeBitAccountDiffers, maskBitAccountId } from '../../../apps/desktop/features/local-agent/work-env-status.js'
import { avatarOptions } from '../../../shared/ui/systemAvatars.js'
import SystemAvatar from '../../../shared/ui/SystemAvatar.vue'

const session = createSessionClient()
const devices = createDeviceBindingClient()
const user = ref(null)
const nickname = ref('')
const avatarId = ref('sky')
const binding = ref(null)
const localDevice = ref(null)
const localAgent = ref(null)
const loading = ref(true)
const saving = ref(false)
const bindingBusy = ref(false)
const unbinding = ref(false)
const confirmUnbind = ref(false)
const understood = ref(false)
// 比特主账号切换确认（CHG-20261002-074 阶段 3）：在比特浏览器客户端换账号后，
// 系统绑定还停在旧账号，这里让站在这台机器前的用户自助确认切换，替代管理员入口。
const confirmTakeEnv = ref(false)
const bitAccountBusy = ref(false)
const error = ref('')
const message = ref('')
const inDesktop = typeof window !== 'undefined' && window.__TAURI_INTERNALS__ !== undefined
const deviceMatches = computed(() => !!binding.value?.bound && !!localDevice.value?.device_id && binding.value.device_id === localDevice.value.device_id)
const deviceConflicts = computed(() => !!binding.value?.bound && !!localDevice.value?.device_id && !deviceMatches.value)
// 本机 Agent 就绪 = 空闲或运行中（Agent 无事可做时报 idle，不是不可用）。
const localAgentReady = computed(() => ['idle', 'running'].includes(localAgent.value?.status))
// 本机账号与绑定账号一致时没有可绑定的东西，入口不出现；不一致才是
// 「客户端换过账号、系统待确认」。判定与顶栏胶囊同源（work-env-status），
// 两处只会同时变绿或同时变橙。
const bitAccountDiffers = computed(() => computeBitAccountDiffers(binding.value, localAgent.value?.main_user_id))
const bitAccountDisplay = computed(() => maskBitAccountId(localAgent.value?.main_user_id))
const deviceLabel = computed(() => binding.value?.bound ? binding.value.device_name || '已绑定的运营电脑' : '尚未绑定运营电脑')
const maskedDeviceId = computed(() => {
  const value = binding.value?.device_id || ''
  return value ? `${value.slice(0, 8)}…${value.slice(-6)}` : '—'
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [currentUser, currentBinding] = await Promise.all([session.me(), devices.get()])
    user.value = currentUser
    nickname.value = currentUser.nickname || currentUser.username
    avatarId.value = currentUser.avatar_id || 'sky'
    binding.value = currentBinding
    if (inDesktop) {
      try { localDevice.value = await invoke('local_device_identity') }
      catch (e) { error.value = String(e) }
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
  error.value = ''; message.value = ''
  try {
    const updated = await session.updateProfile(nickname.value, avatarId.value)
    user.value = updated
    nickname.value = updated.nickname
    window.dispatchEvent(new Event('wt-media:profile-updated'))
    message.value = '个人信息已保存'
  } catch (e) { error.value = e?.message || '保存失败' }
  finally { saving.value = false }
}

async function bindThisDevice() {
  if (!inDesktop || bindingBusy.value) return
  bindingBusy.value = true
  error.value = ''; message.value = ''
  try {
    const { bindTrustedLocalAgent } = await import('../../../apps/desktop/features/local-agent/init.js')
    await bindTrustedLocalAgent({ bindDevice: true })
    const verified = await devices.get()
    if (!verified.bound || verified.device_id !== localDevice.value?.device_id) throw new Error('云端设备绑定尚未确认，请重新检测')
    await load()
    message.value = '这台电脑已绑定，浏览器身份与本机环境已同步'
  } catch (e) { error.value = e?.message || String(e) }
  finally { bindingBusy.value = false }
}

async function unbindDevice() {
  if (!understood.value || unbinding.value) return
  unbinding.value = true
  error.value = ''; message.value = ''
  try {
    await devices.unbind()
    const verified = await devices.get()
    if (verified.bound) throw new Error('云端仍显示设备已绑定，请重新检测')
    confirmUnbind.value = false
    understood.value = false
    await load()
    message.value = '设备已解绑。旧电脑本地文件仍保留；新电脑现在可以手动绑定。'
  } catch (e) { error.value = e?.message || '解绑失败' }
  finally { unbinding.value = false }
}

// 「比特账号绑定」的确认步：把当前比特浏览器登录的账号设为绑定账号（overwrite=true），
// 覆盖系统原绑定 —— 这正是契约里 23002「身份确认」门允许显式越过的那个口子。当前环境的
// 账号从本机 Agent 状态读（上面 `localAgent` 已有），不靠用户手输，也不把绑定账号全文暴露给前端。
// 覆盖后系统只认新账号名下的窗口（profileguard 执行门按 main_user_id 比对），
// 原账号的窗口要重新扫描确认才能用——这句后果必须说在弹窗里，不能只做静默切换。
async function takeCurrentEnvironmentAsAuthoritative() {
  if (!inDesktop || bitAccountBusy.value) return
  const mainUserId = localAgent.value?.main_user_id
  if (!mainUserId) {
    error.value = '未读取到当前比特浏览器账号，请确认比特浏览器已登录后重试'
    return
  }
  bitAccountBusy.value = true
  error.value = ''; message.value = ''
  try {
    await createProfileBindingClient().confirmMainIdentityDirect(mainUserId, { overwrite: true })
    confirmTakeEnv.value = false
    await load()
    message.value = '已把当前比特浏览器账号设为绑定账号；原账号的窗口需重新扫描确认后才能使用。'
  } catch (e) { error.value = e?.message || String(e) }
  finally { bitAccountBusy.value = false }
}
</script>

<template>
  <main class="personal-page">
    <header class="page-heading"><div><p class="eyebrow">ACCOUNT &amp; DEVICE</p><h1>个人信息</h1><p>管理你的展示资料与运营电脑绑定。</p></div></header>
    <t-loading :loading="loading" size="large">
      <div class="personal-grid">
        <section class="profile-card">
          <h2>基本信息</h2>
          <div class="identity-summary"><SystemAvatar :avatar-id="avatarId" :size="64" /><div><strong>{{ user?.nickname || user?.username }}</strong><span>用户 UID {{ user?.id || '—' }}</span></div></div>
          <div class="static-details"><span>用户名</span><strong>{{ user?.username || '—' }}</strong><span>角色</span><strong>{{ user?.role === 'operator' ? '普通运营' : user?.role === 'admin' ? '管理员' : '高级运营' }}</strong></div>
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
          <div class="section-title"><div><h2>设备与本机环境</h2><p>每个账号只能绑定一台运营电脑。</p></div><span class="status-pill" :class="binding?.bound ? 'is-bound' : 'is-unbound'">{{ binding?.bound ? '已绑定' : '未绑定' }}</span></div>
          <div class="device-hero"><span class="device-icon"><t-icon name="desktop" /></span><div><strong>{{ deviceLabel }}</strong><span>{{ deviceMatches ? '当前正在使用这台电脑' : deviceConflicts ? '当前电脑与绑定设备不一致' : inDesktop && !binding?.bound ? '可以在这台电脑上手动绑定' : '设备信息由云端保存' }}</span></div></div>
          <dl class="device-details">
            <div><dt>设备标识</dt><dd>{{ maskedDeviceId }}</dd></div>
            <div><dt>绑定时间</dt><dd>{{ binding?.bound_at ? new Date(binding.bound_at).toLocaleString() : '—' }}</dd></div>
            <div><dt>最近验证</dt><dd>{{ binding?.last_verified_at ? new Date(binding.last_verified_at).toLocaleString() : '—' }}</dd></div>
            <div><dt>比特浏览器账号</dt><dd>{{ binding?.bit_account_bound ? binding.bit_main_user_id_masked || '已绑定' : '尚未确认' }}</dd></div>
            <div v-if="inDesktop"><dt>Local Agent</dt><dd>{{ localAgentReady ? '可用' : '当前不可用' }}</dd></div>
            <div v-if="inDesktop"><dt>当前比特浏览器</dt><dd>{{ localAgent?.bitbrowser_status === 'normal' ? '已连接' : '待检查' }}</dd></div>
          </dl>
          <div v-if="deviceConflicts" class="device-alert">当前电脑与已绑定设备不同。请先确认旧电脑上的本地文件位置，再手动解除旧设备绑定。</div>
          <div v-else-if="binding?.bound && inDesktop && !deviceMatches" class="device-alert">无法确认当前设备身份，请检查本机应用数据。</div>
          <p v-if="!binding?.bound" class="device-note">首次绑定会读取当前比特浏览器主账号并同步本机环境；后续在同一台电脑重新登录无需重复绑定。</p>
          <div class="device-actions">
            <t-button v-if="!binding?.bound && inDesktop" theme="primary" :loading="bindingBusy" @click="bindThisDevice">绑定这台电脑</t-button>
            <t-button v-if="binding?.bound" theme="danger" variant="outline" @click="confirmUnbind = true">解除设备绑定</t-button>
            <!-- 比特主账号切换后的自助绑定：只在当前环境账号与已绑定账号**不一致**时才给。
                 一致时没有可绑定的东西，给了也只是让用户白点一次重复确认；
                 账号读不到同样不给（那表示浏览器未登录）。 -->
            <t-button v-if="inDesktop && bitAccountDiffers" variant="outline" @click="confirmTakeEnv = true">比特账号绑定</t-button>
          </div>
          <p v-if="!inDesktop" class="device-note">Cloud 网页可查看及解除绑定；绑定新设备需在 Desktop 应用中操作。</p>
        </section>
      </div>
      <t-alert v-if="error" class="feedback" :message="error" theme="error" />
      <t-alert v-if="message" class="feedback" :message="message" theme="success" />
    </t-loading>

    <t-dialog v-model:visible="confirmUnbind" header="确认解除设备绑定" :footer="false" :close-on-overlay-click="false">
      <div class="unbind-warning"><strong>换设备前请确认本地文件</strong><p>解除绑定后，旧电脑上的本地文件和本地索引仍保留在旧电脑；新电脑无法直接访问，也不会自动迁移。云端素材、任务和历史记录不会删除。旧设备将不能领取新任务。</p></div>
      <label class="acknowledge"><input v-model="understood" type="checkbox" />我已了解旧文件保留在旧电脑，新设备无法直接访问</label>
      <div class="dialog-actions"><t-button variant="outline" @click="confirmUnbind = false">取消</t-button><t-button theme="danger" :disabled="!understood" :loading="unbinding" @click="unbindDevice">确认解除绑定</t-button></div>
    </t-dialog>

    <t-dialog v-model:visible="confirmTakeEnv" header="比特账号绑定" :footer="false" :close-on-overlay-click="false">
      <p class="dialog-note">把当前比特浏览器登录的账号（{{ bitAccountDisplay }}）设为绑定账号后：</p>
      <ul class="dialog-list">
        <li>这台电脑的比特浏览器窗口按新账号归属，只有新账号下的窗口能执行任务；</li>
        <li>原账号下的窗口需要在新账号下重新扫描确认后才能继续使用；</li>
        <li>原账号的窗口记录不会删除，切回原账号即可恢复。</li>
      </ul>
      <p class="dialog-note">请确认这是你当前使用的机器与账号。</p>
      <div class="dialog-actions"><t-button variant="outline" @click="confirmTakeEnv = false">取消</t-button><t-button theme="primary" :loading="bitAccountBusy" @click="takeCurrentEnvironmentAsAuthoritative">确认绑定当前账号</t-button></div>
    </t-dialog>
  </main>
</template>

<style scoped>
.personal-page { max-width: 1140px; margin: 0 auto; color: #193250; }.page-heading { margin-bottom: 25px; }.eyebrow { margin: 0 0 7px; color: #3b86e9; font-size: 11px; font-weight: 700; letter-spacing: .16em; }.page-heading h1 { margin: 0 0 6px; font-size: 28px; }.page-heading p { margin: 0; color: #7b8ba2; }
.personal-grid { display: grid; grid-template-columns: minmax(330px, 1fr) minmax(340px, 1.1fr); gap: 20px; }.profile-card,.device-card { padding: 28px; border: 1px solid #e7edf5; border-radius: 14px; background: #fff; box-shadow: 0 8px 24px #1733570a; }.profile-card h2,.device-card h2 { margin: 0; font-size: 20px; }.identity-summary { display: flex; align-items: center; gap: 14px; margin: 24px 0; }.identity-summary div { display: flex; flex-direction: column; gap: 5px; }.identity-summary strong { font-size: 19px; }.identity-summary span { color: #8294ab; font-size: 13px; }.static-details { display: grid; grid-template-columns: 75px 1fr; gap: 10px; padding: 16px 0; border-top: 1px solid #eef2f6; border-bottom: 1px solid #eef2f6; }.static-details span { color: #8a9aaf; }.static-details strong { font-weight: 500; }.form-label { display: block; margin: 22px 0 9px; font-weight: 600; }.field-help { margin: 6px 0 0; color: #94a3b8; font-size: 12px; }.avatar-options { display: flex; flex-wrap: wrap; gap: 9px; margin-bottom: 20px; }.avatar-choice { display: flex; flex-direction: column; align-items: center; gap: 5px; min-width: 66px; padding: 8px 5px; border: 2px solid transparent; border-radius: 10px; background: #f7f9fc; cursor: pointer; color: #637995; font-size: 12px; }.avatar-choice.selected { border-color: #2984fa; background: #eef6ff; color: #1465d2; }
.section-title { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }.section-title p { margin: 7px 0 0; color: #8d9db0; font-size: 13px; }.status-pill { padding: 5px 10px; border-radius: 20px; font-size: 12px; }.is-bound { color: #167e5c; background: #e7f7f0; }.is-unbound { color: #8f6f1c; background: #fff5da; }.device-hero { display: flex; gap: 14px; align-items: center; margin: 24px 0; padding: 18px; border-radius: 10px; background: #f3f8ff; }.device-icon { display: grid; place-items: center; width: 48px; height: 48px; border-radius: 10px; background: #deedff; color: #2075e8; font-size: 24px; }.device-hero div { display: flex; flex-direction: column; gap: 5px; }.device-hero span:not(.device-icon) { color: #7890ae; font-size: 13px; }.device-details { margin: 0; }.device-details div { display: flex; justify-content: space-between; gap: 14px; padding: 12px 0; border-bottom: 1px solid #eef2f6; }.device-details dt { color: #8a9bb0; }.device-details dd { margin: 0; text-align: right; overflow-wrap: anywhere; }.device-alert { margin-top: 19px; padding: 12px; border: 1px solid #f7cdab; border-radius: 8px; color: #99550d; background: #fff8ed; }.device-note { margin: 18px 0 0; color: #8294a9; font-size: 13px; line-height: 1.6; }.device-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 20px; }.feedback { margin-top: 18px; }.unbind-warning { padding: 15px; border-radius: 9px; color: #81421a; background: #fff3e8; line-height: 1.7; }.unbind-warning p { margin: 7px 0 0; }.dialog-note { margin: 0; color: #7b8ba2; line-height: 1.7; }.dialog-list { margin: 10px 0; padding-left: 20px; color: #5b6d85; line-height: 1.9; }.dialog-list li + li { margin-top: 2px; }.acknowledge { display: flex; align-items: flex-start; gap: 8px; margin: 19px 0; cursor: pointer; line-height: 1.5; }.acknowledge input { margin-top: 4px; }.dialog-actions { display: flex; justify-content: flex-end; gap: 9px; }
@media (max-width: 830px) { .personal-grid { grid-template-columns: 1fr; } }
@media (max-width: 480px) { .profile-card,.device-card { padding: 20px; } }
</style>
