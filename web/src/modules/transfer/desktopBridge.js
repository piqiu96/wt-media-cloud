// 下载中心唯一一件只有桌面端能做的事：把已下载的文件交给系统打开。
//
// 文件落在运营这台机器上（Agent 写的），浏览器打不开它 —— 所以按钮由 `isDesktop()`
// 守卫，而不是「点了报个错」。云端的 Web 构建同样会打包这个模块（`AccountsPage.vue`
// 也是这么引 `@tauri-apps/api/core` 的），因此这里的判断必须发生在**调用时**：
// 静态导入在浏览器里是安全的，真正会炸的是没有 `__TAURI_INTERNALS__` 还去 invoke。
import { invoke } from '@tauri-apps/api/core'

// 名字与 Desktop 仓 `commands/downloads.rs` 里的命令逐字一致。
// `local_open_saved_file` **收名字不收路径** —— 收路径就等于把「打开任意文件」
// 变成一个可以被诱导的动作，而名字是在保存目录里按名匹配出来的。
export const TRANSFER_COMMANDS = Object.freeze({
  openSavedFile: 'local_open_saved_file',
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
