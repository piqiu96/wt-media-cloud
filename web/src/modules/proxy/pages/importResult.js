const STATUS = { imported: 'success', failed: 'failed', skipped: 'skipped' }

export function reconcileImportItems(items, rows = []) {
  const byLine = new Map(rows.map(row => [Number(row.line), row]))
  return items.map(item => {
    if (item.status !== 'pending') return item
    const row = byLine.get(Number(item.key))
    if (!row) return { ...item, status: 'unconfirmed', reason: '服务未返回此行结果，请先核实，暂不重复导入。' }
    return { ...item, status: STATUS[row.status] || 'unconfirmed', reason: row.reason || '' }
  })
}

export function markImportUnknown(items) {
  return items.map(item => item.status === 'pending'
    ? { ...item, status: 'unconfirmed', reason: '提交结果未返回，请先核实系统是否已保存此行；暂不直接重试。' }
    : item)
}

export function prepareFailedCheckRetry(items, ids) {
  const selected = new Set(ids)
  return items.map(item => selected.has(item.key) && item.status === 'failed'
    ? { ...item, status: 'pending', reason: '' }
    : item)
}
