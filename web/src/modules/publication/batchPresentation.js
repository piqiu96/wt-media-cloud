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
