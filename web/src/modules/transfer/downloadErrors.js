// 下载失败怎么讲给运营听。
//
// 两个来源、两套词表，别混：
//
// 1. **发起下载被拒**（HTTP 409／422）。响应信封里的 `error.type` 是冻结的错误名
//    （contracts/cloud-error-codes/v1/file-transfer.yaml）。它不是给日志用的——两个
//    409 的 errcode 无法区分，能区分的是这个名字。
// 2. **任务终结时报的错**（任务行上的 `error_code`）。词表来自 Agent 仓的
//    `contracts/local-error-codes/v1/transfer.yaml` 的 `executor_errors`：没有谁会被
//    调用去问「为什么失败」，原因跟着任务走到 Cloud，运营在任务行上读到的就是它。

const CREATE_FAILURE_MESSAGES = {
  // Cloud 只在素材的 `source_url` 为空时抛它：这是一个等多久都不会变的结论，
  // 不是「还没准备好」。原来那句「云端正在准备」是假承诺 —— 它让人以为等着就好了，
  // 而素材库里那条行照样点得动（点第二次、第三次，得到的都是这句话）。
  material_unavailable: '这个素材没有可用的来源地址，无法准备',
  // 这句提示给的是**去处**，不只是一个结论。「绑定」这个动作在「本地环境 → Agent 状态」
  // 页上（未绑定是「绑定当前比特浏览器账号」，已绑定是「确认并刷新本机可信环境」）；
  // 只说「请绑定」等于让运营自己去找那个按钮在哪一页。
  local_transfer_node_unavailable: '本机没有可用的下载节点，请到「本地环境 → Agent 状态」确认 Agent 已启动并绑定',
}

/**
 * 发起下载失败时的提示。
 *
 * 认不出 `type` 就退回服务端自己的 message —— 绝不吞掉它换成一句「操作失败」：
 * 那是把这个接口唯一能给出的诊断信息扔掉，剩下的只有让运营去翻日志。
 */
export function createDownloadFailureMessage(error) {
  const known = CREATE_FAILURE_MESSAGES[error?.type]
  return known || error?.message || '发起下载失败'
}

const EXECUTOR_ERROR_LABELS = {
  cancelled_by_user: '已由用户取消',
  download_save_dir_unset: '本机还没有选择保存目录',
  download_save_dir_unwritable: '保存目录已不可写',
  download_disk_insufficient: '保存目录所在磁盘空间不足',
  download_source_unavailable: '素材来源地址已失效',
  download_integrity_failed: '文件完整性校验未通过',
  download_stalled: '下载停滞超时',
  lease_unconfirmed: '与云端的租约未能确认，任务可能已被其他节点接管',
  download_name_unusable: '素材标题无法作为文件名',
}

/**
 * 任务行上的 `error_code` 对应的说明。
 *
 * 认不出来就原样显示这个码：空白会让人以为「没出错」，而一个没见过的码恰恰是
 * 最需要被看到的东西——它说明有一个失败原因还没被这个界面认识。
 */
export function taskErrorLabel(code) {
  if (!code) return ''
  return EXECUTOR_ERROR_LABELS[code] || code
}
