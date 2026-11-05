// Candidate scope is independent of the ownership inherited after selection.
export function taskStrategyFilters(categoryID, user) {
  const filters = { category_id: categoryID, status: 'enabled' }
  if (user?.role !== 'admin') filters.team_id = user?.team_id
  if (user?.role === 'operator') filters.operator_id = user.id
  return filters
}

export function taskStrategyContext(strategy, strategyID, categoryID, user) {
  if (!user || !strategy || !strategyID || !categoryID ||
      Number(strategy.id) !== Number(strategyID) || Number(strategy.category_id) !== Number(categoryID) ||
      strategy.status !== 'enabled' || !strategy.team_id || !strategy.operator_id) return null
  if (user.role !== 'admin' && Number(strategy.team_id) !== Number(user.team_id)) return null
  if (user.role === 'operator' && Number(strategy.operator_id) !== Number(user.id)) return null
  return { team_id: strategy.team_id, operator_id: strategy.operator_id }
}
