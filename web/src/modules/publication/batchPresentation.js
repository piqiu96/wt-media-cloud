import { batchPendingCount, batchControlLabels } from './batchExecution.js';
import { executionLabels } from './batchPreparation.js';

// Execution status remains separate from configuration readiness and group progress.
export function batchStatus(batch) {
  if (batch.execution_status === 'running' && batch.execution?.phase === 'awaiting_processing') return '待处理';
  return executionLabels[batch.execution_status] || '状态待刷新';
}
export function batchActionLabel(action, batch) {
  if (action === 'stop' && batch.execution_status === 'not_started') return '终止';
  return { edit: '继续配置', detail: '详情', copy: '复制', process: '去处理' }[action] || batchControlLabels[action];
}
export function batchListActions(batch, owner) {
  if (!owner) return ['detail'];
  const allowed = batch.allowed_actions || [];
  if (['ended','cancelled'].includes(batch.execution_status)) return ['detail', ...(allowed.includes('copy') ? ['copy'] : [])];
  const before = batch.execution_status === 'not_started';
  const actions = before ? ['start', 'edit'].filter(a => allowed.includes(a))
    : batchPendingCount(batch) > 0 ? ['process'] : [];
  return [...actions, 'detail', ...['pause', 'resume', 'finish', 'stop', 'copy'].filter(a => allowed.includes(a))];
}

const batchRecordLabels = {
  'publication_batch.create': '保存批次', 'publication_batch.configure': '保存配置修改',
  created: '保存批次', configuration_updated: '保存配置修改',
  publication_batch_created: '保存批次', publication_batch_configuration_updated: '保存配置修改',
  'publication_batch.start': '开始发布', 'publication_batch.pause': '暂停发布',
  'publication_batch.resume': '继续发布', 'publication_batch.stop': '终止发布',
  'publication_batch.finish': '结束批次', 'publication_batch.ended': '批次执行结束',
  'publication_batch.group_assigned': '安排本组任务', 'publication_batch.group_ended': '本组处理结束',
  'publication_batch.stop_settled': '终止处理完成', 'publication_batch.plan_skeleton': '创建计划批次',
};
export function batchRecordLabel(event) {
  return batchRecordLabels[event] || '批次业务记录';
}
