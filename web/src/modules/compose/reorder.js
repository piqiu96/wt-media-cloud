export const REORDER_MIME = 'application/x-wt-compose-reorder'

// Retain object identity so a move never reconstructs or resets a form draft.
export function reorderItems(items, sourceKey, targetKey, keyField = 'key') {
  const from = items.findIndex((item) => item[keyField] === sourceKey)
  const to = items.findIndex((item) => item[keyField] === targetKey)
  if (from < 0 || to < 0 || from === to) return items
  const reordered = [...items]
  const [item] = reordered.splice(from, 1)
  reordered.splice(to, 0, item)
  return reordered
}
export function draggedKey(transfer, scope) {
  try {
    const value = JSON.parse(transfer.getData(REORDER_MIME))
    return value.scope === scope && typeof value.key === 'string' ? value.key : null
  } catch { return null }
}
