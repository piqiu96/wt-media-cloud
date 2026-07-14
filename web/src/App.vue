<script setup>
import { onMounted, ref } from 'vue'

import { createSessionClient } from './session.js'

const client = createSessionClient()
const username = ref('')
const password = ref('')
const user = ref(null)
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    user.value = await client.me()
  } catch {
    user.value = null
  } finally {
    loading.value = false
  }
})

async function login() {
  error.value = ''
  try {
    user.value = await client.login(username.value, password.value)
    password.value = ''
  } catch (requestError) {
    error.value = requestError.message
  }
}

async function logout() {
  await client.logout()
  user.value = null
}
</script>

<template>
  <main class="shell">
    <section v-if="loading" class="card">正在检查会话…</section>
    <section v-else-if="!user" class="card login-card">
      <p class="eyebrow">WT MEDIA CLOUD</p>
      <h1>登录运营平台</h1>
      <form @submit.prevent="login">
        <label>用户名<input v-model.trim="username" autocomplete="username" required /></label>
        <label>密码<input v-model="password" type="password" autocomplete="current-password" required /></label>
        <p v-if="error" class="error">{{ error }}</p>
        <button type="submit">登录</button>
      </form>
    </section>
    <section v-else class="card">
      <p class="eyebrow">当前会话</p>
      <h1>{{ user.username }}</h1>
      <dl>
        <div><dt>角色</dt><dd>{{ user.role }}</dd></div>
        <div><dt>状态</dt><dd>{{ user.status }}</dd></div>
        <div><dt>游戏范围</dt><dd>{{ user.game_ids.length ? user.game_ids.join('、') : '全局 / 未分配' }}</dd></div>
      </dl>
      <button class="secondary" type="button" @click="logout">退出登录</button>
    </section>
  </main>
</template>

<style>
:root { font-family: Inter, ui-sans-serif, system-ui, sans-serif; color: #13231b; background: #edf3ef; }
* { box-sizing: border-box; }
body { margin: 0; }
.shell { min-height: 100vh; display: grid; place-items: center; padding: 32px; }
.card { width: min(440px, 100%); padding: 36px; border: 1px solid #c9d7cf; border-radius: 18px; background: #fff; box-shadow: 0 18px 50px rgba(26, 56, 39, .1); }
.eyebrow { margin: 0 0 8px; color: #387255; font-size: 12px; font-weight: 750; letter-spacing: .14em; }
h1 { margin: 0 0 28px; font-size: 28px; }
form { display: grid; gap: 18px; }
label { display: grid; gap: 8px; font-size: 14px; font-weight: 650; }
input { width: 100%; padding: 12px 14px; border: 1px solid #aebfb5; border-radius: 10px; font: inherit; }
button { padding: 12px 18px; border: 0; border-radius: 10px; background: #1d6a45; color: #fff; font: inherit; font-weight: 700; cursor: pointer; }
.secondary { background: #e5eee9; color: #174b34; }
.error { margin: 0; color: #a12626; font-size: 14px; }
dl { display: grid; gap: 12px; margin: 0 0 28px; }
dl div { display: flex; justify-content: space-between; gap: 24px; }
dt { color: #617067; }
dd { margin: 0; text-align: right; font-weight: 650; }
</style>
