// Publishing uses the general team AND responsible-category scope (not the production exception).
export function assignableOwners(actor, members, categories = []) {
  if (!actor || actor.disabled) return []
  const admin = actor.role === '管理员', senior = actor.role === '高级运营'
  if (!admin && categories.some(c => !actor.categories?.includes(c))) return []
  return members.filter(member => !member.disabled &&
    (admin || (member.team === actor.team && (senior || member.id === actor.id))) &&
    categories.every(c => member.categories?.includes(c)))
}
export function ownershipIssue(editor, original, actor, members) {
  if (original && editor.owner === original.owner && editor.ownerId === original.ownerId) return ''
  return assignableOwners(actor, members, editor.categories).some(person => person.id === editor.ownerId && person.name === editor.owner)
    ? '' : '请选择当前权限范围内、负责计划类目的有效负责人'
}
