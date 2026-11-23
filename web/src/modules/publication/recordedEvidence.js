import { executionVerification } from './executionPresentation.js';

// Display evidence already recorded for this exact attempt. The existing
// transaction checks remain authoritative when saving a management decision.
export function recordedPublicationEvidence(task, attempts) {
  const a = attempts.find(a => a.id === task.current_attempt_id && a.publication_id === task.id && a.executor_user_id === task.operator_id);
  if (!a || a.input_snapshot?.platform_uid !== task.platform_account_uid) return null;
  const resolution = executionVerification(a);
  if (a.resolved_at && resolution?.attempt_id === a.id && resolution.conclusion === task.status && resolution.evidence?.platform_uid === task.platform_account_uid && resolution.evidence.scene_closed) {
    return { scene_closed: true, no_publishable_work: task.status === 'failed' && resolution.evidence.no_publishable_work === true, basis: resolution.evidence.basis };
  }
  const result = a.evidence, ev = result?.evidence;
  if (a.status !== 'completed' || !a.result_hash || result?.status !== task.status || !ev?.scene_closed || ev.platform_uid !== task.platform_account_uid) return null;
  if (task.status === 'failed' && ev.not_submitted && !a.submit_started_at)
    return { scene_closed: true, no_publishable_work: true, basis: a.error_message || '原尝试已记录现场安全结束、未投稿' };
  if (task.status === 'reviewing' && a.submit_started_at && result.platform_work_id && ev.receipt_id === result.platform_work_id)
    return { scene_closed: true, no_publishable_work: false, basis: `原尝试已记录现场安全结束，投稿回执 ${ev.receipt_id}` };
  return null;
}
