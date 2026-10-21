export function canSelectForDownload(row) {
  return row?.usage_status !== 'removed'
    && row?.download_status !== 'downloaded'
    && row?.download_status !== 'downloading'
}

export async function submitMaterialDownloads(rows, selectedUsageIds, createDownload) {
  const selected = new Set(selectedUsageIds.map(String))
  const eligible = rows.filter((row) => selected.has(String(row.usage_id)) && canSelectForDownload(row))
  const skipped = selected.size - eligible.length
  let submitted = 0
  const failures = []
  for (const row of eligible) {
    try {
      await createDownload(row.id)
      submitted += 1
    } catch (error) {
      failures.push({ row, error })
    }
  }
  return { total: selected.size, submitted, skipped, failures }
}
