<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createSessionClient } from '../../../shared/api/session.js'

const router = useRouter()
const sessionClient = createSessionClient()
const username = ref('')
const password = ref('')
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    await sessionClient.me()
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
    await sessionClient.login(username.value, password.value)
    router.push('/')
  } catch (e) {
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
