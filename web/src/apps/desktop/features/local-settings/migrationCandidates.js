// 「改保存位置时要搬哪些文件」——候选名单从哪来。
//
// ## 名单只有一个来源，而且不是文件系统
//
// 云端任务表里那些**这台机器**执行过、并且执行器报了文件名的行。理由有两条，都不是
// 偏好：
//
// 1. 磁盘上没有「哪些文件是这个 App 下的」这件事。保存位置可能是 `~/Downloads`，
//    整目录扫描会把运营自己的东西一并列进来，而搬运和删除都不是可以猜的动作。
// 2. 反过来，Session 里那张表是「这台机器为这个运营下过什么」的**权威记录**：名字是
//    执行器写下去的那一个（换目录、重命名、` (2)` 后缀都只反映在它上面）。
//
// 于是判据是 `execution_scope === 'local_agent'` 而不是「文件在不在当前目录里」——
// 前者是事实，后者恰恰是这一页要回答的问题。
//
// ## 取不到名单时要报出原因
//
// 「读云端记录失败」和「这台机器没下过文件」在界面上是两件事：前者该说接口为什么
// 没答上来，后者是一句「暂无可搬文件」。把前者显示成后者，运营会以为东西被清空了。
// 所以 `loadCandidateNames` 抛错，由页面决定怎么讲；它绝不返回空数组冒充答案。
import { createFileTransferClient } from '../../../../shared/api/fileTransfer.js'

/**
 * 这台机器下过的文件名，去重后按名字排序。
 *
 * 空名字跳过：执行器没报名字的任务没有可定位的对象，一个空串送进 Rust 只会换来
 * 「不是一个文件名」。名字**不裁剪**——磁盘上的 `a.mp4` 与 ` a.mp4` 是两个文件，
 * 裁掉前导空格就会让后者的搬运去找错对象；只把全空白当成没有名字。
 */
export function candidateNames(tasks) {
  const names = new Set()
  for (const task of Array.isArray(tasks) ? tasks : []) {
    if (task?.execution_scope !== 'local_agent') continue
    const name = typeof task?.file_name === 'string' ? task.file_name : ''
    if (!name.trim()) continue
    names.add(name)
  }
  return [...names].sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'))
}

/** 取名单。取不到就带着原因抛出去，不返回空名单。 */
export async function loadCandidateNames({ client = createFileTransferClient() } = {}) {
  let tasks
  try {
    tasks = await client.listTasks()
  } catch (error) {
    throw new Error(`读取本机下载记录失败：${error?.message || String(error)}`)
  }
  return candidateNames(tasks)
}
