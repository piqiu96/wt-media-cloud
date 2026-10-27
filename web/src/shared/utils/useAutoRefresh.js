import { getCurrentInstance, onMounted, onUnmounted } from 'vue'

// 列表页的定时刷新。用户 2026-10-27 裁定：内容挖掘、内容生产、系统设置的列表页不设
// 刷新按钮，改为每 10 秒自动重读一次。
//
// 与 useTaskPolling 的区别在语义：那个只在「有任务在跑」时轮询，这里只要页面开着就
// 刷新，因此必须自己让开用户正在做的事——抽屉/弹窗/表单打开、有选中行、正在提交时
// 跳过这一拍，避免列表在脚下换掉正在看的行。
export const AUTO_REFRESH_INTERVAL_MS = 10000

/**
 * 一个页面级的定时刷新器。
 *
 * `run` 由页面给，负责以 quiet 方式重读；这里只保证：同一时刻只有一个 timer、上一拍
 * 没回来就不再发、页面不可见或 `shouldSkip()` 为真时跳过、组件卸载即停。
 */
export function useAutoRefresh(run, { intervalMs = AUTO_REFRESH_INTERVAL_MS, shouldSkip = () => false, autoStart = true } = {}) {
  let timer = null
  let inFlight = false

  function paused() {
    // 后台标签页不刷新：看不见的列表不需要每 10 秒问一次服务端。
    if (typeof document !== 'undefined' && document.hidden) return true
    try {
      return Boolean(shouldSkip())
    } catch {
      // 判据自己出错时宁可少刷一次，也不要把异常抛进定时器。
      return true
    }
  }

  async function tick() {
    if (inFlight || paused()) return
    inFlight = true
    try {
      // 刷新失败保持上一次读数，不弹错（初次加载仍显式报错）。
      await run()
    } catch {
      /* 页面已在自己的 quiet 读取里记账 */
    } finally {
      inFlight = false
    }
  }

  function start() {
    if (timer) return
    timer = setInterval(tick, intervalMs)
  }

  function stop() {
    if (timer) clearInterval(timer)
    timer = null
  }

  // 挂在组件上才有挂载/卸载时机；节点环境里单独复用时由调用方启停。
  if (getCurrentInstance()) {
    if (autoStart) onMounted(start)
    onUnmounted(stop)
  }
  return { start, stop, tick }
}
