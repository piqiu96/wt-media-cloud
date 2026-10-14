const stageLabels = {
  validating_input: '校验素材',
  waiting_input: '等待素材准备',
  cutting: '视频合成',
  uploading: '上传成片',
  committing: '保存成片',
}

const errorLabels = {
  execution_uncertain: '结果不明确',
  storage_error: '存储暂时失败',
  task_timeout: '任务超时',
  input_missing: '源文件缺失',
  input_invalid: '素材校验失败',
  template_unavailable: '模板不可用',
  execution_failed: '执行失败',
}

export function safeComposeError(code) {
  return errorLabels[code] || '执行异常，请联系管理员查看诊断摘要'
}

// Only allowlisted fields enter the summary. Raw FFmpeg output, object keys,
// download URLs and the stored error_message never become diagnostic text.
export function diagnosticSummary(attempt) {
  const number = Number.isInteger(attempt?.attempt) && attempt.attempt > 0 ? attempt.attempt : 0
  const stage = stageLabels[attempt?.stage] || '阶段未记录'
  const category = attempt?.outcome === 'success' ? '已完成' : safeComposeError(attempt?.error_code)
  return `第 ${number} 次 · ${stage} · ${category}`
}

export function reviewReady({ oldExecutionStopped, noOutputCommitted, evidence }) {
  const note = String(evidence || '').trim()
  return oldExecutionStopped === true && noOutputCommitted === true &&
    Array.from(note).length >= 20 && Array.from(note).length <= 2000 && !note.includes('://')
}
