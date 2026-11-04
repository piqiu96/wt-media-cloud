export function materialRow(row) {
  return row.material ? { ...row.material, usage_id: row.id, owner_usage_status: row.status } : row
}
export function mergeSelection(previous, pageRows, selectedIds) {
  const next = { ...previous }
  const selected = new Set(selectedIds)
  for (const row of pageRows) {
    if (selected.has(row.id)) next[row.id] = row
    else delete next[row.id]
  }
  return next
}
export async function acceptSelectedMaterials(rows, accept, concurrency = 4) {
  const accepted = [], failed = []
  let index = 0
  async function worker() {
    while (index < rows.length) {
      const row = rows[index++]
      try { await accept(row.id); accepted.push(row) }
      catch (e) { failed.push({ ...row, selection_error: e?.message || '使用素材失败' }) }
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, rows.length) }, worker))
  return { accepted, failed }
}
