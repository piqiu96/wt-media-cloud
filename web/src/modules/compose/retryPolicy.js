// 为什么这条任务没有重试入口。受理范围是**闭集**（重试接口按同一个判据受理），再加上角色门槛，
// 于是「没有按钮」有不同的原因，页面必须说清是哪一种 —— 否则运营面对的是同一个没有按钮的
// 详情，只能猜。
//
// 判据与 ComposeTasksPage 的重试入口同源：页面据此决定给不给按钮，详情据此说明理由。

// 执行结果不明确：成片可能已经产生、也可能没有，机器判断不了。它有自己的管理员核对通道，
// 绕过核对直接重跑会把待核对的现场盖掉，所以它是唯一不被人工重试受理的失败。
export const UNCERTAIN_CODE = 'execution_uncertain'

// 人工重试受理已取消的，以及除执行结果不明确以外的一切失败。失败原因是素材缺失、模板停用
// 这类需要修数据的情况时，修完仍旧从这条任务重试 —— 一条 (策略, 素材) 只有一条任务。
export function isRetryableTask(task) {
  if (!task) return false
  if (task.status === 'cancelled') return true
  return task.status === 'failed' && task.error_code !== UNCERTAIN_CODE
}

const PERMISSION_HINT = '可重试，但当前账号无此权限（需高级运营或管理员）'

// 返回空串表示不必补话：要么这条任务本来就没有重试入口，要么能重试且有权限，要么执行结果
// 不明确（它的理由由详情里那条专用告警说）。空串以外的每一句都是「为什么这里没有按钮」的答案。
export function retryHint(task, canRetry) {
  if (!isRetryableTask(task)) return ''
  return canRetry ? '' : PERMISSION_HINT
}
