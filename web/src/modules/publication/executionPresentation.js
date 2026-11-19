import { publicationStatuses } from './status.js';
export const executionPhase = value => ({ claimed: '已领取任务', preparing: '准备资源', ready: '资源及账号已核对', submitting: '正在提交', awaiting_human: '等待人工', connection_lost: '连接中断，等待核实', finished: '执行已结束' })[value] || '等待执行器反馈';
export function executionVerification(attempt) {
  try { const value = JSON.parse(attempt.resolution_note || 'null'); return value && typeof value === 'object' ? value : null; } catch { return null; }
}
export function executionResult(attempt) {
  if (attempt.resolved_at && executionVerification(attempt)?.conclusion === 'failed') return { label: '已核实未发布', tone: 'danger' };
  const status = attempt.result_status || (attempt.status === 'interrupted' ? 'uncertain' : attempt.stage === 'awaiting_human' ? 'awaiting_human' : attempt.status === 'running' ? 'running' : 'uncertain');
  return { label: publicationStatuses[status] || '结果待确认', tone: { reviewing: 'warning', failed: 'danger', uncertain: 'warning', awaiting_human: 'warning', running: 'info', published: 'success' }[status] || 'warning' };
}
export function executionSummary(attempt) {
  if (attempt.resolved_at && executionVerification(attempt)?.conclusion === 'failed') return executionVerification(attempt)?.evidence?.reason || '人工已核实未发布。';
  if (attempt.error_code === 'PREPARATION_ONLY_NOT_SUBMITTED') return '执行结束，未上传、未投稿';
  if (attempt.error_message) return attempt.error_message;
  const result = executionResult(attempt).label;
  if (result === '平台审核中') return '平台已接收，正在审核；尚未确认公开。';
  if (result === '已发布') return '已确认发布成功。';
  if (result === '结果待确认') return '结果尚未确认，请先核实，避免重复发布。';
  return executionPhase(attempt.stage);
}
export function executionTimeline(attempt) {
  const steps = (Array.isArray(attempt.steps) ? [...attempt.steps] : []).sort((a, b) => Number(a.seq) - Number(b.seq));
  const events = steps.filter(step => step.report?.stage !== 'finished').map(step => ({ key: `step-${step.seq}`, at: step.at, label: step.label || executionPhase(step.report?.stage), tone: ['awaiting_human', 'connection_lost'].includes(step.report?.stage) ? 'warning' : 'success' }));
  // A completed attempt is not itself proof of submission or publication.
  if (attempt.finished_at || attempt.error_message) events.push({ key: 'result', at: attempt.finished_at || attempt.heartbeat_at, label: executionSummary({ ...attempt, resolved_at: null, resolution_note: null }), tone: executionResult({ ...attempt, resolved_at: null, resolution_note: null }).tone });
  else if (events.length) events.at(-1).tone = executionResult(attempt).tone;
  return events;
}
export function executionCoverName(attempt, task) {
  const snapshot = attempt.input_snapshot?.content;
  if (!snapshot?.cover_file_id) return '—';
  if (String(snapshot.cover_file_id) === String(task.content?.cover_file_id) && task.resources?.cover_name) return task.resources.cover_name;
  return `文件 ID：${snapshot.cover_file_id}`;
}
