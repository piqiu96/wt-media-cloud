// Reuse the idempotent single-material command with bounded concurrency.
export async function joinMaterials(rows, addUsage) {
  const result = { joined: [], skipped: [], failed: [] }
  const pending = [...new Map(rows.map(row => [row.id, row])).values()]
  async function next() {
    while (pending.length) {
      const row = pending.shift()
      if (row.usage_status === 'active' || (row.status && row.status !== 'available')) {
        result.skipped.push({ ...row, reason: row.usage_status === 'active' ? '已加入' : '素材不可用' })
        continue
      }
      try {
        await addUsage(row.id)
        result.joined.push(row)
      } catch (error) {
        result.failed.push({ ...row, reason: error?.message || '加入失败' })
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(3, pending.length) }, next))
  return result
}
