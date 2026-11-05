export function relatedTarget(path, kind, id) {
  id = Number(id)
  if (!Number.isSafeInteger(id) || id <= 0 || !['task','batch','material','strategy','output'].includes(kind)) return null
  const ancestor = path.find(item => item.kind === kind && item.id === id)
  return ancestor ? { back: ancestor.key } : { kind, id, key: `${kind}-${id}` }
}
