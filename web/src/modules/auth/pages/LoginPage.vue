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
      <div class="login-glow login-glow-left" aria-hidden="true"></div>
      <div class="login-glow login-glow-right" aria-hidden="true"></div>
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
          <div class="story-art" aria-hidden="true">
            <div class="story-orbit orbit-one"></div><div class="story-orbit orbit-two"></div>
            <div class="story-mini-card story-media"><t-icon name="image" /></div>
            <div class="story-mini-card story-chart"><t-icon name="chart-bar" /></div>
            <div class="story-mini-card story-play"><t-icon name="play-circle" /></div>
            <div class="story-emblem"></div>
            <span class="story-spark spark-one">✦</span><span class="story-spark spark-two">✦</span>
          </div>
        </div>

        <section class="login-card" aria-labelledby="login-heading">
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
            <button type="submit" class="login-button primary" :disabled="submitting">{{ submitting ? loginAction : replaceNeeded ? '替换旧会话并登录' : '登录' }} <span v-if="!submitting" aria-hidden="true">→</span></button>
            <button v-if="replaceNeeded" type="button" class="login-button outline" :disabled="submitting" @click="resetReplacePrompt">取消</button>
          </form>
          <p class="login-help">账号由管理员创建与维护，如需开通请联系管理员。</p>
        </section>
      </section>
      <div class="login-wave wave-one" aria-hidden="true"></div><div class="login-wave wave-two" aria-hidden="true"></div>
    </main>
  </t-loading>
</template>

<style scoped>
.login-shell{position:relative;isolation:isolate;display:flex;flex-direction:column;min-height:100vh;overflow:hidden;background:linear-gradient(135deg,#f9fdff 0%,#edf8ff 45%,#cfeaff 79%,#f8fcff 100%);color:#071b50;font-family:"PingFang SC","Microsoft YaHei","Noto Sans SC",sans-serif}
.login-glow{position:absolute;z-index:-1;pointer-events:none;border-radius:50%;filter:blur(22px)}.login-glow-left{width:70vw;height:37vw;left:-28vw;top:-17vw;background:#fff}.login-glow-right{width:68vw;height:49vw;right:-27vw;top:-22vw;background:#a6d6ffb8}
.login-topbar,.login-layout{position:relative;z-index:2}.login-topbar{display:flex;align-items:center;justify-content:space-between;padding:26px 4% 0}.brand-link{text-decoration:none}.version-pill{display:inline-block;padding:4px 11px;border:1px solid #b9d8ff;border-radius:100px;background:#e9f5ffb8;color:#126bf0;font-size:12px;font-weight:700}
.login-layout{display:grid;grid-template-columns:minmax(0,1.6fr) minmax(370px,.9fr);align-items:center;gap:4%;flex:1;width:min(1600px,92%);margin:0 auto;padding:12px 0 75px}.login-story{position:relative;min-height:630px}.story-copy{position:relative;z-index:2;padding:52px 0 0 9%}.story-copy h1{position:relative;width:max-content;max-width:100%;margin:0;font-size:clamp(58px,5.7vw,104px);font-weight:950;letter-spacing:-.07em;line-height:1.15;white-space:nowrap}.story-copy h1 span{background:linear-gradient(105deg,#057cf1,#01c6e8 63%,#1264f9);-webkit-background-clip:text;background-clip:text;color:transparent}.story-copy h1 i{position:absolute;bottom:-4px;right:3%;width:47%;height:18px;border-bottom:7px solid #09d0e8;border-radius:50%;transform:rotate(-5deg)}.story-description{margin:15px 0 0;color:#415e8b;font-size:clamp(16px,1.6vw,24px);line-height:1.6;white-space:nowrap}
.story-art{position:absolute;inset:175px 0 0 0}.story-orbit{position:absolute;border:3px solid #ffffffe3;border-radius:50%;box-shadow:0 0 30px #7acfff,inset 0 0 25px #fff}.orbit-one{width:86%;height:56%;left:5%;top:19%;transform:rotate(-18deg)}.orbit-two{width:79%;height:50%;left:9%;top:24%;border-color:#adf1ff;transform:rotate(15deg)}.story-emblem{position:absolute;z-index:1;width:350px;height:350px;left:calc(50% - 175px);top:17px;border-radius:50%;background:#fff url('/qifei-reference-logo.png') center 54% / 150% no-repeat;box-shadow:0 0 0 12px #edffffad,0 21px 65px #62aee85e}.story-mini-card{position:absolute;z-index:2;display:grid;place-items:center;width:100px;height:85px;border:3px solid #fff;border-radius:18px;background:#effaffaf;box-shadow:0 12px 31px #49a7e68c;color:#33a8f8;font-size:48px}.story-media{left:4%;top:17%;transform:rotate(-10deg)}.story-chart{right:5%;top:19%;transform:rotate(7deg)}.story-play{left:10%;bottom:10%;width:77px;height:65px;font-size:37px;transform:rotate(-9deg)}.story-spark{position:absolute;z-index:2;color:#fff;font-size:42px;text-shadow:0 0 15px #11bfe5}.spark-one{right:12%;bottom:8%}.spark-two{left:0;top:45%}
.login-card{box-sizing:border-box;width:100%;max-width:520px;min-height:570px;padding:65px 44px 38px;border:1px solid #fff;border-radius:23px;background:#ffffffed;box-shadow:0 22px 67px #61a3d237;backdrop-filter:blur(14px);text-align:center}.login-card h2{margin:0;color:#081848;font-size:38px;letter-spacing:.08em}.login-intro{margin:10px 0 42px;color:#7589a8;font-size:18px}.login-form{display:flex;flex-direction:column;gap:21px;text-align:left}.field-label{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}.login-field{display:flex;align-items:center;gap:13px;height:59px;padding:0 18px;border:1px solid #d4e0f1;border-radius:10px;background:#fbfcff;color:#8295b6;font-size:23px;transition:border-color .2s,box-shadow .2s}.login-field:focus-within{border-color:#247fff;box-shadow:0 0 0 3px #247fff1f}.login-field input{min-width:0;width:100%;height:100%;border:0;outline:0;background:transparent;color:#183557;font-size:18px}.login-field input::placeholder{color:#8295b6}.password-toggle{display:grid;place-items:center;flex:none;width:32px;height:32px;padding:0;border:0;background:transparent;color:#8295b6;font-size:21px;cursor:pointer}.password-toggle:hover{color:#126ff4}
.login-button{position:relative;width:100%;height:66px;border:1px solid transparent;border-radius:11px;font-size:22px;font-weight:700;cursor:pointer}.login-button:disabled{cursor:not-allowed;opacity:.7}.login-button.primary{margin-top:2px;background:linear-gradient(100deg,#0868f8,#06c6e3);box-shadow:0 12px 24px #168be43c;color:#fff}.login-button.primary:hover:not(:disabled){filter:brightness(1.04)}.login-button.primary span{position:absolute;right:33%;font-size:26px}.login-button.outline{height:45px;background:#fff;border-color:#c8dcf0;color:#34547c;font-size:15px}.login-help{margin:33px 0 0;padding-top:26px;border-top:1px solid #d9e6f4;color:#748bad;font-size:14px;line-height:1.7}
.login-wave{position:absolute;z-index:1;bottom:-160px;left:-6%;width:112%;height:330px;border-radius:45% 55% 0 0 / 38% 45% 0 0;pointer-events:none}.wave-one{background:#90cbff8c;transform:rotate(-3deg)}.wave-two{bottom:-199px;background:#ffffffb0;transform:rotate(4deg)}
@media(max-width:1100px){.login-layout{grid-template-columns:minmax(0,1fr) minmax(340px,430px);gap:2%}.login-story{min-height:560px}.story-copy{padding-left:0}.story-copy h1{font-size:clamp(45px,5.1vw,68px)}.story-description{font-size:15px;white-space:normal}.story-art{inset:175px 0 0}.story-emblem{width:260px;height:260px;left:calc(50% - 130px)}.login-card{min-height:540px;padding:55px 30px 30px}}
@media(max-width:760px){.login-topbar{padding:19px 20px 0}.login-layout{display:flex;flex-direction:column;gap:18px;width:calc(100% - 38px);padding:14px 0 80px}.login-story{width:100%;min-height:160px}.story-copy{padding:14px 0}.story-copy h1{font-size:clamp(40px,10vw,66px)}.story-description{margin-top:10px;font-size:14px}.story-art{display:none}.login-card{max-width:520px;min-height:auto;padding:33px 24px 26px}.login-card h2{font-size:29px}.login-intro{margin:6px 0 24px;font-size:15px}.login-form{gap:14px}.login-field{height:52px}.login-field input{font-size:16px}.login-button{height:54px;font-size:18px}.login-button.primary span{right:28%}.login-help{margin-top:22px;padding-top:20px}.login-wave{height:190px;bottom:-110px}.wave-two{bottom:-130px}}
</style>
