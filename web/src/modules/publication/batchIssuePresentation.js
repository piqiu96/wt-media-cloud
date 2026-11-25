// Merge only known equivalent validation failures; preserve unfamiliar provider errors.
const fields = { cover: 'cover_file_id', portrait: 'portrait_cover_file_id', confirmation: 'confirmed' };
const messages = {
  'cover_missing:cover_file_id': '请选择横版发布封面（16:9）',
  'cover_missing:portrait_cover_file_id': '请选择竖版发布封面（3:4）',
  'confirmation_required:confirmed': '请核对并确认当前文案、封面和声明',
  'output_occupied:composite_output_id': '成片已被当前平台任务占用',
};
export function presentBatchIssues(issues) {
  const seen = new Set();
  return issues.flatMap(issue => {
    const field = fields[issue.field] || issue.field || '';
    const identity = `${issue.code}:${field}`;
    const message = messages[identity] || issue.message;
    const key = JSON.stringify([issue.row_key || '', messages[identity] ? identity : [field, issue.code || '', message]]);
    if (seen.has(key)) return [];
    seen.add(key);
    return [{ ...issue, field, message }];
  });
}
export function localContentIssue(issue, rowKey) {
  const code = { cover: 'cover_missing', portrait: 'cover_missing', confirmation: 'confirmation_required' }[issue.field];
  return { ...issue, row_key: rowKey, code };
}
