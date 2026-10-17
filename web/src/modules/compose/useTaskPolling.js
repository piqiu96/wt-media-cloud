import { getCurrentInstance, onUnmounted, ref } from 'vue'

// 合成任务的状态自动刷新。只服务「有任务在排队或执行」这一段窗口：没有活动任务时
// 一个定时器都不留（§三十九：不产生重复 timer、不叠加请求、卸载即停）。
//
// 列表与详情各持一份实例，互不干扰——详情打开时它是任务自己的读数，不该跟着列表走。
export const POLL_INTERVAL_MS = 5000
const ACTIVE_STATUSES = new Set(['pending', 'running'])

/** 这个状态还处在执行中吗（排队中/处理中）。 */
export function isActiveStatus(status) {
  return ACTIVE_STATUSES.has(status)
}

/** 这一批行里还有没有活动任务。列表轮询的启停判据。 */
export function hasActiveTask(rows) {
  return (Array.isArray(rows) ? rows : []).some((row) => isActiveStatus(row?.status))
}

/**
 * 一个可启停的最小轮询器。
 *
 * `run` 由调用方给（它知道该重读哪一个接口、失败该怎么记账）。这里只负责三件事：
 * 按节奏调用、同一时刻只有一个 timer、上一轮没回来就不开下一轮。
 */
export function useTaskPolling(run, { intervalMs = POLL_INTERVAL_MS } = {}) {
  let timer = null
  let inFlight = false
  const polling = ref(false)

  async function tick() {
    // 上一轮还没回来就跳过这一拍：慢接口 + 定时器叠加会自己把自己压垮。
    if (inFlight) return
    inFlight = true
    try {
      // run 自己负责记账（页面把失败写进它自己的提示位）；这里吞掉的是「一拍都不许
      // 掀翻页面」——定时器里抛出去只会变成没人接的 rejection。
      await run()
    } catch {
      /* 调用方已经处理 */
    } finally {
      inFlight = false
    }
  }

  function start() {
    if (timer) return
    timer = setInterval(tick, intervalMs)
    polling.value = true
  }

  function stop() {
    if (timer) clearInterval(timer)
    timer = null
    polling.value = false
  }

  // 挂在组件上才有卸载时机；脱离组件复用时（节点环境下没有实例）由调用方负责停。
  if (getCurrentInstance()) onUnmounted(stop)
  return { start, stop, polling }
}
