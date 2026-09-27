// 下载中心唯一一件只有桌面端能做的事：把已下载的文件交给系统打开。
//
// 文件落在运营这台机器上（Agent 写的），浏览器打不开它 —— 所以按钮由 `isDesktop()`
// 守卫，而不是「点了报个错」。云端的 Web 构建同样会打包这个模块（`AccountsPage.vue`
// 也是这么引 `@tauri-apps/api/core` 的），因此这里的判断必须发生在**调用时**：
// 静态导入在浏览器里是安全的，真正会炸的是没有 `__TAURI_INTERNALS__` 还去 invoke。
import { invoke } from '@tauri-apps/api/core'

// 名字与 Desktop 仓 `commands/downloads.rs` / `commands/saved_files.rs` 里的命令
// 逐字一致。两个命令**都只收名字不收路径** —— 收路径就等于把「打开任意文件」变成
// 一个可以被诱导的动作，而名字是在已知保存位置里按名匹配出来的。
export const TRANSFER_COMMANDS = Object.freeze({
  openSavedFile: 'local_open_saved_file',
  savedFileStates: 'local_saved_file_states',
})

export function isDesktopRuntime() {
  return typeof window !== 'undefined' && window.__TAURI_INTERNALS__ !== undefined
}

/**
 * 用系统默认程序打开一个已下载的文件。
 *
 * 传的是**执行器报告的文件名**（`file_name`），不是推导出来的：名字是它写下去的那一个，
 * 换目录、重命名、加 ` (2)` 后缀都只反映在那个字段上。
 */
export async function openSavedFile(name, { invokeImpl = invoke } = {}) {
  if (!isDesktopRuntime()) throw new Error('打开文件只能在桌面客户端执行')
  if (!name) throw new Error('这个任务没有报告文件名，无法定位文件')
  return invokeImpl(TRANSFER_COMMANDS.openSavedFile, { name })
}

/**
 * 一批文件名现在各自在哪儿 —— 一次 invoke 问完全部，不按文件逐个问。
 *
 * 返回**按名字索引**的表，与 `downloadFacts.fileFact` 的索引方式一致：磁盘上只有一个
 * 文件，而一个名字可能对应多条任务（重试、重新下载各一条）。
 *
 * 与 `openSavedFile` 不同，这个函数在浏览器里**不抛异常**，返回空表。两者的区别不是
 * 风格问题：打开文件是一个动作，浏览器做不到，所以必须说清楚；而「这个文件在哪」是一份
 * 本机事实，浏览器读不到本机 —— 读不到就是 `unknown`，那正是空表的意思，也是今天的行为。
 * 把「我读不到」渲染成「文件不在」，才是这一整块要防的那个错误。
 *
 * 名字为空时同样不发请求：一次注定无事可做的跨进程调用只会把「没名字」这件事变成
 * 一条可能失败的命令。
 */
export async function savedFileStates(names, { invokeImpl = invoke } = {}) {
  const asked = [...new Set((Array.isArray(names) ? names : []).filter((name) => typeof name === 'string' && name.trim()))]
  if (!asked.length) return {}
  if (!isDesktopRuntime()) return {}

  const facts = await invokeImpl(TRANSFER_COMMANDS.savedFileStates, { names: asked })
  const table = {}
  for (const fact of Array.isArray(facts) ? facts : []) {
    const name = typeof fact?.name === 'string' ? fact.name : ''
    if (!name) continue
    // 目录在，且就是当前选定的那个 → `present_current`；目录在但不是当前的 → 文件在旧的
    // 保存位置里（改过目录的文件就是这一种）；没有目录 → 查过、不在。
    const directory = typeof fact?.directory === 'string' ? fact.directory : ''
    table[name] = {
      name,
      presence: directory ? (fact.current ? 'present_current' : 'present_elsewhere') : 'absent',
      directory,
      bytes: Number.isFinite(fact?.bytes) ? fact.bytes : null,
    }
  }
  return table
}
