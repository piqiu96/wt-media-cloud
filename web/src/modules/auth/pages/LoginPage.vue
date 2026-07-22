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
const error = ref('')

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

async function login() {
  error.value = ''
  try {
    const user = await sessionClient.login(username.value, password.value)
    if (isDesktop() && !canUseDesktop(user)) {
      await sessionClient.logout().catch(() => {})
      error.value = '管理员和高级运营不能登录 Desktop，请使用 Cloud Web 管理。'
      return
    }
    router.push('/')
  } catch (e) {
    if (e.errcode === 20010) {
      const confirmed = window.confirm('当前账号已在其他位置登录。确认替换旧会话后，旧 Web、Desktop 和 Agent 将不能继续发起新的敏感操作。是否继续？')
      if (!confirmed) {
        error.value = '已取消登录，旧会话保持有效。'
        return
      }
      try {
        const user = await sessionClient.login(username.value, password.value, { replaceExisting: true })
        if (isDesktop() && !canUseDesktop(user)) {
          await sessionClient.logout().catch(() => {})
          error.value = '管理员和高级运营不能登录 Desktop，请使用 Cloud Web 管理。'
          return
        }
        router.push('/')
        return
      } catch (replaceError) {
        error.value = replaceError.message
        return
      }
    }
    error.value = e.message
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
        <t-form @submit="login">
          <t-form-item label="用户名">
            <t-input v-model="username" placeholder="请输入用户名" autocomplete="username" />
          </t-form-item>
          <t-form-item label="密码">
            <t-input v-model="password" type="password" placeholder="请输入密码" autocomplete="current-password" />
          </t-form-item>
          <t-form-item v-if="error">
            <t-alert :message="error" theme="error" />
          </t-form-item>
          <t-form-item>
            <t-button type="submit" theme="primary" block>登录</t-button>
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
</style>
