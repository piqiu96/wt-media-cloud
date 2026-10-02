import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const DIR = './apps/desktop/features/local-agent/'
const pill = readFileSync(new URL(`${DIR}WorkEnvPill.vue`, import.meta.url), 'utf8')
const status = readFileSync(new URL(`${DIR}work-env-status.js`, import.meta.url), 'utf8')
const service = readFileSync(new URL(`${DIR}service.js`, import.meta.url), 'utf8')

/** 取出两段定界文本之间的内容；找不到就抛，别让断言在空串上静默通过。 */
function sliceBetween(text, start, end) {
  const from = text.indexOf(start)
  const to = text.indexOf(end, from + start.length)
  if (from === -1 || to === -1) throw new Error(`找不到切片：${start} … ${end}`)
  return text.slice(from, to)
}

/**
 * 后台轮询可以复用比特浏览器扫描，用户看得见的几条路径不行。
 *
 * 一次扫描就是一次对本机 54345 的 `POST /browser/list`；30 秒一 tick 约 120 次/小时。
 * 胶囊只是个指示灯，允许 Agent 复用五分钟内的结果。但 `main_user_id` 正是绑定流程与云端
 * 执行门比对的那个值——手动「重新检查」、挂载首取、冷启动重试读它，就必须实时。
 *
 * 「哪几条路径传了」是这里唯一的事实：传错方向的后果不对称（少传一次，界面慢一秒；
 * 多传一次，用户点了重新检查却看到旧账号），所以两个方向都要钉。
 */
describe('work environment pill scan reuse', () => {
  it('reuses the scan on the automatic tick and on becoming visible', () => {
    expect(sliceBetween(pill, 'function syncTimer()', 'function onVisibility()')).toContain(
      'void refresh({ reuseScan: true })'
    )
    expect(sliceBetween(pill, 'function onVisibility()', 'onMounted(')).toContain(
      'void refresh({ reuseScan: true })'
    )
  })

  it('does not reuse the scan for anything the user asked for', () => {
    expect(sliceBetween(pill, 'async function manualCheck()', 'let retryTimer')).toContain(
      'refresh({ report: true })'
    )
    expect(sliceBetween(pill, 'function scheduleBootRetry()', '// 可见期间轮询')).toContain(
      'refresh({ report: true })'
    )
    expect(sliceBetween(pill, 'onMounted(() => {', 'onBeforeUnmount(')).toContain(
      'refresh({ report: true })'
    )
  })

  it('carries the flag from the pill down to the invoke call', () => {
    expect(sliceBetween(pill, 'async function refreshAll', '// 背景刷新')).toContain(
      'loadWorkEnvInputs({ session, devices, service, invoke, reuseScan })'
    )
    expect(sliceBetween(pill, 'async function refresh(', 'async function manualCheck')).toContain(
      'refreshAll({ reuseScan })'
    )
    expect(sliceBetween(status, 'export async function loadWorkEnvInputs', 'const errors =')).toContain(
      'service.status({ reuseScan })'
    )
    expect(sliceBetween(service, 'async status({ reuseScan', 'async health()')).toContain(
      'invoke(LOCAL_AGENT_COMMANDS.status, { reuseScan })'
    )
  })

  it('defaults to a live scan when nobody asks', () => {
    // 缺省必须是实时：`PersonalInfoPage`、账号检查、绑定流程都不传这个参数。
    expect(service).toContain('async status({ reuseScan = false } = {})')
    expect(status).toContain('export async function loadWorkEnvInputs({ session, devices, service, invoke, reuseScan = false })')
  })
})
