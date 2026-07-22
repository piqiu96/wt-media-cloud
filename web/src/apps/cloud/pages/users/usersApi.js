import { createApiClient } from '../../../../shared/api/http.js'

export function createUsersClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    listUsers(filters = {}) {
      return api.get('/users', {
        uid: filters.uid,
        username: filters.username,
        role: filters.role,
        team_id: filters.teamId,
        status: filters.status,
        game_id: filters.gameId,
      })
    },
    createUser(input) {
      return api.post('/users', userPayload(input, true))
    },
    updateUser(userId, input) {
      return api.patch(`/users/${userId}`, userPayload(input, false))
    },
    resetPassword(userId, newPassword) {
      return api.post(`/users/${userId}/reset-password`, { new_password: newPassword })
    },
    deleteUser(userId) {
      return api.delete(`/users/${userId}`)
    },
    listTeams() {
      return api.get('/operation-teams')
    },
    createTeam(name) {
      return api.post('/operation-teams', { name })
    },
    renameTeam(teamId, name) {
      return api.patch(`/operation-teams/${teamId}`, { name })
    },
    deleteTeam(teamId) {
      return api.delete(`/operation-teams/${teamId}`)
    },
    listGames(filters = {}) {
      return api.get('/games', {
        keyword: filters.keyword,
        status: filters.status,
      })
    },
    createGame(input) {
      return api.post('/games', { id: input.id, name: input.name, remark: input.remark })
    },
    updateGame(gameId, input) {
      return api.patch(`/games/${gameId}`, { name: input.name, status: input.status, remark: input.remark })
    },
    deleteGame(gameId) {
      return api.delete(`/games/${gameId}`)
    },
  }
}

function userPayload(input, includeCredentials) {
  const payload = {
    role: input.role,
    team_id: input.role === 'admin' ? null : Number(input.teamId),
    status: input.status,
    game_ids: input.role === 'admin' ? [] : input.gameIds,
  }
  if (includeCredentials) {
    payload.username = input.username
    payload.password = input.password
    delete payload.status
  }
  return payload
}
