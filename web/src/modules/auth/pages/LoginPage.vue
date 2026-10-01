<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createSessionClient } from '../../../shared/api/session.js'
import { canUseDesktop, isDesktop } from '../../../utils.js'
import BrandLogo from '../../../shared/ui/BrandLogo.vue'
import webPackage from '../../../../package.json'

const router = useRouter()
const sessionClient = createSessionClient()
const username = ref('')
const password = ref('')
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const replaceNeeded = ref(false)
const loginAction = ref('')
const version = ref(webPackage.version)

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
  if (isDesktop()) {
    try {
      const { getVersion } = await import('@tauri-apps/api/app')
      version.value = await getVersion()
    } catch { /* Browser preview uses the Web build version. */ }
  }
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
      error.value = '当前账号已在其他位置登录。请确认是否替换旧会话。'
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
        <BrandLogo />
        <span class="version-pill">v{{ version }}</span>
      </header>

      <section class="login-layout">
        <div class="login-story">
          <div class="story-copy">
            <h1>敢想，真做。</h1>
            <p class="story-subtitle">下一场浪，从一个想法开始。</p>
            <p class="story-description">找内容、管素材、做生产、发内容、看数据，让内容运营简单一点。</p>
          </div>
          <div class="story-features" aria-label="平台能力">
            <div v-for="feature in [
              { icon: 'search', title: '内容发现', note: '发现优质灵感' },
              { icon: 'folder', title: '素材管理', note: '沉淀内容资产' },
              { icon: 'send', title: '生产发布', note: '高效协同创作' },
              { icon: 'chart-bar', title: '数据分析', note: '用数据驱动增长' },
            ]" :key="feature.title" class="feature-item">
              <span class="feature-icon"><t-icon :name="feature.icon" /></span>
              <strong>{{ feature.title }}</strong><small>{{ feature.note }}</small>
            </div>
          </div>
        </div>

        <section class="login-card" aria-labelledby="login-heading">
          <BrandLogo class="card-brand" />
          <h2 id="login-heading">欢迎登录</h2>
          <p class="login-intro">使用管理员分配的账号登录</p>
          <form class="login-form" @submit.prevent="replaceNeeded ? submitReplacementLogin() : submitLogin()">
            <label class="field-label" for="login-username">用户名</label>
            <div class="login-field"><t-icon name="user" /><input id="login-username" v-model="username" autocomplete="username" placeholder="用户名" @input="resetReplacePrompt" /></div>
            <label class="field-label" for="login-password">密码</label>
            <div class="login-field"><t-icon name="lock-on" /><input id="login-password" v-model="password" type="password" autocomplete="current-password" placeholder="密码" @input="resetReplacePrompt" /></div>
            <t-alert v-if="error" :message="error" theme="error" />
            <t-alert v-if="replaceNeeded" message="确认后会替换旧登录会话；旧设备不能继续领取新的本地任务。设备绑定本身不会改变。" theme="warning" />
            <t-alert v-if="loginAction" :message="loginAction" theme="info" />
            <button type="submit" class="login-button primary" :disabled="submitting">{{ submitting ? loginAction : replaceNeeded ? '替换旧会话并登录' : '登录' }}</button>
            <button v-if="replaceNeeded" type="button" class="login-button outline" :disabled="submitting" @click="resetReplacePrompt">取消</button>
          </form>
          <p class="login-help">账号由管理员后台统一创建与维护，<br />如需开通请联系管理员。</p>
        </section>
      </section>
      <footer class="login-footer"><span></span>起飞 · 内容运营平台<span></span></footer>
    </main>
  </t-loading>
</template>

<style scoped>
.login-shell { position: relative; display: flex; flex-direction: column; min-height: 100vh; overflow: hidden; background: linear-gradient(125deg, #f8fcff 0%, #e8f4ff 66%, #f8fcff 100%); color: #11294c; font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans SC", sans-serif; }
.login-shell::before { content: ""; position: absolute; inset: 18% 0 0 0; background: url('/login-scene.svg') center bottom / cover no-repeat; pointer-events: none; }
.login-shell::after { content: ""; position: absolute; width: 55vw; height: 55vw; right: -18vw; top: -31vw; border-radius: 0 0 0 100%; background: #d9ecff55; pointer-events: none; }
.login-topbar, .login-layout, .login-footer { position: relative; z-index: 1; }
.login-topbar { display: flex; align-items: center; justify-content: space-between; padding: 34px 5.5% 0; }
.version-pill { display: inline-block; border: 1px solid #b9d8ff; border-radius: 100px; padding: 4px 12px; color: #126bf0; background: #e9f5ffb8; font-size: 13px; font-weight: 700; }
.login-layout { display: grid; grid-template-columns: minmax(0, 1fr) minmax(360px, 450px); gap: 5%; align-items: start; flex: 1; padding: 48px 5.5% 70px; }
.login-story { padding-top: 18px; }
.story-copy h1 { margin: 0 0 3px; color: #10284a; font-size: clamp(52px, 5.7vw, 84px); font-weight: 900; letter-spacing: .02em; line-height: 1.1; }
.story-subtitle { margin: 0; color: #5979a6; font-size: clamp(28px, 3.2vw, 46px); font-weight: 700; letter-spacing: .02em; }
.story-description { margin: 17px 0 0; color: #5f789a; font-size: clamp(14px, 1.3vw, 18px); }
.story-features { display: grid; grid-template-columns: repeat(4, minmax(100px, 1fr)); gap: 12px; max-width: 620px; margin-top: 33px; }
.feature-item { display: flex; flex-direction: column; gap: 4px; align-items: flex-start; }
.feature-icon { display: grid; place-items: center; width: 53px; height: 53px; margin-bottom: 4px; border-radius: 15px; background: linear-gradient(140deg, #f1f8ff, #cfe6ff); color: #1874f8; font-size: 28px; }
.feature-item strong { font-size: 15px; }.feature-item small { color: #7590b0; font-size: 12px; }
.login-card { box-sizing: border-box; width: 100%; padding: 38px 42px 34px; border: 1px solid #ffffffb8; border-radius: 17px; background: #fffffff0; box-shadow: 0 22px 60px #6ea3d52a; backdrop-filter: blur(12px); text-align: center; }
.card-brand { margin: 0 auto 22px; }.login-card h2 { margin: 0; font-size: 28px; }.login-intro { margin: 6px 0 27px; color: #8596ac; font-size: 15px; }
.login-form { display: flex; flex-direction: column; gap: 13px; text-align: left; }
.field-label { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
.login-field { display: flex; align-items: center; height: 48px; padding: 0 14px; gap: 10px; border: 1px solid #d5e2f0; border-radius: 8px; color: #8ca2bb; background: #fff; font-size: 20px; transition: border-color .2s, box-shadow .2s; }
.login-field:focus-within { border-color: #267efa; box-shadow: 0 0 0 3px #267efa1f; }
.login-field input { width: 100%; height: 100%; border: 0; outline: 0; color: #183557; background: transparent; font: inherit; font-size: 15px; }.login-field input::placeholder { color: #9babbd; }
.login-button {
  width: 100%;
  height: 48px;
  border-radius: 8px;
  border: 1px solid transparent;
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
}
.login-button:disabled {
  cursor: not-allowed;
  opacity: 0.72;
}
.login-button.primary {
  color: #fff;
  background: #2378f5;
  border-color: #2378f5;
  margin-top: 8px;
}
.login-button.primary:hover:not(:disabled) {
  background: #126de9;
  border-color: #126de9;
}
.login-button.outline {
  color: var(--td-text-color-primary);
  background: var(--td-bg-color-container);
  border-color: var(--td-border-level-2-color);
}
.login-button.outline:hover:not(:disabled) {
  background: var(--td-bg-color-container-hover);
}
.login-help { margin: 24px 0 0; color: #9baabd; font-size: 13px; line-height: 1.7; }
.login-footer { display: flex; align-items: center; justify-content: center; gap: 16px; padding: 14px; color: #839ab8; font-size: 13px; }
.login-footer span { display: block; width: 40px; height: 1px; background: #b7cfe8; }
@media (max-width: 1000px) { .login-layout { grid-template-columns: 1fr 390px; gap: 20px; }.story-features { grid-template-columns: repeat(2, minmax(100px, 1fr)); max-width: 320px; } }
@media (max-width: 760px) { .login-shell::before { opacity: .47; background-size: auto 55%; }.login-topbar { padding: 22px 20px 0; }.login-layout { display: flex; flex-direction: column; align-items: center; gap: 22px; padding: 30px 20px 30px; }.login-story { width: min(100%, 450px); padding: 0; }.story-copy h1 { font-size: 43px; }.story-subtitle { font-size: 25px; }.story-description, .story-features { display: none; }.login-card { max-width: 450px; padding: 26px 24px; }.card-brand { margin-bottom: 16px; }.login-footer { margin-top: auto; } }
</style>
