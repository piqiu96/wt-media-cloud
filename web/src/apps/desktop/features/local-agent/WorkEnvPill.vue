<script setup>
// 顶栏常驻的工作环境状态胶囊。设计原则：用户只关心正常/不正常——
// 正常时安静（绿点 + 短文案），异常才变色并展开面板看「哪一项坏了、为什么、怎么修」。
//
// 取数全部走 Tauri invoke 与 Cloud API（Local Agent 服务/设备绑定），不直连本机端口、
// 不接触执行凭据。定时自检只做只读的本地状态 + 云端绑定。
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { createLocalAgentService } from './service.js'
import { createDeviceBindingClient } from '../../../../shared/api/deviceBinding.js'
import { createSessionClient } from '../../../../shared/api/session.js'
import { aggregateWorkEnv } from './work-env-status.js'

const POLL_MS = 30_000

const service = createLocalAgentService({ invoke })
const devices = createDeviceBindingClient()
const session = createSessionClient()

const snapshot = ref(null)
const cloudUser = ref(null)
const binding = ref(null)
const localDevice = ref(null)
const checkedAt = ref(null)
const busy = ref(false)
const error = ref('')

const env = computed(() => aggregateWorkEnv({
  snapshot: snapshot.value,
  cloudUser: cloudUser.value,
  binding: binding.value,
  localDevice: localDevice.value,
}))

const checkedTime = computed(() => {
  if (!checkedAt.value) return '—'
  return new Date(checkedAt.value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
})

const taskText = computed(() => snapshot.value?.currentTaskId ? `当前任务 ${snapshot.value.currentTaskId}` : '当前无任务')

const deviceRowText = computed(() => {
  if (!env.value.bound) return '尚未绑定'
  return env.value.mismatched ? '已绑定其他电脑，当前不一致' : '正在这台电脑工作'
})
const localRowText = computed(() => env.value.page.bound ? `已连接，${taskText.value}` : '未连接执行节点')
const bitRowText = computed(() => env.value.page.bound ? '已登录指定账号' : env.value.page.bitbrowserStatusText)

// 轻量自检：只读的本地 Agent 状态 + 云端绑定，供 30s 轮询与「重新检查」共用。
async function refreshStatus() {
  const [s, b] = await Promise.all([service.status(), devices.get()])
  snapshot.value = s
  binding.value = b
}

// 完整检查：在轻量自检之上补当前用户与本机设备身份（登录态/设备一致性的输入）。
async function check() {
  busy.value = true
  error.value = ''
  try {
    const [me, ld] = await Promise.all([
      session.me(),
      invoke('local_device_identity').catch(() => null),
    ])
    cloudUser.value = me
    localDevice.value = ld
    await refreshStatus()
    checkedAt.value = new Date()
  } catch (e) {
    error.value = e?.message || '检查失败'
  } finally {
    busy.value = false
  }
}

// 可见期间轮询，窗口隐藏/最小化时暂停（document.hidden 由 Tauri WebView 反映窗口可见性）。
let timer = null
let visible = typeof document !== 'undefined' ? !document.hidden : true
function syncTimer() {
  if (timer) { clearInterval(timer); timer = null }
  if (visible) timer = setInterval(() => refreshStatus().catch(() => {}), POLL_MS)
}
function onVisibility() {
  visible = !document.hidden
  syncTimer()
}

onMounted(() => {
  check()
  document.addEventListener('visibilitychange', onVisibility)
  syncTimer()
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<template>
  <t-popup trigger="click" placement="bottom-right">
    <button type="button" class="work-env-pill" :class="env.workable ? 'is-ok' : 'is-warn'" aria-label="工作环境状态">
      <span class="env-dot" />
      <span class="env-text">{{ env.workable ? '工作环境' : '工作环境需检查' }}</span>
    </button>
    <template #content>
      <div class="env-panel">
        <header class="env-head">
          <div class="env-title">
            <strong>{{ env.workable ? '这台电脑可以工作' : '这台电脑需要检查' }}</strong>
            <span class="env-checked">最近检查：{{ checkedTime }}</span>
          </div>
          <t-button size="small" variant="outline" :loading="busy" @click="check">重新检查</t-button>
        </header>
        <ul class="env-rows">
          <li><span class="row-label">工作设备</span><span class="row-value">{{ deviceRowText }}</span></li>
          <li><span class="row-label">本机服务</span><span class="row-value">{{ localRowText }}</span></li>
          <li><span class="row-label">比特浏览器</span><span class="row-value">{{ bitRowText }}</span></li>
        </ul>
        <template v-if="!env.workable">
          <ul class="env-blockers">
            <li v-for="(reason, i) in env.blockers" :key="i">{{ reason }}</li>
          </ul>
        </template>
        <t-alert v-if="error" :message="error" theme="error" style="margin-top:10px" />
      </div>
    </template>
  </t-popup>
</template>

<style scoped>
.work-env-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  border: 1px solid #e7edf5;
  border-radius: 16px;
  background: #fff;
  color: #193250;
  font-size: 12px;
  cursor: pointer;
  transition: border-color .15s, box-shadow .15s;
}
.work-env-pill:hover { border-color: #b8d1f5; box-shadow: 0 2px 8px #1733570f; }
.env-dot { width: 8px; height: 8px; border-radius: 50%; }
.work-env-pill.is-ok .env-dot { background: #16a34a; }
.work-env-pill.is-ok .env-text { color: #167e5c; }
.work-env-pill.is-warn .env-dot { background: #ea7d10; }
.work-env-pill.is-warn .env-text { color: #b45309; }
.env-panel { width: 320px; padding: 4px; }
.env-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; padding: 8px 4px 12px; }
.env-title { display: flex; flex-direction: column; gap: 4px; }
.env-title strong { font-size: 14px; color: #193250; }
.env-checked { color: #8294ab; font-size: 12px; }
.env-rows { margin: 0; padding: 4px 0; list-style: none; }
.env-rows li { display: flex; justify-content: space-between; gap: 12px; padding: 9px 4px; border-top: 1px solid #eef2f6; }
.row-label { color: #8a9bb0; }
.row-value { color: #193250; text-align: right; }
.env-blockers { margin: 6px 0 10px; padding: 8px 12px; list-style: none; border: 1px solid #f7cdab; border-radius: 8px; background: #fff8ed; color: #99550d; font-size: 12px; line-height: 1.7; }
</style>
