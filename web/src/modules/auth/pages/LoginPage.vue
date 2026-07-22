<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createSessionClient } from '../../../shared/api/session.js'
import { canUseDesktop, isDesktop } from '../../../utils.js'

const router = useRouter()
const sessionClient = createSessionClient()
const username = ref('')
const password = ref('')
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const replaceNeeded = ref(false)
const loginAction = ref('')

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
    <div class="login-wrapper">
      <t-card :bordered="true" class="login-card">
        <template #title>
          <span style="font-weight:700; letter-spacing:0.04em">WT MEDIA CLOUD</span>
        </template>
        <template #subtitle>
          <span style="color:var(--td-text-color-secondary)">登录运营平台</span>
        </template>
        <t-form @submit.prevent="replaceNeeded ? submitReplacementLogin() : submitLogin()">
          <t-form-item label="用户名">
            <t-input v-model="username" placeholder="请输入用户名" autocomplete="username" @change="resetReplacePrompt" />
          </t-form-item>
          <t-form-item label="密码">
            <t-input v-model="password" type="password" placeholder="请输入密码" autocomplete="current-password" @change="resetReplacePrompt" />
          </t-form-item>
          <t-form-item v-if="error">
            <t-alert :message="error" theme="error" />
          </t-form-item>
          <t-form-item v-if="replaceNeeded">
            <t-alert
              message="确认后会替换旧 Web、Desktop 和 Agent 会话；旧会话不能继续发起新的浏览器窗口操作。"
              theme="warning"
            />
          </t-form-item>
          <t-form-item v-if="loginAction">
            <t-alert :message="loginAction" theme="info" />
          </t-form-item>
          <t-form-item>
            <button
              v-if="!replaceNeeded"
              type="button"
              class="login-button primary"
              :disabled="submitting"
              @click="submitLogin"
            >
              {{ submitting ? '正在登录…' : '登录' }}
            </button>
            <t-space v-else direction="vertical" style="width:100%">
              <button
                type="button"
                class="login-button primary"
                :disabled="submitting"
                @click="submitReplacementLogin"
              >
                {{ submitting ? '正在替换旧会话…' : '替换旧会话并登录' }}
              </button>
              <button
                type="button"
                class="login-button outline"
                :disabled="submitting"
                @click="resetReplacePrompt"
              >
                取消
              </button>
            </t-space>
          </t-form-item>
        </t-form>
      </t-card>
    </div>
  </t-loading>
</template>

<style scoped>
.login-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  padding: 24px;
  background: var(--td-bg-color-page);
}
.login-card { width: 400px; }
.login-button {
  width: 100%;
  height: 40px;
  border-radius: 4px;
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
  background: var(--td-brand-color);
  border-color: var(--td-brand-color);
}
.login-button.primary:hover:not(:disabled) {
  background: var(--td-brand-color-hover);
  border-color: var(--td-brand-color-hover);
}
.login-button.outline {
  color: var(--td-text-color-primary);
  background: var(--td-bg-color-container);
  border-color: var(--td-border-level-2-color);
}
.login-button.outline:hover:not(:disabled) {
  background: var(--td-bg-color-container-hover);
}
</style>
