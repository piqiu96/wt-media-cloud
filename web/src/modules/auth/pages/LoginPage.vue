<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createSessionClient } from '../../../shared/api/session.js'
import { canUseDesktop, isDesktop } from '../../../utils.js'
import { resolveAppVersion } from '../../../shared/utils/appVersion.js'
import BrandLogo from '../../../shared/ui/BrandLogo.vue'

const router = useRouter()
const sessionClient = createSessionClient()
const username = ref('')
const password = ref('')
const showPassword = ref(false)
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const replaceNeeded = ref(false)
const loginAction = ref('')
const version = ref('')

function resetReplacePrompt() {
  replaceNeeded.value = false
  if (!submitting.value) {
    loginAction.value = ''
  }
}

function submitLogin() {
  return login({ replaceExisting: false })
}

function submitReplacementLogin() {
  return login({ replaceExisting: true })
}

onMounted(async () => {
  version.value = await resolveAppVersion()
  if (isDesktop() && new URLSearchParams(window.location.search).has('desktop_role_forbidden')) {
    error.value = '当前角色不能登录 Desktop，请使用 Cloud Web 管理。'
    loading.value = false
    return
  }
  try {
    const user = await sessionClient.me()
    if (isDesktop() && !canUseDesktop(user)) {
      await sessionClient.logout().catch(() => {})
      error.value = '管理员和高级运营不能登录 Desktop，请使用 Cloud Web 管理。'
      return
    }
    router.push('/')
    if (isDesktop()) {
      const { ensureTrustedLocalAgent } = await import('../../../apps/desktop/features/local-agent/init.js')
      void ensureTrustedLocalAgent().catch((e) => console.info('本机执行状态待恢复', e?.message || e))
    }
  } catch {
    // Not logged in, show form
  } finally {
    loading.value = false
  }
})

async function login(options = {}) {
  if (submitting.value) return
  const replaceExisting = options?.replaceExisting === true
  submitting.value = true
  loginAction.value = replaceExisting ? '正在替换旧会话并登录…' : '正在登录…'
  error.value = ''
  if (!replaceExisting) {
    resetReplacePrompt()
  }
  try {
    const user = await sessionClient.login(username.value, password.value, {
      replaceExisting,
    })
    if (isDesktop() && !canUseDesktop(user)) {
      await sessionClient.logout().catch(() => {})
      error.value = '管理员和高级运营不能登录 Desktop，请使用 Cloud Web 管理。'
      return
    }
    router.push('/')
    if (isDesktop()) {
      const { ensureTrustedLocalAgent } = await import('../../../apps/desktop/features/local-agent/init.js')
      void ensureTrustedLocalAgent().catch((e) => console.info('本机执行状态待恢复', e?.message || e))
    }
  } catch (e) {
    if (e.errcode === 20010) {
      replaceNeeded.value = true
      error.value = '当前账号已有进行中的桌面端登录，再次登录将替换旧会话。'
      return
    }
    error.value = e.message
  } finally {
    submitting.value = false
    loginAction.value = ''
  }
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <main class="login-shell">
      <header class="login-topbar">
        <router-link v-if="!isDesktop()" to="/home" class="brand-link" aria-label="起飞官网"><BrandLogo /></router-link>
        <BrandLogo v-else />
        <span class="version-pill">v{{ version }}</span>
      </header>

      <section class="login-layout">
        <div class="login-story">
          <div class="story-copy">
            <h1>让流量<span>跑起来</span><i aria-hidden="true"></i></h1>
            <p class="story-description">找内容、管素材、发内容、看数据，让内容运营超轻松！</p>
          </div>

        </div>

        <section class="login-card" :class="{ 'has-feedback': error || loginAction || replaceNeeded, 'has-multiple-feedback': replaceNeeded }" aria-labelledby="login-heading">
          <h2 id="login-heading">欢迎登录</h2>
          <p class="login-intro">使用管理员分配的账号登录</p>
          <form class="login-form" @submit.prevent="replaceNeeded ? submitReplacementLogin() : submitLogin()">
            <label class="field-label" for="login-username">用户名</label>
            <div class="login-field"><t-icon name="user" /><input id="login-username" v-model="username" autocomplete="username" placeholder="用户名" @input="resetReplacePrompt" /></div>
            <label class="field-label" for="login-password">密码</label>
            <div class="login-field"><t-icon name="lock-on" /><input id="login-password" v-model="password" :type="showPassword ? 'text' : 'password'" autocomplete="current-password" placeholder="密码" @input="resetReplacePrompt" /><button type="button" class="password-toggle" :aria-label="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword"><t-icon :name="showPassword ? 'browse-off' : 'browse'" /></button></div>
            <t-alert v-if="error" :message="error" theme="error" />
            <t-alert v-if="replaceNeeded" message="替换的只是登录会话；任务执行由设备绑定决定，本次操作不改变设备绑定，已绑定电脑的执行不受影响。" theme="warning" />
            <t-alert v-if="loginAction" :message="loginAction" theme="info" />
            <button type="submit" class="login-button primary" :disabled="submitting">{{ submitting ? loginAction : replaceNeeded ? '替换旧会话并登录' : '登录' }}</button>
            <button v-if="replaceNeeded" type="button" class="login-button outline" :disabled="submitting" @click="resetReplacePrompt">取消</button>
          </form>
          <p class="login-help">账号由管理员创建与维护，如需开通请联系管理员。</p>
        </section>
      </section>

      <div class="login-wave wave-one" aria-hidden="true"></div>
      <div class="login-wave wave-two" aria-hidden="true"></div>
      <div class="login-wave wave-three" aria-hidden="true"></div>
    </main>
  </t-loading>
</template>

<style scoped>
/* 官网品牌色、渐变与卡片样式从共享 Token 读取。 */
.login-shell{position:relative;isolation:isolate;display:flex;flex-direction:column;min-height:100vh;overflow:hidden;background:var(--wt-brand-surface);color:var(--wt-brand-ink);font-family:"PingFang SC","Microsoft YaHei","Noto Sans SC",sans-serif}

.login-topbar,.login-layout{position:relative;z-index:2}
.login-topbar{display:flex;align-items:center;justify-content:space-between;padding:18px 2% 0}
.brand-link{text-decoration:none}
.login-topbar :deep(.brand-logo__mark){width:40px;height:40px}
.login-topbar :deep(.brand-logo__copy small){font-size:12px}
.version-pill{display:inline-flex;align-items:center;height:29px;padding:0 12px;border-radius:999px;background:#E8F3FF;color:var(--wt-brand-primary);font-size:13px;font-weight:600}

.login-layout{display:grid;grid-template-columns:minmax(0,1.45fr) minmax(400px,.95fr);align-items:center;gap:3%;flex:1;width:min(1800px,96%);margin:0 auto;padding:12px 0 72px}

/* 左侧品牌区 */
.login-story{position:relative;display:flex;flex-direction:column;min-width:0;padding-left:4%}
.story-copy{position:relative;z-index:2}
.story-copy h1{position:relative;width:max-content;max-width:100%;margin:0;color:var(--wt-brand-dark);font-size:clamp(52px,4.4vw,84px);font-weight:800;letter-spacing:-.02em;line-height:1.08;white-space:nowrap}
.story-copy h1 span{background:var(--wt-brand-gradient);-webkit-background-clip:text;background-clip:text;color:transparent}
.story-copy h1 i{position:absolute;bottom:-8px;right:3%;width:48%;height:18px;border-bottom:6px solid var(--wt-brand-underline);border-radius:50%;transform:rotate(-5deg)}
.story-description{margin:26px 0 0;color:var(--wt-ink-muted);font-size:clamp(16px,1.3vw,21px);font-weight:500;line-height:1.6}

/* 右侧登录卡 */
.login-card{box-sizing:border-box;justify-self:end;width:min(33vw,600px);max-width:100%;min-height:650px;padding:76px 48px 40px;border:var(--wt-card-border);border-radius:var(--wt-card-radius);background:var(--wt-card-bg);box-shadow:var(--wt-card-shadow);text-align:center}
.login-card h2{margin:0;color:var(--wt-brand-dark);font-size:38px;font-weight:700;letter-spacing:.02em}
.login-intro{margin:10px 0 64px;color:#7286A6;font-size:16px}
.login-form{display:flex;flex-direction:column;gap:28px;text-align:left}
.field-label{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}
/* 保持现有输入与密码显隐行为，只调整外框视觉。 */
.login-field{display:flex;align-items:center;gap:12px;height:64px;padding:0 18px;border:1px solid var(--wt-field-border);border-radius:12px;background:rgba(255,255,255,.86);color:var(--wt-field-icon);font-size:20px;transition:border-color .2s,box-shadow .2s}
.login-field:hover{border-color:#BBD3FB}
.login-field:focus-within{border-color:var(--td-brand-color,#0052D9);box-shadow:0 0 0 3px rgba(0,82,217,.07)}
.login-field input{min-width:0;width:100%;height:100%;border:0;outline:0;background:transparent;color:#183557;font-size:16px}
.login-field input::placeholder{color:var(--wt-field-placeholder)}
.password-toggle{display:grid;place-items:center;flex:none;width:32px;height:32px;padding:0;border:0;background:transparent;color:#7890B5;font-size:21px;cursor:pointer;transition:color .2s}
.password-toggle:hover{color:var(--wt-brand-primary)}

.login-button{position:relative;width:100%;height:76px;border:1px solid transparent;border-radius:12px;font-size:18px;font-weight:600;cursor:pointer;transition:filter .18s}
.login-button:disabled{cursor:not-allowed;opacity:.7}
.login-button.primary{margin-top:6px;border:0;background:var(--wt-brand-cta-gradient);color:#fff}
.login-button.primary:hover:not(:disabled){filter:brightness(.98)}
.login-button.primary:active:not(:disabled){filter:brightness(.94)}
.login-button.outline{height:46px;background:#fff;border-color:#C8DCF0;color:#34547C;font-size:15px;font-weight:500}
.login-help{margin:38px 0 0;padding-top:24px;border-top:1px solid var(--wt-divider);color:var(--wt-ink-soft);font-size:14px;line-height:1.7}

.login-wave{position:absolute;z-index:1;left:-15%;width:130%;border-radius:45% 55% 0 0 / 38% 45% 0 0;pointer-events:none}
.wave-one{bottom:-160px;height:330px;background:#8FC9FF;opacity:.32;transform:rotate(-3deg)}
.wave-two{bottom:-199px;height:330px;background:#FFFFFF;opacity:.45;transform:rotate(4deg)}
.wave-three{bottom:-236px;height:300px;background:#BFE3FF;opacity:.25;transform:rotate(-2deg)}

@media(max-width:1400px){.login-layout{grid-template-columns:minmax(0,1.5fr) minmax(400px,.95fr);gap:2.5%}}
@media(max-width:1180px){.login-layout{grid-template-columns:minmax(0,1.35fr) minmax(370px,.95fr);gap:2%}.login-story{padding-left:0}.login-card{max-width:470px;padding:44px 34px 32px}}
@media(max-width:760px){.login-topbar{padding:19px 20px 0}.login-layout{display:flex;flex-direction:column;gap:18px;width:calc(100% - 38px);padding:14px 0 80px}.login-story{width:100%;padding-left:0}.story-copy h1{font-size:clamp(40px,10vw,66px)}.story-description{margin-top:14px;font-size:15px}.login-card{width:100%;max-width:520px;min-height:auto;padding:34px 24px 26px}.login-card h2{font-size:29px}.login-intro{margin:6px 0 26px;font-size:15px}.login-form{gap:16px}.login-field{height:52px}.login-field input{font-size:16px}.login-button{height:54px;font-size:17px}.login-help{margin-top:26px;padding-top:20px}.login-wave{height:190px}.wave-one{bottom:-110px}.wave-two{bottom:-130px}.wave-three{bottom:-150px}}

/* 桌面插画独立于文字和表单，避免把视觉稿中的文字作为位图拉伸。 */
@media(min-width:761px){
  .login-shell{height:100vh;min-height:700px;background:#dceeff url('/login-visual-backdrop.png') center / 100% 100% no-repeat}
  .login-topbar{position:absolute;inset:0 0 auto;z-index:4}
  .login-topbar :deep(.brand-logo){gap:10px}
  .login-topbar :deep(.brand-logo__mark){width:50px;height:50px}
  .login-topbar :deep(.brand-logo__copy strong){font-size:31px;line-height:1}
  .login-topbar :deep(.brand-logo__copy small){margin-top:4px;font-size:13px;letter-spacing:.04em}
  .version-pill{position:absolute;top:26px;right:3.5%;height:31px;background:#e8f4ff}
  .login-layout{position:absolute;inset:0;display:block;width:100%;margin:0;padding:0}
  .login-story{position:absolute;left:6.95%;top:17.2%;width:54%;padding:0}
  .story-copy h1{font-size:clamp(42px,5.55vw,96px);line-height:1.18;letter-spacing:-.055em}
  .story-copy h1 i{bottom:-4px;right:1%;width:48%;border-bottom-width:6px}
  .story-description{margin-top:12px;font-size:clamp(12px,1.56vw,27px);line-height:1.45;white-space:nowrap}
  .login-wave{display:none}
  .login-card{position:absolute;left:63.58%;top:14.67%;width:32.96%;height:69.61%;min-height:0;max-width:none;padding:0;border:0;border-radius:26px;background:#fff;box-shadow:none;backdrop-filter:none}
  .login-card.has-feedback{height:calc(69.61% + 110px)}
  .login-card.has-multiple-feedback{height:calc(69.61% + 260px)}
  .login-shell:has(.login-card.has-feedback){overflow-y:auto}
  .login-card h2{position:absolute;top:13.1%;right:0;left:0;margin:0;font-size:clamp(30px,2.4vw,46px);line-height:1.3}
  .login-intro{position:absolute;top:23%;right:0;left:0;margin:0;font-size:clamp(15px,1.2vw,23px);line-height:1.5}
  .login-form{position:absolute;top:35.3%;right:9.5%;left:9.5%;display:flex;gap:2.45vh}
  .login-field{height:7.45vh;min-height:52px;padding:0 1.4vw;border-radius:12px;background:#fff;font-size:clamp(20px,1.3vw,24px)}
  .login-field input{font-size:clamp(16px,1.25vw,22px)}
  .login-button.primary{height:8.08vh;min-height:58px;margin-top:.4vh;font-size:clamp(18px,1.35vw,25px)}
  .login-help{position:absolute;top:81.8%;right:9.5%;left:9.5%;margin:0;padding-top:5.5%;font-size:clamp(12px,.95vw,16px)}
}
</style>
