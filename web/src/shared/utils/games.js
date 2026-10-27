export const GAME_STATUS_ENABLED = 'enabled'

/** 只保留可被选择 / 筛选的游戏；历史行名称解析仍用全量目录。 */
export function enabledGames(games) {
  if (!Array.isArray(games)) return []
  return games.filter((game) => game?.status === GAME_STATUS_ENABLED)
}
