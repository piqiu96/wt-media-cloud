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
import { aggregateWorkEnv, loadWorkEnvInputs } from './work-env-status.js'

const POLL_MS = 30_000
// 冷启动退避。Agent sidecar 由 main.ts 以 fire-and-forget 方式启动，挂载时的第一次
// 取数常撞在它还没监听上（local_agent_status 直接返回错误）；只靠 30s 轮询来救，
// 用户会先看到一个自己点一下就能变绿的「需检查」。
const BOOT_RETRY_DELAYS = [2_000, 5_000, 10_000]

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

const deviceRowText = computed(() => {
  if (!env.value.bound) return '尚未绑定'
  return env.value.mismatched ? '已绑定其他电脑，当前不一致' : '正在这台电脑工作'
})
// 账号不一致时比特浏览器本身可能是好的，但「已登录指定账号」会撒谎——
// 指向该做的事（去个人信息页确认）而不是事实的另一半。
const bitRowText = computed(() => env.value.bitAccountDiffers ? '账号待确认' : env.value.page.bitbrowserText)

// 唯一的取数路径：挂载、手动「重新检查」、30s 轮询、窗口重新可见都走它。
// 四项输入一次读齐——此前轮询只刷 snapshot/binding，cloudUser 与 localDevice
// 挂载后再不重读，于是个人中心改完绑定，胶囊仍停在旧结论上。
// `reuseScan` 只由**自动**刷新传：比特浏览器扫描每次都要问一次本机 54345，
// 30 秒一 tick 约 120 次/小时。后台常驻的这盏灯只需要「结论」，可以让 Agent 复用
// 五分钟内的扫描；用户主动点「重新检查」、挂载首取、冷启动重试一律实时，
// 因为它们读的 main_user_id 正是绑定流程与执行门比对的那个值。
async function refreshAll({ reuseScan = false } = {}) {
  const inputs = await loadWorkEnvInputs({ session, devices, service, invoke, reuseScan })
  cloudUser.value = inputs.cloudUser
  localDevice.value = inputs.localDevice
  snapshot.value = inputs.snapshot
  binding.value = inputs.binding
  checkedAt.value = new Date()
  return inputs.errors
}

// 背景刷新（轮询、窗口重新可见）只更新事实，不动错误提示：会话过期这类持续失败
// 不该让胶囊常驻一条红色报错——行文案与阻断项已经把状态说清楚了。用户主动点的
// 「重新检查」和冷启动那几次才回报错，那时他正盯着这个面板。
async function refresh({ report = false, reuseScan = false } = {}) {
  let errors = []
  try {
    errors = await refreshAll({ reuseScan })
  } catch (e) {
    errors = [e]
  }
  if (report) error.value = errors.length ? String(errors[0]?.message || errors[0]) : ''
  return errors
}

async function manualCheck() {
  busy.value = true
  await refresh({ report: true })
  busy.value = false
}

let retryTimer = null
let retryIndex = 0
function scheduleBootRetry() {
  if (retryTimer || retryIndex >= BOOT_RETRY_DELAYS.length) return
  const delay = BOOT_RETRY_DELAYS[retryIndex]
  retryIndex += 1
  retryTimer = setTimeout(async () => {
    retryTimer = null
    await refresh({ report: true })
    if (!env.value.workable) scheduleBootRetry()
  }, delay)
}

// 可见期间轮询，窗口隐藏/最小化时暂停。
//
// 初始值不读 `document.hidden`：WebView 若在启动时就把自己报成隐藏，interval 会
// 一个都不建；万一它此后也不再发 visibilitychange，轮询就整场缺席，胶囊只能靠点击
// 更新。先乐观地当作可见，交给事件来纠正——多轮询几次是明面上的小代价。
let timer = null
let visible = true
function syncTimer() {
  if (timer) { clearInterval(timer); timer = null }
  if (visible) timer = setInterval(() => { void refresh({ reuseScan: true }) }, POLL_MS)
}
function onVisibility() {
  const wasVisible = visible
  visible = !document.hidden
  // 重新可见是一次「用户回到这个窗口」的事件，当下就刷新，而不是等下一次轮询。
  if (visible && !wasVisible) void refresh({ reuseScan: true })
  syncTimer()
}

onMounted(() => {
  void refresh({ report: true }).then(() => { if (!env.value.workable) scheduleBootRetry() })
  document.addEventListener('visibilitychange', onVisibility)
  syncTimer()
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  if (retryTimer) clearTimeout(retryTimer)
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<template>
  <t-popup trigger="click" placement="bottom-right">
    <button type="button" class="work-env-pill" :class="env.workable ? 'is-ok' : 'is-warn'" aria-label="工作环境状态" @click="manualCheck">
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
          <t-button size="small" variant="outline" :loading="busy" @click="manualCheck">重新检查</t-button>
        </header>
        <ul class="env-rows">
          <li><span class="row-label">工作设备</span><span class="row-value">{{ deviceRowText }}</span></li>
          <li><span class="row-label">本机服务</span><span class="row-value">{{ env.page.serviceText }}</span></li>
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
