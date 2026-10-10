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

          <!-- 中央主视觉：抽象流量轨迹（内容进入 → 流动 → 分发 → 增长）。不再使用完整品牌标。 -->
          <div class="story-art" aria-hidden="true">
            <div class="story-orbit orbit-one"></div>
            <div class="story-orbit orbit-two"></div>

            <div class="story-core"></div>
            <svg class="story-flow" viewBox="0 0 690 460" role="presentation" focusable="false">
                <defs>
                  <linearGradient id="wtqFlowMain" x1="0" y1="1" x2="1" y2="0">
                    <stop offset="0%" stop-color="#126EF5" stop-opacity="0.12" />
                    <stop offset="36%" stop-color="#126EF5" stop-opacity="1" />
                    <stop offset="100%" stop-color="#00C4DC" stop-opacity="1" />
                  </linearGradient>
                  <linearGradient id="wtqFlowAux" x1="0" y1="1" x2="1" y2="0">
                    <stop offset="0%" stop-color="#479DFF" stop-opacity="0.12" />
                    <stop offset="45%" stop-color="#479DFF" stop-opacity="0.45" />
                    <stop offset="100%" stop-color="#00C4DC" stop-opacity="0.45" />
                  </linearGradient>
                  <linearGradient id="wtqFlowSoft" x1="0" y1="1" x2="1" y2="0">
                    <stop offset="0%" stop-color="#51D4E2" stop-opacity="0.06" />
                    <stop offset="100%" stop-color="#51D4E2" stop-opacity="0.3" />
                  </linearGradient>
                  <radialGradient id="wtqNodeBlue" cx="34%" cy="30%" r="74%">
                    <stop offset="0%" stop-color="#8ACBFF" />
                    <stop offset="100%" stop-color="#126EF5" />
                  </radialGradient>
                  <radialGradient id="wtqNodeCyan" cx="34%" cy="30%" r="74%">
                    <stop offset="0%" stop-color="#8CEDF8" />
                    <stop offset="100%" stop-color="#00B6D4" />
                  </radialGradient>
                </defs>

                <path class="flow-line flow-soft" d="M 70 280 C 190 300 265 253 350 200 S 505 124 560 106" />
                <path class="flow-line flow-aux" d="M 115 354 C 235 354 320 314 415 250 S 552 180 618 149" />
                <path class="flow-line flow-main" d="M 80 322 C 190 336 280 300 370 252 S 535 163 594 120" />
                <path class="flow-tip" d="M 608 110 L 585 119 L 600 135 Z" />

                <circle class="flow-node" cx="150" cy="325" r="7" fill="url(#wtqNodeCyan)" />
                <circle class="flow-node" cx="368" cy="252" r="21" fill="url(#wtqNodeBlue)" />
                <circle class="flow-node" cx="458" cy="204" r="11" fill="url(#wtqNodeCyan)" />
                <circle class="flow-node" cx="546" cy="146" r="7" fill="url(#wtqNodeCyan)" />
            </svg>

            <div class="story-mini-card story-media"><t-icon name="image" /></div>
            <div class="story-mini-card story-chart"><t-icon name="chart-bar" /></div>
            <div class="story-mini-card story-play"><t-icon name="play-circle" /></div>

            <span class="story-spark spark-one">✦</span>
            <span class="story-spark spark-two">✦</span>
            <span class="story-dot dot-one"></span>
            <span class="story-dot dot-two"></span>
            <span class="story-dot dot-three"></span>
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

/* 中央主视觉 */
.story-art{position:relative;width:min(760px,100%);height:460px;margin:28px auto 0}
.story-orbit{position:absolute;left:50%;top:50%;border-radius:50%;pointer-events:none}
.orbit-one{width:94%;height:52%;transform:translate(-50%,-50%) rotate(-16deg);border:1.5px solid rgba(255,255,255,.78)}
.orbit-two{width:86%;height:46%;transform:translate(-50%,-50%) rotate(14deg);border:1.5px solid rgba(0,190,220,.25)}

.story-core{position:absolute;left:50%;top:50%;z-index:1;width:min(450px,62%);aspect-ratio:1/1;transform:translate(-50%,-50%);border:1px solid rgba(255,255,255,.8);border-radius:50%;background:rgba(255,255,255,.72);box-shadow:0 16px 60px rgba(63,150,220,.08)}
.story-flow{position:absolute;inset:0;z-index:2;width:100%;height:100%;overflow:visible;pointer-events:none}
.flow-line{fill:none;stroke-linecap:round}
.flow-main{stroke:url(#wtqFlowMain);stroke-width:17px}
.flow-aux{stroke:url(#wtqFlowAux);stroke-width:9px}
.flow-soft{stroke:url(#wtqFlowSoft);stroke-width:6px}
.flow-tip{fill:#04B9DB}
.flow-node{stroke:rgba(255,255,255,.85);stroke-width:1.5px}

.story-mini-card{position:absolute;z-index:3;display:grid;place-items:center;width:92px;height:92px;border:1px solid rgba(255,255,255,.9);border-radius:var(--wt-card-radius);background:rgba(255,255,255,.82);box-shadow:var(--wt-card-shadow-soft);color:var(--wt-brand-primary);font-size:38px}
.story-media{left:0;top:12%}
.story-play{left:5%;bottom:9%}
.story-chart{right:0;top:20%}

.story-spark{position:absolute;z-index:2;color:rgba(120,196,255,.55);font-size:22px;line-height:1}
.spark-one{right:9%;bottom:12%}
.spark-two{left:2%;top:32%}
.story-dot{position:absolute;z-index:2;border-radius:50%;background:rgba(126,201,255,.45)}
.dot-one{left:14%;top:22%;width:9px;height:9px}
.dot-two{right:6%;top:46%;width:7px;height:7px}
.dot-three{left:23%;bottom:15%;width:11px;height:11px}

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
.login-button.primary{margin-top:6px;background:var(--wt-brand-cta-gradient);color:#fff}
.login-button.primary:hover:not(:disabled){filter:brightness(.98)}
.login-button.primary:active:not(:disabled){filter:brightness(.94)}
.login-button.outline{height:46px;background:#fff;border-color:#C8DCF0;color:#34547C;font-size:15px;font-weight:500}
.login-help{margin:38px 0 0;padding-top:24px;border-top:1px solid var(--wt-divider);color:var(--wt-ink-soft);font-size:14px;line-height:1.7}

.login-wave{position:absolute;z-index:1;left:-15%;width:130%;border-radius:45% 55% 0 0 / 38% 45% 0 0;pointer-events:none}
.wave-one{bottom:-160px;height:330px;background:#8FC9FF;opacity:.32;transform:rotate(-3deg)}
.wave-two{bottom:-199px;height:330px;background:#FFFFFF;opacity:.45;transform:rotate(4deg)}
.wave-three{bottom:-236px;height:300px;background:#BFE3FF;opacity:.25;transform:rotate(-2deg)}

@media(max-width:1400px){.login-layout{grid-template-columns:minmax(0,1.5fr) minmax(400px,.95fr);gap:2.5%}.story-art{height:420px}}
@media(max-width:1180px){.login-layout{grid-template-columns:minmax(0,1.35fr) minmax(370px,.95fr);gap:2%}.login-story{padding-left:0}.story-art{height:378px;margin-top:28px}.story-mini-card{width:80px;height:80px;font-size:32px}.login-card{max-width:470px;padding:44px 34px 32px}}
@media(max-width:760px){.login-topbar{padding:19px 20px 0}.login-layout{display:flex;flex-direction:column;gap:18px;width:calc(100% - 38px);padding:14px 0 80px}.login-story{width:100%;padding-left:0}.story-copy h1{font-size:clamp(40px,10vw,66px)}.story-description{margin-top:14px;font-size:15px}.story-art{display:none}.login-card{width:100%;max-width:520px;min-height:auto;padding:34px 24px 26px}.login-card h2{font-size:29px}.login-intro{margin:6px 0 26px;font-size:15px}.login-form{gap:16px}.login-field{height:52px}.login-field input{font-size:16px}.login-button{height:54px;font-size:17px}.login-help{margin-top:26px;padding-top:20px}.login-wave{height:190px}.wave-one{bottom:-110px}.wave-two{bottom:-130px}.wave-three{bottom:-150px}}
</style>
